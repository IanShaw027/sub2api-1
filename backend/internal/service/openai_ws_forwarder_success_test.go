package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayService_Forward_WSv2_SuccessAndBindSticky(t *testing.T) {
	setGinTestMode()

	type receivedPayload struct {
		Type               string
		PreviousResponseID string
		StreamExists       bool
		Stream             bool
	}
	receivedCh := make(chan receivedPayload, 1)

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read ws request failed: %v", err)
			return
		}
		requestJSON := requestToJSONString(request)
		receivedCh <- receivedPayload{
			Type:               strings.TrimSpace(gjson.Get(requestJSON, "type").String()),
			PreviousResponseID: strings.TrimSpace(gjson.Get(requestJSON, "previous_response_id").String()),
			StreamExists:       gjson.Get(requestJSON, "stream").Exists(),
			Stream:             gjson.Get(requestJSON, "stream").Bool(),
		}

		if err := conn.WriteJSON(map[string]any{
			"type": "response.created",
			"response": map[string]any{
				"id":    "resp_new_1",
				"model": "gpt-5.1",
			},
		}); err != nil {
			t.Errorf("write response.created failed: %v", err)
			return
		}
		if err := conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":    "resp_new_1",
				"model": "gpt-5.1",
				"usage": map[string]any{
					"input_tokens":  12,
					"output_tokens": 7,
					"input_tokens_details": map[string]any{
						"cached_tokens": 3,
					},
				},
			},
		}); err != nil {
			t.Errorf("write response.completed failed: %v", err)
			return
		}
	}))
	defer wsServer.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "unit-test-agent/1.0")
	groupID := int64(1001)
	c.Set("api_key", &APIKey{GroupID: &groupID})

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 30
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 10
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}

	cache := &stubGatewayCache{}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            cache,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          9,
		Name:        "openai-ws",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"previous_response_id":"resp_prev_1","input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.Equal(t, "resp_new_1", result.RequestID)
	require.True(t, result.OpenAIWSMode)
	require.Equal(t, "session_bound", result.OpenAIWSProfile)
	require.False(t, gjson.GetBytes(upstream.lastBody, "model").Exists(), "WSv2 成功时不应回落 HTTP 上游")

	received := <-receivedCh
	require.Equal(t, "response.create", received.Type)
	require.Equal(t, "resp_prev_1", received.PreviousResponseID)
	require.True(t, received.StreamExists, "WS 请求应携带 stream 字段")
	require.False(t, received.Stream, "应保持客户端 stream=false 的原始语义")

	store := svc.getOpenAIWSStateStore()
	mappedAccountID, getErr := store.GetResponseAccount(context.Background(), groupID, 0, "resp_new_1")
	require.NoError(t, getErr)
	require.Equal(t, account.ID, mappedAccountID)
	connID, ok := store.GetResponseConn(groupID, 0, "resp_new_1")
	require.True(t, ok)
	require.NotEmpty(t, connID)

	responseBody := rec.Body.Bytes()
	require.Equal(t, "resp_new_1", gjson.GetBytes(responseBody, "id").String())
}

func TestOpenAIGatewayService_Forward_WSv2_UsesPatchedBodyAfterValidationDecode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type receivedPayload struct {
		MaxCompletionTokensExists bool
	}
	receivedCh := make(chan receivedPayload, 1)

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read ws request failed: %v", err)
			return
		}
		requestJSON := requestToJSONString(request)
		receivedCh <- receivedPayload{MaxCompletionTokensExists: gjson.Get(requestJSON, "max_completion_tokens").Exists()}

		if err := conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":    "resp_patched_ws_1",
				"model": "gpt-5.3-codex-spark",
				"usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
			},
		}); err != nil {
			t.Errorf("write response.completed failed: %v", err)
			return
		}
	}))
	defer wsServer.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "unit-test-agent/1.0")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 30
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 10

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          10,
		Name:        "openai-ws",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{"responses_websockets_v2_enabled": true},
	}

	body := []byte(`{"model":"gpt-5.4","stream":false,"max_completion_tokens":12,"tools":[{"type":"image_generation"}],"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OpenAIWSMode)

	received := <-receivedCh
	require.False(t, received.MaxCompletionTokensExists)
}

func TestOpenAIGatewayService_Forward_WSv2_ImageGenerationCountsOutputs(t *testing.T) {
	setGinTestMode()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read ws request failed: %v", err)
			return
		}

		if err := conn.WriteJSON(map[string]any{
			"type": "response.output_item.done",
			"item": map[string]any{
				"id":     "ig_ws_1",
				"type":   "image_generation_call",
				"result": "final-image",
			},
		}); err != nil {
			t.Errorf("write response.output_item.done failed: %v", err)
			return
		}
		if err := conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":    "resp_ws_image_1",
				"model": "gpt-5.4",
				"output": []any{
					map[string]any{
						"id":     "ig_ws_1",
						"type":   "image_generation_call",
						"result": "final-image",
					},
				},
				"usage": map[string]any{
					"input_tokens":  9,
					"output_tokens": 4,
				},
			},
		}); err != nil {
			t.Errorf("write response.completed failed: %v", err)
			return
		}
	}))
	defer wsServer.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	groupID := int64(1010)
	c.Set("api_key", &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			AllowImageGeneration: true,
		},
	})

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          10,
		Name:        "openai-ws-image",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.4","stream":false,"input":"draw","tools":[{"type":"image_generation","model":"gpt-image-2","size":"1024x1024"}],"tool_choice":{"type":"image_generation"}}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_ws_image_1", result.RequestID)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "1K", result.ImageSize)
	require.Equal(t, "gpt-image-2", result.BillingModel)
	require.Equal(t, 9, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.True(t, result.OpenAIWSMode)
	require.Equal(t, "resp_ws_image_1", gjson.GetBytes(rec.Body.Bytes(), "id").String())
}

func requestToJSONString(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func TestLogOpenAIWSBindResponseAccountWarn(t *testing.T) {
	require.NotPanics(t, func() {
		logOpenAIWSBindResponseAccountWarn(1, 2, "resp_ok", nil)
	})
	require.NotPanics(t, func() {
		logOpenAIWSBindResponseAccountWarn(1, 2, "resp_err", errors.New("bind failed"))
	})
}

func TestOpenAIGatewayService_Forward_WSv2_RewriteModelAndToolCallsOnCompletedEvent(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	groupID := int64(3001)
	c.Set("api_key", &APIKey{GroupID: &groupID})

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_model_tool_1","model":"gpt-5.1","tool_calls":[{"function":{"name":"apply_patch","arguments":"{\"file_path\":\"/tmp/a.txt\",\"old_string\":\"a\",\"new_string\":\"b\"}"}}],"usage":{"input_tokens":2,"output_tokens":1}},"tool_calls":[{"function":{"name":"apply_patch","arguments":"{\"file_path\":\"/tmp/a.txt\",\"old_string\":\"a\",\"new_string\":\"b\"}"}}]}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          1301,
		Name:        "openai-rewrite",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
			"model_mapping": map[string]any{
				"custom-original-model": "gpt-5.1",
			},
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"custom-original-model","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_model_tool_1", result.RequestID)
	require.Equal(t, "custom-original-model", gjson.GetBytes(rec.Body.Bytes(), "model").String(), "响应模型应回写为原始请求模型")
	require.Equal(t, "edit", gjson.GetBytes(rec.Body.Bytes(), "tool_calls.0.function.name").String(), "工具名称应被修正为 OpenCode 规范")
}

func TestOpenAIGatewayService_Forward_WSv2_NonStreamMaterializesOutputFromDoneEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "unit-test-agent/1.0")
	groupID := int64(3002)
	c.Set("api_key", &APIKey{GroupID: &groupID})

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","content":[],"summary":[]}}`),
			[]byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"msg_1","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"2"}]}}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_empty_terminal_output","object":"response","model":"gpt-5.5","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          1302,
		Name:        "openai-materialize-output",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.5","stream":false,"input":"Reply with exactly: 2"}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_empty_terminal_output", result.RequestID)
	require.Equal(t, 2, int(gjson.GetBytes(rec.Body.Bytes(), "output.#").Int()))
	require.Equal(t, "reasoning", gjson.GetBytes(rec.Body.Bytes(), "output.0.type").String())
	require.Equal(t, "2", gjson.GetBytes(rec.Body.Bytes(), "output.1.content.0.text").String())
}

