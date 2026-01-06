package service

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/gin-gonic/gin"
)

type geminiStreamResult struct {
	usage        *ClaudeUsage
	firstTokenMs *int
}

func (s *GeminiMessagesCompatService) handleNonStreamingResponse(c *gin.Context, resp *http.Response, originalModel string, startTime time.Time) (*ClaudeUsage, *int, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, nil, s.writeClaudeError(c, http.StatusBadGateway, "upstream_error", "Failed to read upstream response")
	}

	// 非流式响应：TTFT = 收到完整响应的时间
	ttft := int(time.Since(startTime).Milliseconds())

	geminiResp, err := unwrapGeminiResponse(body)
	if err != nil {
		return nil, nil, s.writeClaudeError(c, http.StatusBadGateway, "upstream_error", "Failed to parse upstream response")
	}

	claudeResp, usage := convertGeminiToClaudeMessage(geminiResp, originalModel)
	c.JSON(http.StatusOK, claudeResp)

	return usage, &ttft, nil
}

func (s *GeminiMessagesCompatService) handleStreamingResponse(c *gin.Context, resp *http.Response, startTime time.Time, originalModel string) (*geminiStreamResult, error) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	messageID := "msg_" + randomHex(12)
	messageStart := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         originalModel,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	writeSSE(c.Writer, "message_start", messageStart)
	flusher.Flush()

	var firstTokenMs *int
	var usage ClaudeUsage
	finishReason := ""
	sawToolUse := false

	nextBlockIndex := 0
	openBlockIndex := -1
	openBlockType := ""
	seenText := ""
	openToolIndex := -1
	openToolID := ""
	openToolName := ""
	seenToolJSON := ""

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("stream read error: %w", err)
		}

		if !strings.HasPrefix(line, "data:") {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}

		geminiResp, err := unwrapGeminiResponse([]byte(payload))
		if err != nil {
			continue
		}

		if fr := extractGeminiFinishReason(geminiResp); fr != "" {
			finishReason = fr
		}

		parts := extractGeminiParts(geminiResp)
		for _, part := range parts {
			if text, ok := part["text"].(string); ok && text != "" {
				delta, newSeen := computeGeminiTextDelta(seenText, text)
				seenText = newSeen
				if delta == "" {
					continue
				}

				if openBlockType != "text" {
					if openBlockIndex >= 0 {
						writeSSE(c.Writer, "content_block_stop", map[string]any{
							"type":  "content_block_stop",
							"index": openBlockIndex,
						})
					}
					openBlockType = "text"
					openBlockIndex = nextBlockIndex
					nextBlockIndex++
					writeSSE(c.Writer, "content_block_start", map[string]any{
						"type":  "content_block_start",
						"index": openBlockIndex,
						"content_block": map[string]any{
							"type": "text",
							"text": "",
						},
					})
				}

				if firstTokenMs == nil {
					ms := int(time.Since(startTime).Milliseconds())
					firstTokenMs = &ms
				}
				writeSSE(c.Writer, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": openBlockIndex,
					"delta": map[string]any{
						"type": "text_delta",
						"text": delta,
					},
				})
				flusher.Flush()
				continue
			}

			if fc, ok := part["functionCall"].(map[string]any); ok && fc != nil {
				name, _ := fc["name"].(string)
				args := fc["args"]
				if strings.TrimSpace(name) == "" {
					name = "tool"
				}

				// Close any open text block before tool_use.
				if openBlockIndex >= 0 {
					writeSSE(c.Writer, "content_block_stop", map[string]any{
						"type":  "content_block_stop",
						"index": openBlockIndex,
					})
					openBlockIndex = -1
					openBlockType = ""
				}

				// If we receive streamed tool args in pieces, keep a single tool block open and emit deltas.
				if openToolIndex >= 0 && openToolName != name {
					writeSSE(c.Writer, "content_block_stop", map[string]any{
						"type":  "content_block_stop",
						"index": openToolIndex,
					})
					openToolIndex = -1
					openToolName = ""
					seenToolJSON = ""
				}

				if openToolIndex < 0 {
					openToolID = "toolu_" + randomHex(8)
					openToolIndex = nextBlockIndex
					openToolName = name
					nextBlockIndex++
					sawToolUse = true

					writeSSE(c.Writer, "content_block_start", map[string]any{
						"type":  "content_block_start",
						"index": openToolIndex,
						"content_block": map[string]any{
							"type":  "tool_use",
							"id":    openToolID,
							"name":  name,
							"input": map[string]any{},
						},
					})
				}

				argsJSONText := "{}"
				switch v := args.(type) {
				case nil:
					// keep default "{}"
				case string:
					if strings.TrimSpace(v) != "" {
						argsJSONText = v
					}
				default:
					if b, err := json.Marshal(args); err == nil && len(b) > 0 {
						argsJSONText = string(b)
					}
				}

				delta, newSeen := computeGeminiTextDelta(seenToolJSON, argsJSONText)
				seenToolJSON = newSeen
				if delta != "" {
					writeSSE(c.Writer, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": openToolIndex,
						"delta": map[string]any{
							"type":         "input_json_delta",
							"partial_json": delta,
						},
					})
				}
				flusher.Flush()
			}
		}

		if u := extractGeminiUsage(geminiResp); u != nil {
			usage = *u
		}

		// Process the final unterminated line at EOF as well.
		if errors.Is(err, io.EOF) {
			break
		}
	}

	if openBlockIndex >= 0 {
		writeSSE(c.Writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": openBlockIndex,
		})
	}
	if openToolIndex >= 0 {
		writeSSE(c.Writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": openToolIndex,
		})
	}

	stopReason := mapGeminiFinishReasonToClaudeStopReason(finishReason)
	if sawToolUse {
		stopReason = "tool_use"
	}

	usageObj := map[string]any{
		"output_tokens": usage.OutputTokens,
	}
	if usage.InputTokens > 0 {
		usageObj["input_tokens"] = usage.InputTokens
	}
	writeSSE(c.Writer, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": usageObj,
	})
	writeSSE(c.Writer, "message_stop", map[string]any{
		"type": "message_stop",
	})
	flusher.Flush()

	return &geminiStreamResult{usage: &usage, firstTokenMs: firstTokenMs}, nil
}

func writeSSE(w io.Writer, event string, data any) {
	if event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", event)
	}
	b, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *GeminiMessagesCompatService) writeClaudeError(c *gin.Context, status int, errType, message string) error {
	c.JSON(status, gin.H{
		"type":  "error",
		"error": gin.H{"type": errType, "message": message},
	})
	return fmt.Errorf("%s", message)
}

func (s *GeminiMessagesCompatService) writeGoogleError(c *gin.Context, status int, message string) error {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    status,
			"message": message,
			"status":  googleapi.HTTPStatusToGoogleStatus(status),
		},
	})
	return fmt.Errorf("%s", message)
}
