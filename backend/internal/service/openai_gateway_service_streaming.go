package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// openaiStreamingResult streaming response result
type openaiStreamingResult struct {
	usage        *OpenAIUsage
	firstTokenMs *int
}

func (s *OpenAIGatewayService) handleStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, startTime time.Time, originalModel, mappedModel string) (*openaiStreamingResult, error) {
	// Set SSE response headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Pass through other headers
	if v := resp.Header.Get("x-request-id"); v != "" {
		c.Header("x-request-id", v)
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	usage := &OpenAIUsage{}
	var firstTokenMs *int
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	needModelReplace := originalModel != mappedModel

	for scanner.Scan() {
		line := scanner.Text()

		// Extract data from SSE line (supports both "data: " and "data:" formats)
		if openaiSSEDataRe.MatchString(line) {
			data := openaiSSEDataRe.ReplaceAllString(line, "")

			// Replace model in response if needed
			if needModelReplace {
				line = s.replaceModelInSSELine(line, mappedModel, originalModel)
			}

			// Forward line
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return &openaiStreamingResult{usage: usage, firstTokenMs: firstTokenMs}, err
			}
			flusher.Flush()

			// Record first token time
			if firstTokenMs == nil && data != "" && data != "[DONE]" {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			s.parseSSEUsage(data, usage)
		} else {
			// Forward non-data lines as-is
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return &openaiStreamingResult{usage: usage, firstTokenMs: firstTokenMs}, err
			}
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return &openaiStreamingResult{usage: usage, firstTokenMs: firstTokenMs}, fmt.Errorf("stream read error: %w", err)
	}

	return &openaiStreamingResult{usage: usage, firstTokenMs: firstTokenMs}, nil
}

func (s *OpenAIGatewayService) replaceModelInSSELine(line, fromModel, toModel string) string {
	if !openaiSSEDataRe.MatchString(line) {
		return line
	}
	data := openaiSSEDataRe.ReplaceAllString(line, "")
	if data == "" || data == "[DONE]" {
		return line
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return line
	}

	// Replace model in response
	if m, ok := event["model"].(string); ok && m == fromModel {
		event["model"] = toModel
		newData, err := json.Marshal(event)
		if err != nil {
			return line
		}
		return "data: " + string(newData)
	}

	// Check nested response
	if response, ok := event["response"].(map[string]any); ok {
		if m, ok := response["model"].(string); ok && m == fromModel {
			response["model"] = toModel
			newData, err := json.Marshal(event)
			if err != nil {
				return line
			}
			return "data: " + string(newData)
		}
	}

	return line
}

func (s *OpenAIGatewayService) parseSSEUsage(data string, usage *OpenAIUsage) {
	// Parse response.completed event for usage (OpenAI Responses format)
	var event struct {
		Type     string `json:"type"`
		Response struct {
			Usage struct {
				InputTokens       int `json:"input_tokens"`
				OutputTokens      int `json:"output_tokens"`
				InputTokenDetails struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"input_tokens_details"`
			} `json:"usage"`
		} `json:"response"`
	}

	if json.Unmarshal([]byte(data), &event) == nil && event.Type == "response.completed" {
		usage.InputTokens = event.Response.Usage.InputTokens
		usage.OutputTokens = event.Response.Usage.OutputTokens
		usage.CacheReadInputTokens = event.Response.Usage.InputTokenDetails.CachedTokens
	}
}

func (s *OpenAIGatewayService) handleNonStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, originalModel, mappedModel string) (*OpenAIUsage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse usage
	var response struct {
		Usage struct {
			InputTokens       int `json:"input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			InputTokenDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	usage := &OpenAIUsage{
		InputTokens:          response.Usage.InputTokens,
		OutputTokens:         response.Usage.OutputTokens,
		CacheReadInputTokens: response.Usage.InputTokenDetails.CachedTokens,
	}

	// Replace model in response if needed
	if originalModel != mappedModel {
		body = s.replaceModelInResponseBody(body, mappedModel, originalModel)
	}

	// Pass through headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	c.Data(resp.StatusCode, "application/json", body)

	return usage, nil
}

func (s *OpenAIGatewayService) replaceModelInResponseBody(body []byte, fromModel, toModel string) []byte {
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}

	model, ok := resp["model"].(string)
	if !ok || model != fromModel {
		return body
	}

	resp["model"] = toModel
	newBody, err := json.Marshal(resp)
	if err != nil {
		return body
	}

	return newBody
}