func TestOpenAIWSPayloadString_OnlyAcceptsStringValues(t *testing.T) {
	payload := map[string]any{
		"type":                 nil,
		"model":                123,
		"prompt_cache_key":     " cache-key ",
		"previous_response_id": []byte(" resp_1 "),
	}

	require.Equal(t, "", openAIWSPayloadString(payload, "type"))
	require.Equal(t, "", openAIWSPayloadString(payload, "model"))
	require.Equal(t, "cache-key", openAIWSPayloadString(payload, "prompt_cache_key"))
	require.Equal(t, "resp_1", openAIWSPayloadString(payload, "previous_response_id"))
}

func TestOpenAIGatewayService_Forward_WSv2_SessionBoundWithoutAffinityDoesNotReuse(t *testing.T) {
	setGinTestMode()

	var upgradeCount atomic.Int64
	var sequence atomic.Int64
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgradeCount.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		for {
			var request map[string]any
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			idx := sequence.Add(1)
			responseID := "resp_reuse_" + strconv.FormatInt(idx, 10)
			if err := conn.WriteJSON(map[string]any{
				"type": "response.created",
				"response": map[string]any{
					"id":    responseID,
					"model": "gpt-5.1",
				},
			}); err != nil {
				return
			}
			if err := conn.WriteJSON(map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id":    responseID,
					"model": "gpt-5.1",
					"usage": map[string]any{
						"input_tokens":  2,
						"output_tokens": 1,
					},
				},
			}); err != nil {
				return
			}
		}
	}))
	defer wsServer.Close()

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 30
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 10

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}
	account := &Account{
		ID:          19,
		Name:        "openai-ws",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
		groupID := int64(2001)
		c.Set("api_key", &APIKey{GroupID: &groupID})

		body := []byte(`{"model":"gpt-5.1","stream":false,"previous_response_id":"resp_prev_reuse","input":[{"type":"input_text","text":"hello"}]}`)
		result, err := svc.Forward(context.Background(), c, account, body)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, strings.HasPrefix(result.RequestID, "resp_reuse_"))
	}

	// session_bound 连接的握手头携带请求级身份；无 response/session affinity 时不得按账号泛复用。
	require.Equal(t, int64(2), upgradeCount.Load(), "无 affinity 的 session_bound 请求应新建连接，不能泛复用上一请求的握手身份")
	metrics := svc.SnapshotOpenAIWSPoolMetrics()
	require.Equal(t, int64(0), metrics.AcquireReuseTotal)
	require.GreaterOrEqual(t, metrics.ConnPickTotal, int64(1))
}

func TestOpenAIGatewayService_Forward_WSv2_OAuthStoreFalseByDefault(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
	c.Request.Header.Set("originator", "codex_exec")
	c.Request.Header.Set("session_id", "sess-oauth-1")
	c.Request.Header.Set("conversation_id", "conv-oauth-1")
	c.Request.Header.Set("x-codex-window-id", "sess-oauth-1:0")
	c.Request.Header.Set("x-codex-beta-features", "memories,prevent_idle_sleep")
	c.Request.Header.Set("x-client-request-id", "client-req-1")
	c.Request.Header.Set("x-codex-installation-id", "install-1")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.AllowStoreRecovery = false
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          29,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "oauth-token-1",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"store":true,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_oauth_1", result.RequestID)

	require.NotNil(t, captureConn.lastWrite)
	requestJSON := requestToJSONString(captureConn.lastWrite)
	isolatedWindowID := isolateOpenAIWSWindowID(0, "sess-oauth-1:0")
	isolatedSessionID := isolateOpenAISessionID(0, "sess-oauth-1")
	isolatedConversationID := isolateOpenAISessionID(0, "conv-oauth-1")
	require.True(t, gjson.Get(requestJSON, "store").Exists(), "OAuth WSv2 应显式写入 store 字段")
	require.False(t, gjson.Get(requestJSON, "store").Bool(), "默认策略应将 OAuth store 置为 false")
	require.True(t, gjson.Get(requestJSON, "stream").Exists(), "WSv2 payload 应保留 stream 字段")
	require.True(t, gjson.Get(requestJSON, "stream").Bool(), "OAuth Codex 规范化后应强制 stream=true")
	require.Equal(t, "session_bound", result.OpenAIWSProfile)
	require.Equal(t, openAIWSBetaV2Value, captureDialer.lastHeaders.Get("OpenAI-Beta"))
	require.Equal(t, isolatedSessionID, captureDialer.lastHeaders.Get("session_id"))
	require.Equal(t, isolatedConversationID, captureDialer.lastHeaders.Get("conversation_id"))
	require.Equal(t, isolatedWindowID, captureDialer.lastHeaders.Get("x-codex-window-id"))
	require.Equal(t, "memories,prevent_idle_sleep", captureDialer.lastHeaders.Get("x-codex-beta-features"))
	require.Equal(t, "client-req-1", captureDialer.lastHeaders.Get("x-client-request-id"))
	require.Equal(t, "install-1", captureDialer.lastHeaders.Get("x-codex-installation-id"))
	require.Equal(t, "codex_exec/0.124.0", captureDialer.lastHeaders.Get("user-agent"))
	require.Equal(t, "codex_exec", captureDialer.lastHeaders.Get("originator"))
	require.Equal(t, "install-1", gjson.Get(requestJSON, "client_metadata.x-codex-installation-id").String())
	require.Equal(t, isolatedWindowID, gjson.Get(requestJSON, "client_metadata.x-codex-window-id").String())
	require.Equal(t, "memories,prevent_idle_sleep", gjson.Get(requestJSON, "client_metadata.x-codex-beta-features").String())
	require.Equal(t, "client-req-1", gjson.Get(requestJSON, "client_metadata.x-client-request-id").String())
}

