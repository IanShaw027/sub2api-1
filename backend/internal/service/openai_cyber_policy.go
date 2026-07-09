package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// opsCyberPolicyKey 在 gin context 中携带 cyber_policy 命中标记。
// 由 gateway 服务层在检测到上游 error.code=="cyber_policy" 时设置，
// handler 在 Forward 返回后读取以触发风控记录、邮件与 tokens=0 用量行。
const opsCyberPolicyKey = "ops_cyber_policy"

// errOpenAICyberPolicyForwarded 表示 cyber_policy 已按当前端点格式透传给客户端
// （error 已写出/下发）。compat 路径 ForwardAsChatCompletions / ForwardAsAnthropic 出口
// 据此丢弃 result 并返回该哨兵，使 handler 落入 tokens=0 免费用量行（对齐 /v1/responses），
// 既不计费、也不 failover、不重复写响应。
var errOpenAICyberPolicyForwarded = errors.New("openai cyber_policy forwarded to client")

// CyberPolicyMark 记录一次 cyber_policy 硬阻断的上游证据。
type CyberPolicyMark struct {
	Code           string // 固定 "cyber_policy"
	Message        string // 上游 error.message
	Body           string // 上游 response.failed / 400 原始 body（已截断；未脱敏，ops_error 落库由 sanitizeErrorBodyForStorage、风控日志由 redactContentModerationSecrets 统一脱敏）
	UpstreamStatus int    // 上游 HTTP 状态（流式=200，非流式=400）
	UpstreamInTok  int    // 上游已报 input tokens（如有）
	UpstreamOutTok int    // 上游已报 output tokens（如有）
}

// MarkOpsCyberPolicy 记录 cyber 标记；首个写入生效，后续只允许补齐首次未捕获的
// upstream token/status 字段（同一 turn 只记一次 message/body）。
// WS 多轮场景由 handler 在每个 turn 结束后调用 ClearOpsCyberPolicy 重置。
func MarkOpsCyberPolicy(c *gin.Context, mark CyberPolicyMark) {
	if c == nil {
		return
	}
	if existing := GetOpsCyberPolicy(c); existing != nil {
		updated := *existing
		if updated.UpstreamStatus == 0 && mark.UpstreamStatus != 0 {
			updated.UpstreamStatus = mark.UpstreamStatus
		}
		if updated.UpstreamInTok == 0 && mark.UpstreamInTok != 0 {
			updated.UpstreamInTok = mark.UpstreamInTok
		}
		if updated.UpstreamOutTok == 0 && mark.UpstreamOutTok != 0 {
			updated.UpstreamOutTok = mark.UpstreamOutTok
		}
		c.Set(opsCyberPolicyKey, &updated)
		return
	}
	mark.Code = "cyber_policy"
	mark.Message = strings.TrimSpace(mark.Message)
	mark.Body = strings.TrimSpace(mark.Body)
	c.Set(opsCyberPolicyKey, &mark)
}

// GetOpsCyberPolicy 返回 cyber 标记，未命中（或已被 Clear）返回 nil。
func GetOpsCyberPolicy(c *gin.Context) *CyberPolicyMark {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(opsCyberPolicyKey); ok {
		if m, ok := v.(*CyberPolicyMark); ok && m != nil {
			return m
		}
	}
	return nil
}

// ClearOpsCyberPolicy 清除 cyber 标记（typed-nil 覆盖；gin context 无并发安全的
// 删除原语，Set 走内部锁，与异步 GetOpsCyberPolicy 不构成 data race）。
// 仅 WS 多轮路径在 turn 收尾调用；HTTP 单请求路径不调用（context 随请求销毁，
// 且中间件 shouldSkipOpsErrorLogForCyber 依赖标记防双写）。
// WS 路径 clear 发生在中间件收尾之前，连接响应状态为 101，不触发中间件 status>=400
// 落库分支，故无双写/漏写。
func ClearOpsCyberPolicy(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(opsCyberPolicyKey, (*CyberPolicyMark)(nil))
}

// detectOpenAICyberPolicy 精确识别 cyber_policy（对齐 codex api_bridge.rs:145 /
// sse/responses.rs:529），并支持从 SSE 流式帧（data:/event:）递归解析。
// 命中返回 (true, code, message)。
func detectOpenAICyberPolicy(payload []byte) (matched bool, code, msg string) {
	if len(payload) == 0 {
		return false, "", ""
	}

	candidates := []struct {
		codePath string
		msgPath  string
	}{
		{codePath: "error.code", msgPath: "error.message"},
		{codePath: "response.error.code", msgPath: "response.error.message"},
	}

	for _, candidate := range candidates {
		codeValue := strings.TrimSpace(gjson.GetBytes(payload, candidate.codePath).String())
		if !strings.EqualFold(codeValue, "cyber_policy") {
			continue
		}
		return true, codeValue, strings.TrimSpace(gjson.GetBytes(payload, candidate.msgPath).String())
	}

	if bytes.Contains(payload, []byte("data:")) || bytes.Contains(payload, []byte("event:")) {
		for _, frame := range openAICompatSSEFramesFromBody(string(payload)) {
			data := strings.TrimSpace(openAICompatPayloadWithEventType(frame.Data, frame.EventType))
			if data == "" || data == "[DONE]" {
				continue
			}
			if matched, code, msg := detectOpenAICyberPolicy([]byte(data)); matched {
				return true, code, msg
			}
		}
	}

	return false, "", ""
}

func markOpsCyberPolicyIfDetected(c *gin.Context, payload []byte, upstreamStatus int, upstreamInTok int, upstreamOutTok int) bool {
	matched, code, msg := detectOpenAICyberPolicy(payload)
	if !matched {
		return false
	}
	MarkOpsCyberPolicy(c, CyberPolicyMark{
		Code:           code,
		Message:        msg,
		Body:           truncateString(string(payload), 4096),
		UpstreamStatus: upstreamStatus,
		UpstreamInTok:  upstreamInTok,
		UpstreamOutTok: upstreamOutTok,
	})
	return true
}

func markOpsCyberPolicyIfDetectedWithUsage(c *gin.Context, payload []byte, upstreamStatus int) bool {
	usage, _ := extractOpenAIUsageFromJSONBytes(payload)
	return markOpsCyberPolicyIfDetected(c, payload, upstreamStatus, usage.InputTokens, usage.OutputTokens)
}

func markOpenAIWSPassthroughCyberPolicy(c *gin.Context, payload []byte) bool {
	return markOpsCyberPolicyIfDetectedWithUsage(c, payload, http.StatusOK)
}

// HandleOpenAICyberPolicy 仅保留 cyber_policy 识别入口的兼容壳。
// cyber_policy 不再修改账号状态；请求级记录和后续本地拦截由风控 hash 负责。
func (s *RateLimitService) HandleOpenAICyberPolicy(ctx context.Context, account *Account, responseBody []byte) bool {
	return false
}
