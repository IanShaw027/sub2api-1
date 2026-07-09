package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// kiroMCPStubUpstream 捕获请求并返回预设响应,用于验证 InvokeMCP 调用。
type kiroMCPStubUpstream struct {
	lastReq  *http.Request
	lastBody []byte
	calls    int
	status   int
	respBody string
	err      error
}

func (u *kiroMCPStubUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *kiroMCPStubUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	u.lastReq = req
	if req.GetBody != nil {
		if rc, err := req.GetBody(); err == nil {
			u.lastBody, _ = io.ReadAll(rc)
			_ = rc.Close()
		}
	}
	if u.err != nil {
		return nil, u.err
	}
	status := u.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(u.respBody)),
		Header:     make(http.Header),
	}, nil
}

func newKiroMCPTestAccount() *Account {
	return &Account{
		ID:       42,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey, // apikey 走 resolveAccessToken 直接取 api_key,免 tokenProvider
		Credentials: map[string]any{
			"api_key":     "test-token",
			"profile_arn": "arn:aws:codewhisperer:us-east-1:123:profile/ABC",
		},
	}
}

func newKiroMCPOAuthTestAccount() *Account {
	return &Account{
		ID:       43,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "oauth-token",
			"expires_at":   "2099-01-01T00:00:00Z",
			"profile_arn":  "arn:aws:codewhisperer:us-east-1:123:profile/OAUTH",
		},
	}
}

func TestInvokeKiroMCP_BuildsJSONRPCRequest(t *testing.T) {
	stub := &kiroMCPStubUpstream{
		respBody: `{"id":"1","jsonrpc":"2.0","result":{"content":[{"type":"text","text":"{\"results\":[]}"}]}}`,
	}
	svc := &KiroGatewayService{httpUpstream: stub}
	acct := newKiroMCPTestAccount()

	_, err := svc.invokeKiroMCP(context.Background(), acct, kiroMCPToolSearch, map[string]any{"query": "hello"})
	require.NoError(t, err)

	// 验证请求头
	require.Equal(t, kiroMCPTarget, stub.lastReq.Header.Get("x-amz-target"))
	require.Equal(t, kiroMCPContentType, stub.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "Bearer test-token", stub.lastReq.Header.Get("Authorization"))
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123:profile/ABC", stub.lastReq.Header.Get("x-amzn-kiro-profile-arn"))

	// 验证 JSON-RPC 请求体
	var body kiroMCPRequest
	require.NoError(t, json.Unmarshal(stub.lastBody, &body))
	require.Equal(t, "2.0", body.JSONRPC)
	require.Equal(t, "tools/call", body.Method)
	require.Equal(t, "web_search", body.Params.Name)
	require.Equal(t, "hello", body.Params.Arguments["query"])
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123:profile/ABC", body.ProfileArn)
}

func TestInvokeKiroMCP_RejectsNilAccount(t *testing.T) {
	svc := &KiroGatewayService{}

	text, err := svc.invokeKiroMCP(context.Background(), nil, kiroMCPToolSearch, map[string]any{"query": "hello"})

	require.Empty(t, text)
	require.EqualError(t, err, "account is required")
}

func TestInvokeKiroMCP_RejectsNilService(t *testing.T) {
	var svc *KiroGatewayService

	text, err := svc.invokeKiroMCP(context.Background(), newKiroMCPTestAccount(), kiroMCPToolSearch, map[string]any{"query": "hello"})

	require.Empty(t, text)
	require.EqualError(t, err, "kiro gateway service is required")
}

func TestInvokeKiroMCP_ParsesWebSearchResults(t *testing.T) {
	inner := `{"results":[{"title":"AI News","url":"https://example.com/ai","snippet":"latest","domain":"example.com"},{"title":"T2","url":"https://e2.com","snippet":"s2"}]}`
	outer, _ := json.Marshal(map[string]any{
		"id": "1", "jsonrpc": "2.0",
		"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}},
	})
	stub := &kiroMCPStubUpstream{respBody: string(outer)}
	svc := &KiroGatewayService{httpUpstream: stub}

	// 直接调 invokeKiroMCP + 解析(用 apikey 账号免 tokenProvider),
	// eligibility 门控单独在 TestKiroMCPEligible 覆盖。
	innerText, err := svc.invokeKiroMCP(context.Background(), newKiroMCPTestAccount(), kiroMCPToolSearch, map[string]any{"query": "ai news"})
	require.NoError(t, err)
	results, err := kiroMCPWebSearchResults(innerText)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "AI News", results[0].Title)
	require.Equal(t, "https://example.com/ai", results[0].URL)
	require.Equal(t, "latest", results[0].Snippet)
}