func TestOpenAIGatewayService_Forward_WSv2_OAuthStickyPreviousResponseKeepsStoreFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_sticky_prev_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_sticky_prev_2","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":2}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          130,
		Name:        "openai-oauth-sticky-prev",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"access_token": "oauth-token-sticky-prev",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	groupID := int64(13001)
	apiKeyID := int64(13002)
	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-oauth-sticky-prev")
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	firstBody := []byte(`{"model":"gpt-5.1","stream":false,"store":true,"input":[{"type":"input_text","text":"hello"}]}`)
	firstResult, err := svc.Forward(context.Background(), newContext(), account, firstBody)
	require.NoError(t, err)
	require.NotNil(t, firstResult)
	require.Equal(t, "resp_oauth_sticky_prev_1", firstResult.RequestID)

	secondBody := []byte(`{"model":"gpt-5.1","stream":false,"previous_response_id":"resp_oauth_sticky_prev_1","input":[{"type":"input_text","text":"continue"}]}`)
	secondResult, err := svc.Forward(context.Background(), newContext(), account, secondBody)
	require.NoError(t, err)
	require.NotNil(t, secondResult)
	require.Equal(t, "resp_oauth_sticky_prev_2", secondResult.RequestID)

	require.Equal(t, 1, captureDialer.DialCount(), "sticky previous_response_id 应复用 response_id 绑定的连接")

	captureConn.mu.Lock()
	writes := append([]map[string]any(nil), captureConn.writes...)
	captureConn.mu.Unlock()
	require.Len(t, writes, 2)

	firstWrite := requestToJSONString(writes[0])
	require.True(t, gjson.Get(firstWrite, "store").Exists())
	require.False(t, gjson.Get(firstWrite, "store").Bool(), "OAuth 首轮无 previous_response_id 仍应 store=false")
	require.False(t, gjson.Get(firstWrite, "previous_response_id").Exists())

	secondWrite := requestToJSONString(writes[1])
	require.Equal(t, "resp_oauth_sticky_prev_1", gjson.Get(secondWrite, "previous_response_id").String())
	require.True(t, gjson.Get(secondWrite, "store").Exists(), "sticky OAuth continuation should keep explicit store=false")
	require.False(t, gjson.Get(secondWrite, "store").Bool(), "sticky OAuth continuation must stay on store=false")
}

func TestOpenAIGatewayService_Forward_WSv2_OAuthColdSessionUsesNeutralIdleConn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_cold_neutral_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          131,
		Name:        "openai-oauth-cold-neutral",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"access_token": "oauth-token-cold-neutral",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	wsURL, err := svc.buildOpenAIResponsesWSURL(account)
	require.NoError(t, err)
	decision := OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2}
	neutralHeaders := svc.buildOpenAIWSNeutralHeaders(account, account.GetOpenAIAccessToken(), decision, true)
	seedLease, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   wsURL,
		Headers: neutralHeaders,
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	seedConnID := seedLease.ConnID()
	seedLease.Release()
	require.Equal(t, 1, captureDialer.DialCount(), "预置一条 neutral idle 连接")

	groupID := int64(13101)
	apiKeyID := int64(13102)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
	c.Request.Header.Set("originator", "codex_exec")
	c.Request.Header.Set("session_id", "sess-oauth-cold-neutral")
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
	sessionHash := svc.GenerateSessionHash(c, nil)

	body := []byte(`{"model":"gpt-5.1","stream":false,"prompt_cache_key":"pcache-cold-neutral","input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_oauth_cold_neutral_1", result.RequestID)
	require.Equal(t, "session_bound", result.OpenAIWSProfile)
	require.False(t, result.OpenAIWSConnReused, "当前 session 绑定语义下，带显式 session 信号的 store=false 首轮请求会新建 session_bound 连接")
	require.Equal(t, 2, captureDialer.DialCount(), "当前 strict store=false 语义下不会直接复用预热 neutral idle 连接")

	connID, ok := svc.getOpenAIWSStateStore().GetSessionConn(groupID, sessionHash)
	require.True(t, ok, "冷 session 完成后仍应绑定实际连接用于后续亲和")
	require.NotEqual(t, seedConnID, connID)
	profile, ok := pool.ConnProfile(account.ID, seedConnID)
	require.True(t, ok)
	require.Equal(t, openAIWSConnProfileNeutral, profile, "未被复用的预热 neutral 连接应保持 neutral profile")
	profile, ok = pool.ConnProfile(account.ID, connID)
	require.True(t, ok)
	require.Equal(t, openAIWSConnProfileSessionBound, profile)
}

func TestOpenAIGatewayService_Forward_WSv2AccountOnlyPreviousResponseContinuesOnFreshConn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 3
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	neutralSeed := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_reused_neutral_should_not_use","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
		},
	}
	dialer := &openAIWSSequentialCaptureDialer{
		conns: []*openAIWSCaptureConn{neutralSeed},
	}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	t.Cleanup(pool.Close)

	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          132,
		Name:        "openai-oauth-prev-response-recover",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 3,
		Credentials: map[string]any{
			"access_token": "oauth-token-prev-response-recover",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	wsURL, err := svc.buildOpenAIResponsesWSURL(account)
	require.NoError(t, err)
	decision := OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2}
	neutralHeaders := svc.buildOpenAIWSNeutralHeaders(account, account.GetOpenAIAccessToken(), decision, true)
	seedLease, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   wsURL,
		Headers: neutralHeaders,
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	seedLease.Release()
	require.Equal(t, 1, dialer.DialCount(), "预置一条 neutral idle 连接")

	groupID := int64(13201)
	apiKeyID := int64(13202)
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(
		context.Background(),
		groupID,
		apiKeyID,
		"resp_missing",
		account.ID,
		time.Hour,
	))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
	c.Request.Header.Set("originator", "codex_exec")
	c.Request.Header.Set("session_id", "sess-oauth-prev-response-recover")
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})

	body := []byte(`{"model":"gpt-5.1","stream":false,"prompt_cache_key":"pcache-prev-response-recover","previous_response_id":"resp_missing","input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_reused_neutral_should_not_use", result.RequestID)
	require.Equal(t, "session_bound", result.OpenAIWSProfile)
	require.Nil(t, upstream.lastReq, "account-only previous_response_id continuation must stay on WS, not HTTP fallback")
	require.Equal(t, 2, dialer.DialCount(), "sticky account 命中但无 conn 绑定时应允许 fresh conn continuation")

	neutralSeed.mu.Lock()
	neutralWrites := append([]map[string]any(nil), neutralSeed.writes...)
	neutralSeed.mu.Unlock()
	require.Len(t, neutralWrites, 1)
	continuationWrite := requestToJSONString(neutralWrites[0])
	require.Equal(t, "resp_missing", gjson.Get(continuationWrite, "previous_response_id").String(), "account-only binding should keep previous_response_id on the fresh WS")
	require.True(t, gjson.Get(continuationWrite, "store").Exists())
	require.False(t, gjson.Get(continuationWrite, "store").Bool())
	require.Equal(t, "hello", gjson.Get(continuationWrite, "input.0.text").String())
}

