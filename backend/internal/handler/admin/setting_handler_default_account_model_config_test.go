//go:build unit

package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestDefaultAccountModelConfigConverter_RoundTrip 验证 service<->dto 转换保留全部字段，
// 包括历史上漏拷的 kiro 嵌套配置以及新增的临时不可调度 / 自定义错误码字段。
func TestDefaultAccountModelConfigConverter_RoundTrip(t *testing.T) {
	src := map[string]service.DefaultAccountModelConfig{
		"openai": {
			ModelWhitelist:           []string{"gpt-5.4", "gpt-*"},
			ModelMapping:             map[string]string{"gpt-5.2": "gpt-5.4"},
			CompactModelMapping:      map[string]string{"gpt-5.2-mini": "gpt-5.4-mini"},
			TempUnschedulableEnabled: true,
			TempUnschedulableRules: []service.TempUnschedulableRule{
				{ErrorCode: 502, Keywords: []string{"Upstream request failed"}, DurationMinutes: 10, Description: "d"},
				{ErrorCode: 524, DurationMinutes: 10},
			},
			CustomErrorCodesEnabled: true,
			CustomErrorCodes:        []int{500, 502, 503},
		},
		"kiro": {
			ModelMapping: map[string]string{"claude-sonnet-*": "claude-sonnet-4.6"},
			KiroSubscriptionTypeModelMap: map[string]service.DefaultAccountModelConfig{
				"pro": {
					ModelMapping:        map[string]string{"claude-opus-*": "claude-opus-4.7"},
					CompactModelMapping: map[string]string{"claude-opus-4.6": "claude-opus-4.7"},
				},
			},
		},
	}

	roundTripped := fromDTODefaultAccountModelConfig(toDTODefaultAccountModelConfig(src))

	require.True(t, defaultAccountModelConfigEqual(src, roundTripped),
		"round-trip must preserve all fields including kiro nesting and temp-unsched/custom-error-code")

	// 显式断言关键新字段
	openai := roundTripped["openai"]
	require.True(t, openai.TempUnschedulableEnabled)
	require.Len(t, openai.TempUnschedulableRules, 2)
	require.Equal(t, 524, openai.TempUnschedulableRules[1].ErrorCode)
	require.Empty(t, openai.TempUnschedulableRules[1].Keywords)
	require.True(t, openai.CustomErrorCodesEnabled)
	require.ElementsMatch(t, []int{500, 502, 503}, openai.CustomErrorCodes)

	// kiro 嵌套不丢失
	require.Contains(t, roundTripped["kiro"].KiroSubscriptionTypeModelMap, "pro")
	require.Equal(t, map[string]string{"claude-opus-*": "claude-opus-4.7"},
		roundTripped["kiro"].KiroSubscriptionTypeModelMap["pro"].ModelMapping)
}
