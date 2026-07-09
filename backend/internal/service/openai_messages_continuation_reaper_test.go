package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestReapExpiredOpenAICompatSessions 后台清扫删除已过期与类型异常的绑定，保留未过期项，堵住无界滞留。
func TestReapExpiredOpenAICompatSessions(t *testing.T) {
	s := &OpenAIGatewayService{}
	now := time.Now()

	s.openaiCompatSessionResponses.Store("expired", openAICompatSessionResponseBinding{
		ResponseID: "r1",
		ExpiresAt:  now.Add(-time.Minute),
	})
	s.openaiCompatSessionResponses.Store("alive", openAICompatSessionResponseBinding{
		ResponseID: "r2",
		ExpiresAt:  now.Add(time.Hour),
	})
	s.openaiCompatSessionResponses.Store("badtype", "not-a-binding")

	s.reapExpiredOpenAICompatSessions(now)

	_, expiredKept := s.openaiCompatSessionResponses.Load("expired")
	require.False(t, expiredKept, "过期绑定应被删除")
	_, badKept := s.openaiCompatSessionResponses.Load("badtype")
	require.False(t, badKept, "类型异常项应被删除")
	alive, aliveKept := s.openaiCompatSessionResponses.Load("alive")
	require.True(t, aliveKept, "未过期绑定应保留")
	aliveBinding, ok := alive.(openAICompatSessionResponseBinding)
	require.True(t, ok)
	require.Equal(t, "r2", aliveBinding.ResponseID)
}