func TestOpenAIGatewayService_Forward_WSv2_SameSessionPreemptsInFlightRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 10
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	firstReadBlocked := make(chan struct{})
	firstConn := &openAIWSCaptureConn{
		readDelayStarted: firstReadBlocked,
		events: [][]byte{
			[]byte(`{"type":"response.output_text.delta","delta":"partial"}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_preempted","model":"gpt-5.5","usage":{"input_tokens":3,"output_tokens":2}}}`),
		},
		readDelays: []time.Duration{0, 5 * time.Second},
	}
	secondConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_replacement","model":"gpt-5.5","usage":{"input_tokens":4,"output_tokens":3}}}`),
		},
	}
	captureDialer := &openAIWSSequentialCaptureDialer{conns: []*openAIWSCaptureConn{firstConn, secondConn}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          132,
		Name:        "openai-oauth-session-preempt",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"access_token": "oauth-token-session-preempt",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	groupID := int64(13201)
	apiKeyID := int64(13202)
	body := []byte(`{"model":"gpt-5.5","stream":true,"prompt_cache_key":"pcache-preempt","input":[{"type":"input_text","text":"hello"}]}`)
	newContext := func(clientRequestID string) (*gin.Context, *httptest.ResponseRecorder) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-preempt")
		c.Request.Header.Set("x-client-request-id", clientRequestID)
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c, rec
	}

	firstCtx, _ := newContext("client-preempt-1")
	firstDone := make(chan error, 1)
	go func() {
		_, err := svc.Forward(context.Background(), firstCtx, account, body)
		firstDone <- err
	}()

	require.Eventually(t, func() bool {
		select {
		case <-firstReadBlocked:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond, "first request should be blocked in upstream read")

	secondCtx, _ := newContext("client-preempt-2")
	secondResult, secondErr := svc.Forward(context.Background(), secondCtx, account, body)
	require.NoError(t, secondErr)
	require.NotNil(t, secondResult)
	require.Equal(t, "resp_replacement", secondResult.RequestID)

	select {
	case firstErr := <-firstDone:
		require.Error(t, firstErr)
		var fallbackErr *openAIWSFallbackError
		require.ErrorAs(t, firstErr, &fallbackErr)
		require.Equal(t, "session_preempted", fallbackErr.Reason)
	case <-time.After(time.Second):
		t.Fatal("first same-session request was not preempted promptly")
	}

	firstConn.mu.Lock()
	firstClosed := firstConn.closed
	firstConn.mu.Unlock()
	require.True(t, firstClosed, "preempted request must close its upstream WS instead of returning it to the pool")
}

func TestOpenAIGatewayService_Forward_WSv2_OAuthUnboundPreviousResponseFallsBackStoreFalseNewConn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	firstConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_unbound_seed","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
		},
	}
	secondConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_unbound_full","model":"gpt-5.1","usage":{"input_tokens":4,"output_tokens":2}}}`),
		},
	}
	dialer := &openAIWSQueueDialer{
		conns: []openAIWSClientConn{firstConn, secondConn},
	}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          131,
		Name:        "openai-oauth-unbound-prev",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"access_token": "oauth-token-unbound-prev",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	groupID := int64(13101)
	apiKeyID := int64(13102)
	newContext := func(sessionID string) *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", sessionID)
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	seedBody := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"seed"}]}`)
	seedResult, err := svc.Forward(context.Background(), newContext("sess-oauth-unbound-seed"), account, seedBody)
	require.NoError(t, err)
	require.NotNil(t, seedResult)
	require.Equal(t, "resp_oauth_unbound_seed", seedResult.RequestID)

	unboundBody := []byte(`{"model":"gpt-5.1","stream":false,"previous_response_id":"resp_external_unbound","input":[{"type":"input_text","text":"full replay from client"}]}`)
	unboundResult, err := svc.Forward(context.Background(), newContext("sess-oauth-unbound-new"), account, unboundBody)
	require.NoError(t, err)
	require.NotNil(t, unboundResult)
	require.Equal(t, "resp_oauth_unbound_full", unboundResult.RequestID)

	require.Equal(t, 2, dialer.DialCount(), "未绑定 previous_response_id 不应借用已有 session-bound 连接")

	firstConn.mu.Lock()
	firstWrites := append([]map[string]any(nil), firstConn.writes...)
	firstConn.mu.Unlock()
	require.Len(t, firstWrites, 1, "已有 session-bound 连接不应收到 unbound 续链请求")

	secondConn.mu.Lock()
	secondWrites := append([]map[string]any(nil), secondConn.writes...)
	secondConn.mu.Unlock()
	require.Len(t, secondWrites, 1)
	secondWrite := requestToJSONString(secondWrites[0])
	require.False(t, gjson.Get(secondWrite, "previous_response_id").Exists(), "无法证明同账号粘连时应降级为 full create")
	require.True(t, gjson.Get(secondWrite, "store").Exists())
	require.False(t, gjson.Get(secondWrite, "store").Bool(), "无法证明同账号粘连时应显式 store=false")
	require.Equal(t, "full replay from client", gjson.Get(secondWrite, "input.0.text").String())
}

func TestOpenAIGatewayService_Forward_WSv2_OAuthStickyAccountWithoutConnContinuesOnNewConn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	firstConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_account_only_seed","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
		},
	}
	secondConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_account_only_next","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":2}}}`),
		},
	}
	dialer := &openAIWSQueueDialer{
		conns: []openAIWSClientConn{firstConn, secondConn},
	}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          132,
		Name:        "openai-oauth-account-only-prev",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"access_token": "oauth-token-account-only-prev",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	groupID := int64(13201)
	apiKeyID := int64(13202)
	newContext := func(sessionID string) *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", sessionID)
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	seedBody := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"seed"}]}`)
	seedResult, err := svc.Forward(context.Background(), newContext("sess-oauth-account-only-seed"), account, seedBody)
	require.NoError(t, err)
	require.NotNil(t, seedResult)
	require.Equal(t, "resp_oauth_account_only_seed", seedResult.RequestID)

	stateStore := svc.getOpenAIWSStateStore()
	require.NoError(t, stateStore.BindResponseAccount(context.Background(), groupID, apiKeyID, "resp_oauth_account_only_prev", account.ID, time.Hour))
	stateStore.DeleteResponseConn(groupID, apiKeyID, "resp_oauth_account_only_prev")

	nextBody := []byte(`{"model":"gpt-5.1","stream":false,"previous_response_id":"resp_oauth_account_only_prev","input":[{"type":"input_text","text":"delta only"}]}`)
	nextResult, err := svc.Forward(context.Background(), newContext("sess-oauth-account-only-next"), account, nextBody)
	require.NoError(t, err)
	require.NotNil(t, nextResult)
	require.Equal(t, "resp_oauth_account_only_next", nextResult.RequestID)

	require.Equal(t, 2, dialer.DialCount(), "账号粘连有效但 conn 亲和缺失时应新建连接，不能借用其它 session-bound 连接")

	firstConn.mu.Lock()
	firstWrites := append([]map[string]any(nil), firstConn.writes...)
	firstConn.mu.Unlock()
	require.Len(t, firstWrites, 1)

	secondConn.mu.Lock()
	secondWrites := append([]map[string]any(nil), secondConn.writes...)
	secondConn.mu.Unlock()
	require.Len(t, secondWrites, 1)
	secondWrite := requestToJSONString(secondWrites[0])
	require.Equal(t, "resp_oauth_account_only_prev", gjson.Get(secondWrite, "previous_response_id").String(), "account-only binding should keep previous_response_id without conn affinity")
	require.True(t, gjson.Get(secondWrite, "store").Exists())
	require.False(t, gjson.Get(secondWrite, "store").Bool(), "account-only binding must stay on store=false incremental semantics")
	require.Equal(t, "delta only", gjson.Get(secondWrite, "input.0.text").String())
}

func TestOpenAIGatewayService_BuildOpenAIWSCreatePayload_DropsUnpersistedReasoningItemsWhenStoreFalse(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{
		Type: AccountTypeOAuth,
	}
	reqBody := map[string]any{
		"model": "gpt-5.1",
		"store": false,
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hi"},
			map[string]any{"type": "reasoning", "id": "rs_0672f12450da0b9c0169f07220a6c08198b68c2455ced99344", "summary": []any{}},
			map[string]any{"type": "function_call_output", "call_id": "call_123", "output": "done"},
		},
	}

	payload := svc.buildOpenAIWSCreatePayload(reqBody, account)
	input, ok := payload["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)
	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "message", first["type"])
	require.Equal(t, "function_call_output", second["type"])
	require.False(t, gjson.Get(requestToJSONString(payload), `input.#(type=="reasoning")`).Exists())
	require.False(t, gjson.Get(requestToJSONString(payload), "store").Bool())
}

