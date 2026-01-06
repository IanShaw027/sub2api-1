package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ForwardCountTokens 转发 count_tokens 请求到上游 API
// 特点：不记录使用量、仅支持非流式响应
func (s *GatewayService) ForwardCountTokens(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) error {
	if parsed == nil {
		s.countTokensError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return fmt.Errorf("parse request: empty request")
	}

	body := parsed.Body
	reqModel := parsed.Model

	// Antigravity 账户不支持 count_tokens 转发，返回估算值
	// 参考 Antigravity-Manager 和 proxycast 实现
	if account.Platform == PlatformAntigravity {
		c.JSON(http.StatusOK, gin.H{"input_tokens": 100})
		return nil
	}

	// 应用模型映射（仅对 apikey 类型账号）
	if account.Type == AccountTypeAPIKey {
		if reqModel != "" {
			mappedModel := account.GetMappedModel(reqModel)
			if mappedModel != reqModel {
				body = s.replaceModelInBody(body, mappedModel)
				reqModel = mappedModel
				log.Printf("CountTokens model mapping applied: %s -> %s (account: %s)", parsed.Model, mappedModel, account.Name)
			}
		}
	}

	// 获取凭证
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to get access token")
		return err
	}

	// 构建上游请求
	upstreamReq, err := s.buildCountTokensRequest(ctx, c, account, body, token, tokenType, reqModel)
	if err != nil {
		s.countTokensError(c, http.StatusInternalServerError, "api_error", "Failed to build request")
		return err
	}

	// 获取代理URL
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// 发送请求
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "timeout_error:") {
			detail := strings.TrimPrefix(errMsg, "timeout_error: ")
			log.Printf("Account %d: count_tokens upstream timeout: %s", account.ID, detail)
		} else if strings.Contains(errMsg, "network_error:") {
			detail := strings.TrimPrefix(errMsg, "network_error: ")
			log.Printf("Account %d: count_tokens upstream network error: %s", account.ID, detail)
		}
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Request failed")
		return fmt.Errorf("upstream request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to read response")
		return err
	}

	// 处理错误响应
	if resp.StatusCode >= 400 {
		// 标记账号状态（429/529等）
		s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)

		// 记录上游错误摘要便于排障（不回显请求内容）
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			log.Printf(
				"count_tokens upstream error %d (account=%d platform=%s type=%s): %s",
				resp.StatusCode,
				account.ID,
				account.Platform,
				account.Type,
				truncateForLog(respBody, s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes),
			)
		}

		// 返回简化的错误响应
		errMsg := "Upstream request failed"
		switch resp.StatusCode {
		case 429:
			errMsg = "Rate limit exceeded"
		case 529:
			errMsg = "Service overloaded"
		}
		s.countTokensError(c, resp.StatusCode, "upstream_error", errMsg)
		return fmt.Errorf("upstream error: %d", resp.StatusCode)
	}

	// 透传成功响应
	c.Data(resp.StatusCode, "application/json", respBody)
	return nil
}

// buildCountTokensRequest 构建 count_tokens 上游请求
func (s *GatewayService) buildCountTokensRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token, tokenType, modelID string) (*http.Request, error) {
	// 确定目标 URL
	targetURL := claudeAPICountTokensURL
	if account.Type == AccountTypeAPIKey {
		baseURL := account.GetBaseURL()
		targetURL = baseURL + "/v1/messages/count_tokens"
	}

	// OAuth 账号：应用统一指纹和重写 userID
	if account.IsOAuth() && s.identityService != nil {
		fp, err := s.identityService.GetOrCreateFingerprint(ctx, account.ID, c.Request.Header)
		if err == nil {
			accountUUID := account.GetExtraString("account_uuid")
			if accountUUID != "" && fp.ClientID != "" {
				if newBody, err := s.identityService.RewriteUserID(body, account.ID, accountUUID, fp.ClientID); err == nil && len(newBody) > 0 {
					body = newBody
				}
			}
		}
	}

	// Filter thinking blocks from request body (prevents 400 errors from invalid signatures)
	body = FilterThinkingBlocks(body)

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 设置认证头
	if tokenType == "oauth" {
		req.Header.Set("authorization", "Bearer "+token)
	} else {
		req.Header.Set("x-api-key", token)
	}

	// White-list passthrough headers.
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if allowedHeaders[lowerKey] {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}

	// OAuth 账号：应用指纹到请求头
	if account.IsOAuth() && s.identityService != nil {
		fp, _ := s.identityService.GetOrCreateFingerprint(ctx, account.ID, c.Request.Header)
		if fp != nil {
			s.identityService.ApplyFingerprint(req, fp)
		}
	}

	// 确保必要的 headers 存在
	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}
	if req.Header.Get("anthropic-version") == "" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	// OAuth 账号：处理 anthropic-beta header
	if tokenType == "oauth" {
		req.Header.Set("anthropic-beta", s.getBetaHeader(modelID, c.GetHeader("anthropic-beta")))
	} else if s.cfg != nil && s.cfg.Gateway.InjectBetaForAPIKey && req.Header.Get("anthropic-beta") == "" {
		// API-key：与 messages 同步的按需 beta 注入（默认关闭）
		if requestNeedsBetaFeatures(body) {
			if beta := defaultAPIKeyBetaHeader(body); beta != "" {
				req.Header.Set("anthropic-beta", beta)
			}
		}
	}

	return req, nil
}

// countTokensError 返回 count_tokens 错误响应
func (s *GatewayService) countTokensError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
