package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/webfetch"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/google/uuid"
)

// Kiro/CodeWhisperer 原生 web_search / web_fetch 走独立的 InvokeMCP API
// (AmazonCodeWhispererStreamingService.InvokeMCP)，采用 JSON-RPC 2.0 tools/call。
// 抓包证实:generateAssistantResponse 只吐出 toolUseEvent(要搜什么),真正执行搜索
// 由客户端对同一端点发独立 InvokeMCP 请求完成。此文件在网关侧复刻该调用。

const (
	kiroMCPTarget           = "AmazonCodeWhispererStreamingService.InvokeMCP"
	kiroMCPContentType      = "application/x-amz-json-1.0"
	kiroMCPToolSearch       = "web_search"
	kiroMCPToolFetch        = "web_fetch"
	kiroMCPMaxResponseBytes = 8 << 20
)

// kiroMCPRequest 是 InvokeMCP 的 JSON-RPC 请求体。
type kiroMCPRequest struct {
	ProfileArn string        `json:"profileArn,omitempty"`
	JSONRPC    string        `json:"jsonrpc"`
	ID         string        `json:"id"`
	Method     string        `json:"method"`
	Params     kiroMCPParams `json:"params"`
}

type kiroMCPParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// kiroMCPResponse 是 InvokeMCP 的响应体(普通 JSON,非 event-stream)。
type kiroMCPResponse struct {
	ID      string         `json:"id"`
	JSONRPC string         `json:"jsonrpc"`
	Result  *kiroMCPResult `json:"result"`
	Error   *kiroMCPError  `json:"error"`
}

type kiroMCPResult struct {
	Content []kiroMCPContent `json:"content"`
	IsError bool             `json:"isError"`
}

type kiroMCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type kiroMCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// invokeKiroMCP 调用 CodeWhisperer InvokeMCP,返回 result.content[0].text 的原始字符串
// (内层通常是被字符串化的 JSON)。401/403 无效 token 时刷新并重试一次。
func (s *KiroGatewayService) invokeKiroMCP(
	ctx context.Context,
	account *Account,
	toolName string,
	arguments map[string]any,
) (string, error) {
	if s == nil {
		return "", errors.New("kiro gateway service is required")
	}
	if account == nil {
		return "", errors.New("account is required")
	}

	runtimeSettings := s.resolveKiroRuntimeSettings(ctx)

	accessToken, err := s.resolveAccessToken(ctx, account)
	if err != nil {
		return "", fmt.Errorf("kiro mcp: resolve access token: %w", err)
	}
	if err := s.ensureKiroResolvedProfileARN(ctx, account, accessToken); err != nil {
		return "", err
	}
	if strings.TrimSpace(accountCredential(account, "profile_arn")) == "" {
		return "", errors.New("kiro mcp: profile_arn is required")
	}

	req, err := s.buildKiroMCPRequest(ctx, account, toolName, arguments, accessToken, runtimeSettings)
	if err != nil {
		return "", err
	}

	tlsRuntime := s.resolveTLSFingerprintRuntime(ctx, nil, account)
	resp, err := s.httpUpstream.DoWithTLS(req, accountProxyURL(account), account.ID, account.EffectiveConcurrency(), tlsRuntime.Profile)
	if err != nil {
		return "", fmt.Errorf("kiro mcp: upstream request: %w", err)
	}
	body, readErr := readKiroMCPResponseBody(resp.Body)
	if readErr != nil {
		return "", readErr
	}

	// 无效 token → 刷新账号并重试一次
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && isKiroInvalidTokenResponse(body) {
		refreshedAccount, refreshErr := s.refreshKiroAccountForRetry(ctx, account)
		if refreshErr == nil && refreshedAccount != nil {
			newToken := refreshedAccount.GetCredential("access_token")
			if newToken != "" {
				retryReq, buildErr := s.buildKiroMCPRequest(ctx, refreshedAccount, toolName, arguments, newToken, runtimeSettings)
				if buildErr != nil {
					return "", buildErr
				}
				retryResp, retryErr := s.httpUpstream.DoWithTLS(retryReq, accountProxyURL(refreshedAccount), refreshedAccount.ID, refreshedAccount.EffectiveConcurrency(), s.resolveTLSFingerprintRuntime(ctx, nil, refreshedAccount).Profile)
				if retryErr != nil {
					return "", fmt.Errorf("kiro mcp: retry upstream request: %w", retryErr)
				}
				body, readErr = readKiroMCPResponseBody(retryResp.Body)
				if readErr != nil {
					return "", readErr
				}
				resp = retryResp
			}
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("kiro mcp: upstream status %d: %s", resp.StatusCode, truncateString(strings.TrimSpace(string(body)), 256))
	}

	var parsed kiroMCPResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("kiro mcp: decode response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("kiro mcp: rpc error %d: %s", parsed.Error.Code, parsed.Error.Message)
	}
	if parsed.Result == nil || len(parsed.Result.Content) == 0 {
		return "", nil
	}
	// 取第一个 text 内容块
	for _, c := range parsed.Result.Content {
		if c.Type == "text" && c.Text != "" {
			return c.Text, nil
		}
	}
	return "", nil
}

func readKiroMCPResponseBody(body io.ReadCloser) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer func() { _ = body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(body, kiroMCPMaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("kiro mcp: read response: %w", err)
	}
	if len(raw) > kiroMCPMaxResponseBytes {
		return nil, fmt.Errorf("kiro mcp: response too large: exceeds %d bytes", kiroMCPMaxResponseBytes)
	}
	return raw, nil
}

// buildKiroMCPRequest 按抓包精确复刻 InvokeMCP 请求头与体。
func (s *KiroGatewayService) buildKiroMCPRequest(
	ctx context.Context,
	account *Account,
	toolName string,
	arguments map[string]any,
	accessToken string,
	runtimeSettings *KiroRuntimeSettings,
) (*http.Request, error) {
	profile, err := LoadOutboundDeviceProfile(ctx, account)
	if err != nil {
		return nil, err
	}
	runtimeSettings = applyKiroProfileRuntimeOverrides(runtimeSettings, profile)
	profileARN := strings.TrimSpace(accountCredential(account, "profile_arn"))

	payload := kiroMCPRequest{
		ProfileArn: profileARN,
		JSONRPC:    "2.0",
		ID:         "1",
		Method:     "tools/call",
		Params: kiroMCPParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("kiro mcp: marshal request: %w", err)
	}

	region := KiroRegion(account)
	url := fmt.Sprintf("https://q.%s.amazonaws.com/", region)
	if kiroEndpointName(account) == "runtime" {
		url = fmt.Sprintf("https://runtime.%s.kiro.dev/mcp", region)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("kiro mcp: new request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	machineID := profile.MachineID
	host := req.URL.Host
	kiroVersion := runtimeSettings.KiroVersion

	req.Header.Set("host", host)
	req.Header.Set("Content-Type", kiroMCPContentType)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if isKiroExternalIDPAccount(account) {
		req.Header.Set("TokenType", "EXTERNAL_IDP")
	} else if account != nil && account.Type == AccountTypeAPIKey {
		req.Header.Set("TokenType", "API_KEY")
	}
	req.Header.Set("x-amz-target", kiroMCPTarget)
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	if profileARN != "" {
		req.Header.Set("x-amzn-kiro-profile-arn", profileARN)
	}
	xAmzUserAgent, userAgent := kiropkg.BuildCodeWhispererStreamingUserAgents(kiroVersion, machineID, runtimeSettings.SystemVersion, runtimeSettings.NodeVersion)
	req.Header.Set("x-amz-user-agent", xAmzUserAgent)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=3")
	applyKiroTLSFingerprintRuntimeWithProfile(req, s.resolveTLSFingerprintRuntime(ctx, nil, account), profile)
	return req, nil
}

// kiroMCPWebSearchResults 把 InvokeMCP web_search 返回的内层 JSON 解析成 websearch.SearchResult。
//
// 内层格式(抓包):{"results":[{"title","url","snippet","publishedDate"(ms),"domain",...}]}
func kiroMCPWebSearchResults(innerText string) ([]websearch.SearchResult, error) {
	trimmed := strings.TrimSpace(innerText)
	if trimmed == "" {
		return nil, nil
	}
	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
			Domain  string `json:"domain"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("kiro mcp: decode web_search results: %w", err)
	}
	results := make([]websearch.SearchResult, 0, len(payload.Results))
	for _, r := range payload.Results {
		results = append(results, websearch.SearchResult{
			URL:     r.URL,
			Title:   r.Title,
			Snippet: r.Snippet,
		})
	}
	return results, nil
}

// kiroMCPEligible 判断该账号是否可尝试原生 InvokeMCP。
// External-IDP OAuth accounts may resolve profile_arn lazily. Other OAuth
// accounts need an existing ARN; their auth flow cannot resolve one.
func kiroMCPEligible(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return false
	}
	return strings.TrimSpace(accountCredential(account, "profile_arn")) != "" || isKiroExternalIDPAccount(account)
}

// kiroMCPWebSearch 用原生 InvokeMCP 执行 web_search。不适用/失败时返回 (nil, err),
// 由调用方回退到外部 provider。
func (s *KiroGatewayService) kiroMCPWebSearch(ctx context.Context, account *Account, query string) ([]websearch.SearchResult, error) {
	if !kiroMCPEligible(account) {
		return nil, errors.New("kiro mcp: account not eligible")
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("kiro mcp: empty query")
	}
	inner, err := s.invokeKiroMCP(ctx, account, kiroMCPToolSearch, map[string]any{"query": query})
	if err != nil {
		return nil, err
	}
	return kiroMCPWebSearchResults(inner)
}

// kiroMCPWebFetch 用原生 InvokeMCP 执行 web_fetch。返回 FetchResult 与是否已处理。
// URL/域名/私网约束先按本地 web_fetch 契约校验；校验失败时返回结构化错误且
// ok=true，避免绕过 allowed_domains / blocked_domains。账号不适用或 MCP 失败
// 时 ok=false，由调用方回退到本地 fetcher。
func (s *KiroGatewayService) kiroMCPWebFetch(ctx context.Context, account *Account, req webfetch.FetchRequest, maxContentTokens int) (*webfetch.FetchResult, bool) {
	if !kiroMCPEligible(account) {
		return nil, false
	}
	normalizedURL, _, fetchErr := webfetch.ValidateRequestURL(req.URL, req)
	if fetchErr != nil {
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			Error:        fetchErr,
		}, true
	}
	inner, err := s.invokeKiroMCP(ctx, account, kiroMCPToolFetch, map[string]any{"url": normalizedURL})
	if err != nil || strings.TrimSpace(inner) == "" {
		return nil, false
	}
	fetchResult := &webfetch.FetchResult{
		RequestedURL: req.URL,
		FinalURL:     normalizedURL,
		StatusCode:   http.StatusOK,
		ContentType:  "text/plain",
		Text:         strings.TrimSpace(inner),
	}
	return applyShadowWebFetchContentLimit(fetchResult, maxContentTokens), true
}