func TestOpenAIGatewayService_BuildOpenAIWSCreatePayload_StripsUnsupportedParameters(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{
		Type: AccountTypeOAuth,
	}
	reqBody := map[string]any{
		"model":          "gpt-5.5",
		"input":          []any{map[string]any{"role": "user", "content": "hi"}},
		"instructions":   "be brief",
		"repeat_penalty": 1.1,
		"top_k":          40,
		"store":          false,
	}

	payload := svc.buildOpenAIWSCreatePayload(reqBody, account)
	payloadJSON := requestToJSONString(payload)
	require.False(t, gjson.Get(payloadJSON, "repeat_penalty").Exists())
	require.False(t, gjson.Get(payloadJSON, "top_k").Exists())
	require.Equal(t, "gpt-5.5", gjson.Get(payloadJSON, "model").String())
	require.Equal(t, "be brief", gjson.Get(payloadJSON, "instructions").String())
}

func TestOpenAIGatewayService_BuildOpenAIWSCreatePayload_ExtractsSystemMessages(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{
		Type: AccountTypeOAuth,
	}
	reqBody := map[string]any{
		"model":        "gpt-5.5",
		"instructions": "existing instructions",
		"store":        false,
		"input": []any{
			map[string]any{
				"type":    "message",
				"role":    "system",
				"content": "repo policy",
			},
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": "hello",
			},
		},
	}

	payload := svc.buildOpenAIWSCreatePayload(reqBody, account)
	payloadJSON := requestToJSONString(payload)
	require.False(t, gjson.Get(payloadJSON, `input.#(role=="system")`).Exists())
	require.Equal(t, "repo policy\n\nexisting instructions", gjson.Get(payloadJSON, "instructions").String())
	require.Equal(t, "user", gjson.Get(payloadJSON, "input.0.role").String())
	require.Equal(t, "hello", gjson.Get(payloadJSON, "input.0.content").String())
}

func TestOpenAIGatewayService_Forward_WSv2_OAuthOriginatorCompatibility(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		name           string
		userAgent      string
		originator     string
		wantOriginator string
	}{
		{name: "desktop originator preserved", originator: "Codex Desktop", wantOriginator: "Codex Desktop"},
		{name: "vscode originator preserved", originator: "codex_vscode", wantOriginator: "codex_vscode"},
		{name: "official ua fallback to codex_cli_rs", userAgent: "Codex Desktop/1.2.3", wantOriginator: "codex_cli_rs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
			if tt.userAgent != "" {
				c.Request.Header.Set("User-Agent", tt.userAgent)
			}
			if tt.originator != "" {
				c.Request.Header.Set("originator", tt.originator)
			}

			cfg := &config.Config{}
			cfg.Security.URLAllowlist.Enabled = false
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			cfg.Security.URLAllowlist.AllowPrivateHosts = true
			cfg.Gateway.OpenAIWS.Enabled = true
			cfg.Gateway.OpenAIWS.OAuthEnabled = true
			cfg.Gateway.OpenAIWS.APIKeyEnabled = true
			cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
			cfg.Gateway.OpenAIWS.AllowStoreRecovery = false
			cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
			cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
			cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

			captureConn := &openAIWSCaptureConn{
				events: [][]byte{
					[]byte(`{"type":"response.completed","response":{"id":"resp_oauth_originator","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`),
				},
			}
			captureDialer := &openAIWSCaptureDialer{conn: captureConn}
			pool := newOpenAIWSConnPool(cfg)
			pool.setClientDialerForTest(captureDialer)

			svc := &OpenAIGatewayService{
				cfg:              cfg,
				httpUpstream:     &httpUpstreamRecorder{},
				cache:            &stubGatewayCache{},
				openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
				toolCorrector:    NewCodexToolCorrector(),
				openaiWSPool:     pool,
			}
			account := &Account{
				ID:          129,
				Name:        "openai-oauth",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Credentials: map[string]any{
					"access_token": "oauth-token-1",
				},
				Extra: map[string]any{
					"responses_websockets_v2_enabled": true,
				},
			}

			body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.wantOriginator, captureDialer.lastHeaders.Get("originator"))
		})
	}
}

func TestOpenAIGatewayService_BuildOpenAIWSHeaders_NonOfficialOAuthPreservesUserAgentWithoutForceCodexCLI(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "custom-client/1.0")
	c.Request.Header.Set("Session_Id", "sess-nonofficial")

	cfg := &config.Config{}
	svc := &OpenAIGatewayService{cfg: cfg}
	account := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
	}

	headers, _ := svc.buildOpenAIWSHeaders(c, account, "token", OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2}, false, "", "", "")
	require.Equal(t, "custom-client/1.0", headers.Get("User-Agent"))
	require.Equal(t, "opencode", headers.Get("Originator"))
	require.Equal(t, openAIWSBetaV2Value, headers.Get("OpenAI-Beta"))
}

