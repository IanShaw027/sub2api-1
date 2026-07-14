package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TempUnschedState 临时不可调度状态
type TempUnschedState struct {
	UntilUnix       int64  `json:"until_unix"`        // 解除时间（Unix 时间戳）
	TriggeredAtUnix int64  `json:"triggered_at_unix"` // 触发时间（Unix 时间戳）
	StatusCode      int    `json:"status_code"`       // 触发的错误码
	MatchedKeyword  string `json:"matched_keyword"`   // 匹配的关键词
	RuleIndex       int    `json:"rule_index"`        // 触发的规则索引
	ErrorMessage    string `json:"error_message"`     // 错误消息
}

// TempUnschedCache 临时不可调度缓存接口
type TempUnschedCache interface {
	SetTempUnsched(ctx context.Context, accountID int64, state *TempUnschedState) error
	GetTempUnsched(ctx context.Context, accountID int64) (*TempUnschedState, error)
	DeleteTempUnsched(ctx context.Context, accountID int64) error
}

// TempUnschedCounterCache 临时不可调度规则的窗口计数缓存接口。
// 按 (accountID, ruleFingerprint) 维度独立计数，用于"窗口内连续命中 N 次才触发"的阈值判定。
type TempUnschedCounterCache interface {
	// IncrementTempUnschedCount 增加某账户某条规则的命中计数，返回当前计数值。
	// windowMinutes 是计数窗口（分钟），每次命中刷新过期时间，窗口滚动后自动重置。
	IncrementTempUnschedCount(ctx context.Context, accountID int64, ruleFingerprint string, windowMinutes int) (int64, error)
	// IncrementTempUnschedThreshold 原子地增加计数、判断阈值，并在达阈值时清零。
	IncrementTempUnschedThreshold(ctx context.Context, accountID int64, ruleFingerprint string, windowMinutes int, thresholdCount int) (int64, bool, error)
	// ResetTempUnschedCount 重置某账户某条规则的命中计数（触发后清零）。
	ResetTempUnschedCount(ctx context.Context, accountID int64, ruleFingerprint string) error
}

// TempUnschedCounterExactResetter 只清理当前 fingerprint 的计数 key，不扫描旧版规则 key。
// 高频成功路径应优先使用该接口，避免 ResetTempUnschedCount 的兼容性 SCAN。
type TempUnschedCounterExactResetter interface {
	ResetTempUnschedFingerprint(ctx context.Context, accountID int64, ruleFingerprint string) error
}

func tempUnschedRuleFingerprint(rule TempUnschedulableRule) string {
	keywords := make([]string, 0, len(rule.Keywords))
	for _, keyword := range rule.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	sort.Strings(keywords)
	return fmt.Sprintf("status=%d;keywords=%s", rule.ErrorCode, strings.Join(keywords, ","))
}

// TimeoutCounterCache 超时计数器缓存接口
type TimeoutCounterCache interface {
	// IncrementTimeoutCount 增加账户的超时计数，返回当前计数值
	// windowMinutes 是计数窗口时间（分钟），超过此时间计数器会自动重置
	IncrementTimeoutCount(ctx context.Context, accountID int64, windowMinutes int) (int64, error)
	// GetTimeoutCount 获取账户当前的超时计数
	GetTimeoutCount(ctx context.Context, accountID int64) (int64, error)
	// ResetTimeoutCount 重置账户的超时计数
	ResetTimeoutCount(ctx context.Context, accountID int64) error
	// GetTimeoutCountTTL 获取计数器剩余过期时间
	GetTimeoutCountTTL(ctx context.Context, accountID int64) (time.Duration, error)
}