func TestKiroMCPEligible(t *testing.T) {
	// OAuth + profile_arn → eligible
	a := newKiroMCPTestAccount()
	a.Type = AccountTypeOAuth
	require.True(t, kiroMCPEligible(a))

	// 缺 profile_arn → 不 eligible
	a2 := newKiroMCPTestAccount()
	a2.Type = AccountTypeOAuth
	delete(a2.Credentials, "profile_arn")
	require.False(t, kiroMCPEligible(a2))

	// 非 kiro 平台 → 不 eligible
	a3 := newKiroMCPTestAccount()
	a3.Type = AccountTypeOAuth
	a3.Platform = "openai"
	require.False(t, kiroMCPEligible(a3))

	require.False(t, kiroMCPEligible(nil))
}

func TestInvokeKiroMCP_RPCError(t *testing.T) {
	stub := &kiroMCPStubUpstream{
		respBody: `{"id":"1","jsonrpc":"2.0","error":{"code":-32000,"message":"boom"}}`,
	}
	svc := &KiroGatewayService{httpUpstream: stub}
	_, err := svc.invokeKiroMCP(context.Background(), newKiroMCPTestAccount(), kiroMCPToolSearch, map[string]any{"query": "x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestInvokeKiroMCP_ErrorsWhenResponseExceedsLimit(t *testing.T) {
	stub := &kiroMCPStubUpstream{
		respBody: strings.Repeat("x", (8<<20)+1),
	}
	svc := &KiroGatewayService{httpUpstream: stub}

	_, err := svc.invokeKiroMCP(context.Background(), newKiroMCPTestAccount(), kiroMCPToolSearch, map[string]any{"query": "x"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "response too large")
}

func TestKiroMCPWebSearchResults_Empty(t *testing.T) {
	results, err := kiroMCPWebSearchResults("")
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestExecuteKiroShadowTool_MCPWebFetchHonorsBlockedDomainsBeforeInvoke(t *testing.T) {
	stub := &kiroMCPStubUpstream{
		respBody: `{"id":"1","jsonrpc":"2.0","result":{"content":[{"type":"text","text":"mcp should not be called"}]}}`,
	}
	svc := &KiroGatewayService{
		httpUpstream:  stub,
		tokenProvider: NewKiroTokenProvider(nil, nil),
	}
	state := &kiroToolState{ToolUseID: "tool-fetch", Name: "web_fetch"}
	_, _ = state.InputBuilder.WriteString(`{"url":"https://blocked.example/fetch"}`)

	blocks, summary, err := svc.executeKiroShadowTool(context.Background(), newKiroMCPOAuthTestAccount(), state, kiropkg.ShadowToolBridge{
		AnthropicType:  "web_fetch_20250305",
		AnthropicName:  "web_fetch",
		BlockedDomains: []string{"blocked.example"},
	})

	require.NoError(t, err)
	require.Zero(t, stub.calls)
	require.Len(t, blocks, 2)
	resultBlock := blocks[1]
	require.Equal(t, "web_fetch_tool_result", resultBlock["type"])
	content, _ := resultBlock["content"].(map[string]any)
	require.Equal(t, "web_fetch_tool_error", content["type"])
	require.Equal(t, "url_not_allowed", content["error_code"])
	require.Contains(t, summary, "host is blocked")
}

func TestExecuteKiroShadowTool_MCPWebFetchAppliesMaxContentTokens(t *testing.T) {
	inner := strings.Repeat("alpha beta gamma delta epsilon ", 8)
	outer, _ := json.Marshal(map[string]any{
		"id": "1", "jsonrpc": "2.0",
		"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}},
	})
	stub := &kiroMCPStubUpstream{respBody: string(outer)}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	svc := &KiroGatewayService{
		httpUpstream:   stub,
		tokenProvider:  NewKiroTokenProvider(nil, nil),
		settingService: NewSettingService(nil, cfg),
	}
	state := &kiroToolState{ToolUseID: "tool-fetch", Name: "web_fetch"}
	_, _ = state.InputBuilder.WriteString(`{"url":"https://example.com/fetch"}`)

	blocks, summary, err := svc.executeKiroShadowTool(context.Background(), newKiroMCPOAuthTestAccount(), state, kiropkg.ShadowToolBridge{
		AnthropicType:    "web_fetch_20250305",
		AnthropicName:    "web_fetch",
		MaxContentTokens: 3,
	})

	require.NoError(t, err)
	require.Equal(t, 1, stub.calls)
	require.Len(t, blocks, 2)
	resultBlock := blocks[1]
	content, _ := resultBlock["content"].(map[string]any)
	require.Equal(t, "web_fetch_result", content["type"])
	text, _ := content["text"].(string)
	require.NotEmpty(t, text)
	require.LessOrEqual(t, kiropkg.AccurateTokenCount(text), 3)
	require.Equal(t, text, summary)
	document, _ := content["document"].(map[string]any)
	source, _ := document["source"].(map[string]any)
	require.Equal(t, text, source["data"])
}