func TestOpenAIGatewayService_Forward_WSv2_HeaderSessionFallbackFromPromptCacheKey(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_prompt_cache_key","model":"gpt-5.1","usage":{"input_tokens":2,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          31,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "oauth-token-1",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":true,"prompt_cache_key":"pcache_123","input":[{"type":"input_text","text":"hi"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_prompt_cache_key", result.RequestID)
	require.Equal(t, "session_bound", result.OpenAIWSProfile)

	require.Equal(t, isolateOpenAISessionID(0, "pcache_123"), captureDialer.lastHeaders.Get("session_id"))
	require.Empty(t, captureDialer.lastHeaders.Get("conversation_id"))
	require.NotNil(t, captureConn.lastWrite)
	require.True(t, gjson.Get(requestToJSONString(captureConn.lastWrite), "stream").Exists())
	sessionHash, _ := deriveOpenAIRequestScopedSessionHashes(c, "pcache_123")
	connID, ok := svc.getOpenAIWSStateStore().GetSessionConn(0, sessionHash)
	require.True(t, ok)
	profile, ok := pool.ConnProfile(account.ID, connID)
	require.True(t, ok)
	require.Equal(t, openAIWSConnProfileSessionBound, profile)
}

func TestOpenAIGatewayService_Forward_WSv2_UsesRoutingPromptCacheKeyWhenPayloadKeyWasStripped(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	groupID := int64(1401)
	apiKeyID := int64(2401)
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
	c.Set(openAIRoutingPromptCacheKeyKey, "pcache_routing_only")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.HttpIngressUpstreamWSEnabled = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_routing_prompt_cache","model":"gpt-5.1","usage":{"input_tokens":2,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          32,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "oauth-token-1",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	reqBody := map[string]any{
		"model":  "gpt-5.1",
		"stream": true,
		"store":  false,
		"input": []any{
			map[string]any{"type": "input_text", "text": "hi"},
		},
	}
	result, err := svc.forwardOpenAIWSV2(
		context.Background(),
		c,
		account,
		reqBody,
		"oauth-token-1",
		OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2},
		true,
		true,
		"gpt-5.1",
		"gpt-5.1",
		time.Now(),
		1,
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_routing_prompt_cache", result.RequestID)
	_, oneShot := c.Get("openai_http_ingress_ws_one_shot")
	require.False(t, oneShot, "routing prompt_cache_key must prevent HTTP ingress one-shot even when upstream payload key was stripped")

	require.NotNil(t, captureConn.lastWrite)
	requestJSON := requestToJSONString(captureConn.lastWrite)
	require.False(t, gjson.Get(requestJSON, "prompt_cache_key").Exists(), "routing-only prompt_cache_key must not be re-added to upstream payload")
	sessionHash, _ := deriveOpenAIRequestScopedSessionHashes(c, "pcache_routing_only")
	connID, ok := svc.getOpenAIWSStateStore().GetSessionConn(groupID, sessionHash)
	require.True(t, ok, "routing prompt_cache_key should bind a session connection")
	require.NotEmpty(t, connID)
	profile, ok := pool.ConnProfile(account.ID, connID)
	require.True(t, ok)
	require.Equal(t, openAIWSConnProfileSessionBound, profile)
}

func TestOpenAIGatewayService_Forward_WSv2_PreservesTurnStateAndMetadata(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	c.Request.Header.Set("x-codex-turn-state", "turn-state-1")
	c.Request.Header.Set("x-codex-turn-metadata", "turn-metadata-1")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_turn_state","model":"gpt-5.1","usage":{"input_tokens":2,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          32,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "oauth-token-1",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":true,"prompt_cache_key":"pcache_turn_state","input":[{"type":"input_text","text":"hi"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_turn_state", result.RequestID)

	require.Equal(t, "turn-state-1", captureDialer.lastHeaders.Get("x-codex-turn-state"))
	require.Equal(t, "turn-metadata-1", captureDialer.lastHeaders.Get("x-codex-turn-metadata"))

	clientMetadata, ok := captureConn.lastWrite["client_metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "turn-metadata-1", clientMetadata["x-codex-turn-metadata"])
}

func TestOpenAIGatewayService_Forward_WSv1_Unsupported(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsockets = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = false

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          39,
		Name:        "openai-ws-v1",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.openai.com/v1/responses",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"previous_response_id":"resp_prev_v1","input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "ws v1")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "WSv1")
	require.Nil(t, upstream.lastReq, "WSv1 不支持时不应触发 HTTP 上游请求")
}

func TestOpenAIGatewayService_Forward_WSv2_TurnStateAndMetadataReplayOnReconnect(t *testing.T) {
	setGinTestMode()

	var connIndex atomic.Int64
	headersCh := make(chan http.Header, 4)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := connIndex.Add(1)
		headersCh <- cloneHeader(r.Header)

		respHeader := http.Header{}
		if idx == 1 {
			respHeader.Set("x-codex-turn-state", "turn_state_first")
		}
		conn, err := upgrader.Upgrade(w, r, respHeader)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read ws request failed: %v", err)
			return
		}
		responseID := "resp_turn_" + strconv.FormatInt(idx, 10)
		if err := conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":    responseID,
				"model": "gpt-5.1",
				"usage": map[string]any{
					"input_tokens":  2,
					"output_tokens": 1,
				},
			},
		}); err != nil {
			t.Errorf("write response.completed failed: %v", err)
			return
		}
	}))
	defer wsServer.Close()

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 0

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          49,
		Name:        "openai-turn-state",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	reqBody := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	rec1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(rec1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c1.Request.Header.Set("session_id", "session_turn_state")
	c1.Request.Header.Set("x-codex-turn-metadata", "turn_meta_1")
	result1, err := svc.Forward(context.Background(), c1, account, reqBody)
	require.NoError(t, err)
	require.NotNil(t, result1)

	sessionHash := svc.GenerateSessionHash(c1, reqBody)
	store := svc.getOpenAIWSStateStore()
	turnState, ok := store.GetSessionTurnState(0, sessionHash)
	require.True(t, ok)
	require.Equal(t, "turn_state_first", turnState)

	// 主动淘汰连接，模拟下一次请求发生重连。
	connID, hasConn := store.GetResponseConn(0, 0, result1.RequestID)
	require.True(t, hasConn)
	svc.getOpenAIWSConnPool().evictConn(account.ID, connID)

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c2.Request.Header.Set("session_id", "session_turn_state")
	c2.Request.Header.Set("x-codex-turn-metadata", "turn_meta_2")
	result2, err := svc.Forward(context.Background(), c2, account, reqBody)
	require.NoError(t, err)
	require.NotNil(t, result2)

	firstHandshakeHeaders := <-headersCh
	secondHandshakeHeaders := <-headersCh
	require.Equal(t, "turn_meta_1", firstHandshakeHeaders.Get("X-Codex-Turn-Metadata"))
	require.Equal(t, "turn_meta_2", secondHandshakeHeaders.Get("X-Codex-Turn-Metadata"))
	require.Equal(t, "turn_state_first", secondHandshakeHeaders.Get("X-Codex-Turn-State"))
}

func TestOpenAIGatewayService_Forward_WSv2_GeneratePrewarm(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("session_id", "session-prewarm")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.PrewarmGenerateEnabled = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_prewarm_1","model":"gpt-5.1","usage":{"input_tokens":0,"output_tokens":0}}}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_main_1","model":"gpt-5.1","usage":{"input_tokens":4,"output_tokens":2}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          59,
		Name:        "openai-prewarm",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_main_1", result.RequestID)

	require.Len(t, captureConn.writes, 2, "开启 generate=false 预热后应发送两次 WS 请求")
	firstWrite := requestToJSONString(captureConn.writes[0])
	secondWrite := requestToJSONString(captureConn.writes[1])
	require.True(t, gjson.Get(firstWrite, "generate").Exists())
	require.False(t, gjson.Get(firstWrite, "generate").Bool())
	require.False(t, gjson.Get(secondWrite, "generate").Exists())
}

