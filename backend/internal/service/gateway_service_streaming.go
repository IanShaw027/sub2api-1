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

// streamingResult 流式响应结果
type streamingResult struct {
	usage        *ClaudeUsage
	firstTokenMs *int
}

func (s *GatewayService) handleStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, startTime time.Time, originalModel, mappedModel string) (*streamingResult, error) {
	// 更新5h窗口状态
	s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 透传其他响应头
	if v := resp.Header.Get("x-request-id"); v != "" {
		c.Header("x-request-id", v)
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	usage := &ClaudeUsage{}
	var firstTokenMs *int
	scanner := bufio.NewScanner(resp.Body)
	// 设置更大的buffer以处理长行
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	needModelReplace := originalModel != mappedModel

	for scanner.Scan() {
		line := scanner.Text()
		if line == "event: error" {
			return nil, errors.New("have error in stream")
		}

		// Extract data from SSE line (supports both "data: " and "data:" formats)
		if sseDataRe.MatchString(line) {
			data := sseDataRe.ReplaceAllString(line, "")

			// 如果有模型映射，替换响应中的model字段
			if needModelReplace {
				line = s.replaceModelInSSELine(line, mappedModel, originalModel)
			}

			// 转发行
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, err
			}
			flusher.Flush()

			// 记录首字时间：第一个有效的 content_block_delta 或 message_start
			if firstTokenMs == nil && data != "" && data != "[DONE]" {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			s.parseSSEUsage(data, usage)
		} else {
			// 非 data 行直接转发
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, err
			}
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, fmt.Errorf("stream read error: %w", err)
	}

	return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, nil
}

// replaceModelInSSELine 替换SSE数据行中的model字段
func (s *GatewayService) replaceModelInSSELine(line, fromModel, toModel string) string {
	if !sseDataRe.MatchString(line) {
		return line
	}
	data := sseDataRe.ReplaceAllString(line, "")
	if data == "" || data == "[DONE]" {
		return line
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return line
	}

	// 只替换 message_start 事件中的 message.model
	if event["type"] != "message_start" {
		return line
	}

	msg, ok := event["message"].(map[string]any)
	if !ok {
		return line
	}

	model, ok := msg["model"].(string)
	if !ok || model != fromModel {
		return line
	}

	msg["model"] = toModel
	newData, err := json.Marshal(event)
	if err != nil {
		return line
	}

	return "data: " + string(newData)
}

func (s *GatewayService) parseSSEUsage(data string, usage *ClaudeUsage) {
	// 解析message_start获取input tokens（标准Claude API格式）
	var msgStart struct {
		Type    string `json:"type"`
		Message struct {
			Usage ClaudeUsage `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal([]byte(data), &msgStart) == nil && msgStart.Type == "message_start" {
		usage.InputTokens = msgStart.Message.Usage.InputTokens
		usage.CacheCreationInputTokens = msgStart.Message.Usage.CacheCreationInputTokens
		usage.CacheReadInputTokens = msgStart.Message.Usage.CacheReadInputTokens
	}

	// 解析message_delta获取tokens（兼容GLM等把所有usage放在delta中的API）
	var msgDelta struct {
		Type  string `json:"type"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &msgDelta) == nil && msgDelta.Type == "message_delta" {
		// output_tokens 总是从 message_delta 获取
		usage.OutputTokens = msgDelta.Usage.OutputTokens

		// 如果 message_start 中没有值，则从 message_delta 获取（兼容GLM等API）
		if usage.InputTokens == 0 {
			usage.InputTokens = msgDelta.Usage.InputTokens
		}
		if usage.CacheCreationInputTokens == 0 {
			usage.CacheCreationInputTokens = msgDelta.Usage.CacheCreationInputTokens
		}
		if usage.CacheReadInputTokens == 0 {
			usage.CacheReadInputTokens = msgDelta.Usage.CacheReadInputTokens
		}
	}
}

func (s *GatewayService) handleNonStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, originalModel, mappedModel string, startTime time.Time) (*ClaudeUsage, *int, error) {
	// 更新5h窗口状态
	s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// 非流式响应：TTFT = 收到完整响应的时间
	ttft := int(time.Since(startTime).Milliseconds())

	// 解析usage
	var response struct {
		Usage ClaudeUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("parse response: %w", err)
	}

	// 如果有模型映射，替换响应中的model字段
	if originalModel != mappedModel {
		body = s.replaceModelInResponseBody(body, mappedModel, originalModel)
	}

	// 透传响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 写入响应
	c.Data(resp.StatusCode, "application/json", body)

	return &response.Usage, &ttft, nil
}

// replaceModelInResponseBody 替换响应体中的model字段
func (s *GatewayService) replaceModelInResponseBody(body []byte, fromModel, toModel string) []byte {
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