func TestOpenAIGatewayService_PrewarmReadHonorsParentContext(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.PrewarmGenerateEnabled = true
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	svc := &OpenAIGatewayService{
		cfg:           cfg,
		toolCorrector: NewCodexToolCorrector(),
	}
	account := &Account{
		ID:          601,
		Name:        "openai-prewarm-timeout",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
	}
	conn := newOpenAIWSConn("prewarm_ctx_conn", account.ID, &openAIWSBlockingConn{
		readDelay: 200 * time.Millisecond,
	}, nil)
	lease := &openAIWSConnLease{
		accountID: account.ID,
		conn:      conn,
	}
	payload := map[string]any{
		"type":  "response.create",
		"model": "gpt-5.1",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := svc.performOpenAIWSGeneratePrewarm(
		ctx,
		lease,
		OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2},
		payload,
		"",
		map[string]any{"model": "gpt-5.1"},
		account,
		nil,
		0,
		0,
	)
	elapsed := time.Since(start)
	require.Error(t, err)
	require.Contains(t, err.Error(), "prewarm_read_event")
	require.Less(t, elapsed, 180*time.Millisecond, "预热读取应受父 context 取消控制，不应阻塞到 read_timeout")
}

func TestOpenAIGatewayService_Forward_WSv2_TurnMetadataInPayloadOnConnReuse(t *testing.T) {
	setGinTestMode()

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_meta_1","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_meta_2","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          69,
		Name:        "openai-turn-metadata",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)

	rec1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(rec1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c1.Request.Header.Set("session_id", "session-metadata-reuse")
	c1.Request.Header.Set("x-codex-turn-metadata", "turn_meta_payload_1")
	c1.Request.Header.Set("x-codex-installation-id", "install-meta")
	c1.Request.Header.Set("x-codex-window-id", "session-metadata-reuse:0")
	c1.Request.Header.Set("x-codex-beta-features", "memories,prevent_idle_sleep")
	c1.Request.Header.Set("x-client-request-id", "client-req-meta-1")
	result1, err := svc.Forward(context.Background(), c1, account, body)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.Equal(t, "resp_meta_1", result1.RequestID)

	require.Len(t, captureConn.writes, 1)
	firstWrite := requestToJSONString(captureConn.writes[0])
	require.Equal(t, "turn_meta_payload_1", gjson.Get(firstWrite, "client_metadata.x-codex-turn-metadata").String())
	require.False(t, gjson.Get(firstWrite, "client_metadata.x-codex-installation-id").Exists())
	require.False(t, gjson.Get(firstWrite, "client_metadata.x-codex-window-id").Exists())
	require.False(t, gjson.Get(firstWrite, "client_metadata.x-codex-beta-features").Exists())
	require.False(t, gjson.Get(firstWrite, "client_metadata.x-client-request-id").Exists())

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c2.Request.Header.Set("session_id", "session-metadata-reuse")
	c2.Request.Header.Set("x-codex-turn-metadata", "turn_meta_payload_2")
	c2.Request.Header.Set("x-codex-installation-id", "install-meta")
	c2.Request.Header.Set("x-codex-window-id", "session-metadata-reuse:1")
	c2.Request.Header.Set("x-codex-beta-features", "memories,prevent_idle_sleep")
	c2.Request.Header.Set("x-client-request-id", "client-req-meta-2")
	body2 := []byte(`{"model":"gpt-5.1","stream":false,"store":true,"previous_response_id":"resp_meta_1","input":[{"type":"input_text","text":"hello again"}]}`)
	result2, err := svc.Forward(context.Background(), c2, account, body2)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.Equal(t, "resp_meta_2", result2.RequestID)

	require.Equal(t, 1, captureDialer.DialCount(), "previous_response_id 绑定命中时应复用同一 WS 连接")
	require.Len(t, captureConn.writes, 2)

	firstWrite = requestToJSONString(captureConn.writes[0])
	secondWrite := requestToJSONString(captureConn.writes[1])
	require.Equal(t, "turn_meta_payload_1", gjson.Get(firstWrite, "client_metadata.x-codex-turn-metadata").String())
	require.Equal(t, "turn_meta_payload_2", gjson.Get(secondWrite, "client_metadata.x-codex-turn-metadata").String())
	require.False(t, gjson.Get(secondWrite, "client_metadata.x-codex-installation-id").Exists())
	require.False(t, gjson.Get(secondWrite, "client_metadata.x-codex-window-id").Exists())
	require.False(t, gjson.Get(secondWrite, "client_metadata.x-codex-beta-features").Exists())
	require.False(t, gjson.Get(secondWrite, "client_metadata.x-client-request-id").Exists())
}

func TestOpenAIGatewayService_Forward_WSv2StoreFalseSessionConnIsolation(t *testing.T) {
	setGinTestMode()

	var upgradeCount atomic.Int64
	var sequence atomic.Int64
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgradeCount.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		for {
			var request map[string]any
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			responseID := "resp_store_false_" + strconv.FormatInt(sequence.Add(1), 10)
			if err := conn.WriteJSON(map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id":    responseID,
					"model": "gpt-5.1",
					"usage": map[string]any{
						"input_tokens":  1,
						"output_tokens": 1,
					},
				},
			}); err != nil {
				return
			}
		}
	}))
	defer wsServer.Close()

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StoreDisabledForceNewConn = true

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          79,
		Name:        "openai-store-false",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"store":false,"input":[{"type":"input_text","text":"hello"}]}`)

	rec1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(rec1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c1.Request.Header.Set("session_id", "session_store_false_a")
	result1, err := svc.Forward(context.Background(), c1, account, body)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.Equal(t, int64(1), upgradeCount.Load())

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c2.Request.Header.Set("session_id", "session_store_false_a")
	result2, err := svc.Forward(context.Background(), c2, account, body)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.Equal(t, int64(1), upgradeCount.Load(), "同一 session(store=false) 应复用同一 WS 连接")

	rec3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(rec3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c3.Request.Header.Set("session_id", "session_store_false_b")
	result3, err := svc.Forward(context.Background(), c3, account, body)
	require.NoError(t, err)
	require.NotNil(t, result3)
	require.Equal(t, int64(2), upgradeCount.Load(), "不同 session(store=false) 应隔离连接，避免续链状态互相覆盖")
}

func TestOpenAIGatewayService_Forward_WSv2StoreFalseDisableForceNewConnAllowsReuse(t *testing.T) {
	setGinTestMode()

	var upgradeCount atomic.Int64
	var sequence atomic.Int64
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgradeCount.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		for {
			var request map[string]any
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			responseID := "resp_store_false_reuse_" + strconv.FormatInt(sequence.Add(1), 10)
			if err := conn.WriteJSON(map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id":    responseID,
					"model": "gpt-5.1",
					"usage": map[string]any{
						"input_tokens":  1,
						"output_tokens": 1,
					},
				},
			}); err != nil {
				return
			}
		}
	}))
	defer wsServer.Close()

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.StoreDisabledForceNewConn = false

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	account := &Account{
		ID:          80,
		Name:        "openai-store-false-reuse",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"store":false,"input":[{"type":"input_text","text":"hello"}]}`)

	rec1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(rec1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c1.Request.Header.Set("session_id", "session_store_false_reuse_a")
	result1, err := svc.Forward(context.Background(), c1, account, body)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.Equal(t, int64(1), upgradeCount.Load())

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c2.Request.Header.Set("session_id", "session_store_false_reuse_b")
	result2, err := svc.Forward(context.Background(), c2, account, body)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.Equal(t, int64(2), upgradeCount.Load(), "关闭强制新连后，不同 session(store=false) 也不得泛复用 session-bound 连接")
}

func TestOpenAIGatewayService_Forward_WSv2ReadTimeoutAppliesPerRead(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 1
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	captureConn := &openAIWSCaptureConn{
		readDelays: []time.Duration{
			700 * time.Millisecond,
			700 * time.Millisecond,
		},
		events: [][]byte{
			[]byte(`{"type":"response.created","response":{"id":"resp_timeout_ok","model":"gpt-5.1"}}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_timeout_ok","model":"gpt-5.1","usage":{"input_tokens":2,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_fallback","usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          81,
		Name:        "openai-read-timeout",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_timeout_ok", result.RequestID)
	require.Nil(t, upstream.lastReq, "每次 Read 都应独立应用超时；总时长超过 read_timeout 不应误回退 HTTP")
}

func TestOpenAIGatewayService_Forward_WSv2ResponseFailedNoFailoverReturnsMappedError(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.created","response":{"id":"resp_failed_no_failover","model":"gpt-5.1"}}`),
			[]byte(`{"type":"response.failed","response":{"id":"resp_failed_no_failover","status":"failed","error":{"code":"invalid_request_error","type":"invalid_request_error","message":"This request has been flagged for potentially high-risk cyber activity."}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_should_not_be_used","usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          82,
		Name:        "openai-failed-no-fallback",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "response.failed 非可回退场景不应走 HTTP fallback")
	require.Contains(t, err.Error(), "high-risk cyber")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "high-risk cyber")
}

func TestOpenAIGatewayService_Forward_WSv2ResponseFailedRetryableKeepsFallbackSignal(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.created","response":{"id":"resp_failed_retryable","model":"gpt-5.1"}}`),
			[]byte(`{"type":"response.failed","response":{"id":"resp_failed_retryable","status":"failed","error":{"code":"server_error","type":"server_error","message":"temporary upstream failure"}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"resp_http_fallback_retryable","usage":{"input_tokens":9,"output_tokens":3}}`,
			)),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	account := &Account{
		ID:          83,
		Name:        "openai-failed-fallback",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "应保留 response.failed 的 fallback signal 语义，由上层决定是否转 HTTP")
	var fallbackErr *openAIWSFallbackError
	require.ErrorAs(t, err, &fallbackErr)
	require.Equal(t, "response_failed", fallbackErr.Reason)
	require.Contains(t, fallbackErr.Error(), "temporary upstream failure")
}

type openAIWSCaptureDialer struct {
	mu          sync.Mutex
	conn        *openAIWSCaptureConn
	lastHeaders http.Header
	handshake   http.Header
	dialCount   int
}

func (d *openAIWSCaptureDialer) Dial(
	ctx context.Context,
	wsURL string,
	headers http.Header,
	proxyURL string,
	tlsProfile *tlsfingerprint.Profile,
) (openAIWSClientConn, int, http.Header, error) {
	_ = ctx
	_ = wsURL
	_ = proxyURL
	_ = tlsProfile
	d.mu.Lock()
	d.lastHeaders = cloneHeader(headers)
	d.dialCount++
	respHeaders := cloneHeader(d.handshake)
	d.mu.Unlock()
	return d.conn, 0, respHeaders, nil
}

func (d *openAIWSCaptureDialer) DialCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dialCount
}

type openAIWSSequentialCaptureDialer struct {
	mu          sync.Mutex
	conns       []*openAIWSCaptureConn
	lastHeaders http.Header
	dialCount   int
}

func (d *openAIWSSequentialCaptureDialer) Dial(
	ctx context.Context,
	wsURL string,
	headers http.Header,
	proxyURL string,
	tlsProfile *tlsfingerprint.Profile,
) (openAIWSClientConn, int, http.Header, error) {
	_ = ctx
	_ = wsURL
	_ = proxyURL
	_ = tlsProfile
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastHeaders = cloneHeader(headers)
	d.dialCount++
	idx := d.dialCount - 1
	if idx >= len(d.conns) {
		idx = len(d.conns) - 1
	}
	if idx < 0 {
		return nil, 0, nil, errors.New("no capture websocket connections configured")
	}
	return d.conns[idx], 0, nil, nil
}

func (d *openAIWSSequentialCaptureDialer) DialCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dialCount
}

type openAIWSCaptureConn struct {
	mu                   sync.Mutex
	readDelays           []time.Duration
	events               [][]byte
	lastWrite            map[string]any
	writes               []map[string]any
	closed               bool
	readDelayStarted     chan struct{}
	readDelayStartedOnce sync.Once
}

func (c *openAIWSCaptureConn) WriteJSON(ctx context.Context, value any) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errOpenAIWSConnClosed
	}
	switch payload := value.(type) {
	case map[string]any:
		c.lastWrite = cloneMapStringAny(payload)
		c.writes = append(c.writes, cloneMapStringAny(payload))
	case json.RawMessage:
		var parsed map[string]any
		if err := json.Unmarshal(payload, &parsed); err == nil {
			c.lastWrite = cloneMapStringAny(parsed)
			c.writes = append(c.writes, cloneMapStringAny(parsed))
		}
	case []byte:
		var parsed map[string]any
		if err := json.Unmarshal(payload, &parsed); err == nil {
			c.lastWrite = cloneMapStringAny(parsed)
			c.writes = append(c.writes, cloneMapStringAny(parsed))
		}
	}
	return nil
}

func (c *openAIWSCaptureConn) ReadMessage(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errOpenAIWSConnClosed
	}
	if len(c.events) == 0 {
		c.mu.Unlock()
		return nil, io.EOF
	}
	delay := time.Duration(0)
	if len(c.readDelays) > 0 {
		delay = c.readDelays[0]
		c.readDelays = c.readDelays[1:]
	}
	event := c.events[0]
	c.events = c.events[1:]
	c.mu.Unlock()
	if delay > 0 {
		if c.readDelayStarted != nil {
			c.readDelayStartedOnce.Do(func() { close(c.readDelayStarted) })
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return event, nil
}

func (c *openAIWSCaptureConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	payload, err := c.ReadMessage(ctx)
	if err != nil {
		return coderws.MessageText, nil, err
	}
	return coderws.MessageText, payload, nil
}

func (c *openAIWSCaptureConn) WriteFrame(ctx context.Context, _ coderws.MessageType, payload []byte) error {
	return c.WriteJSON(ctx, json.RawMessage(payload))
}

func (c *openAIWSCaptureConn) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

func (c *openAIWSCaptureConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func cloneMapStringAny(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
