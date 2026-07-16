//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type skillBillingAPIKeyRepoStub struct {
	key *APIKey
	err error
	// ownerUserID, when > 0, forces returned key ownership (for multi-user tests).
	ownerUserID int64
}

func (s *skillBillingAPIKeyRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	if s == nil {
		return nil, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.key == nil {
		if s.ownerUserID <= 0 {
			return nil, nil
		}
		return &APIKey{ID: id, UserID: s.ownerUserID, Status: StatusActive, User: &User{ID: s.ownerUserID}}, nil
	}
	cloned := *s.key
	if cloned.Status == "" {
		cloned.Status = StatusActive
	}
	if id > 0 {
		cloned.ID = id
	}
	if s.ownerUserID > 0 {
		cloned.UserID = s.ownerUserID
		cloned.User = &User{ID: s.ownerUserID}
	}
	out := cloned
	return &out, nil
}

func TestResolveBillingAPIKey_RejectsForeignKey(t *testing.T) {
	t.Parallel()
	keyID := int64(99)
	svc := &AISkillRunService{apiKeyRepo: &skillBillingAPIKeyRepoStub{
		key: &APIKey{ID: keyID, UserID: 2, User: &User{ID: 2}},
	}}
	_, err := svc.resolveBillingAPIKey(context.Background(), 1, AIWriteTrace{APIKeyID: &keyID})
	require.Error(t, err)
}

func TestResolveBillingAPIKey_AcceptsOwnedKey(t *testing.T) {
	t.Parallel()
	keyID := int64(99)
	groupID := int64(5)
	svc := &AISkillRunService{apiKeyRepo: &skillBillingAPIKeyRepoStub{
		key: &APIKey{ID: keyID, UserID: 1, GroupID: &groupID, User: &User{ID: 1}},
	}}
	key, err := svc.resolveBillingAPIKey(context.Background(), 1, AIWriteTrace{APIKeyID: &keyID})
	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, keyID, key.ID)
}

func TestResolveBillingAPIKey_RejectsUnusableKeys(t *testing.T) {
	t.Parallel()
	keyID := int64(99)
	now := time.Now()
	tests := []struct {
		name string
		key  *APIKey
	}{
		{name: "disabled", key: &APIKey{Status: StatusDisabled}},
		{name: "expired status", key: &APIKey{Status: StatusAPIKeyExpired}},
		{name: "expired timestamp", key: &APIKey{Status: StatusActive, ExpiresAt: pointerToTime(now.Add(-time.Minute))}},
		{name: "quota exhausted", key: &APIKey{Status: StatusActive, Quota: 1, QuotaUsed: 1}},
		{name: "5h rate exhausted", key: &APIKey{Status: StatusActive, RateLimit5h: 1, Usage5h: 1, Window5hStart: pointerToTime(now)}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.key.ID = keyID
			tt.key.UserID = 1
			tt.key.User = &User{ID: 1}
			svc := &AISkillRunService{apiKeyRepo: &skillBillingAPIKeyRepoStub{key: tt.key}}
			_, err := svc.resolveBillingAPIKey(context.Background(), 1, AIWriteTrace{APIKeyID: &keyID})
			require.Error(t, err)
		})
	}
}

func pointerToTime(value time.Time) *time.Time {
	return &value
}

func TestResolveAISkillGroupID_PrefersBillingAPIKeyGroup(t *testing.T) {
	t.Parallel()
	keyGroup := int64(10)
	skillGroup := int64(20)
	got := resolveAISkillGroupID(AISkillExecutionRequest{
		BillingAPIKey: &APIKey{GroupID: &keyGroup},
		Skill:         &AISkill{Trace: AITraceRef{GroupID: &skillGroup}},
	})
	require.NotNil(t, got)
	require.Equal(t, keyGroup, *got)
}

func TestBuildExecutionRequest_UseModeRejectsForeignAPIKey(t *testing.T) {
	t.Parallel()
	keyID := int64(99)
	svc := &AISkillRunService{apiKeyRepo: &skillBillingAPIKeyRepoStub{
		key: &APIKey{ID: keyID, UserID: 2, User: &User{ID: 2}},
	}}
	_, err := svc.buildExecutionRequest(context.Background(),
		&AISkillRun{ID: 1, UserID: 1, Mode: AISkillRunModeUse, Trace: AIWriteTrace{APIKeyID: &keyID}},
		&AISkill{ID: 1, Type: AISkillTypePromptChat},
		&AISkillVersion{
			ID: 1, SkillID: 1, Type: AISkillTypePromptChat, Status: AISkillVersionStatusApproved,
			ExecutionSpec: AISkillExecutionSpec{
				Type:       AISkillTypePromptChat,
				PromptChat: &AISkillPromptChatSpec{UserPromptTemplate: "hi {{x}}"},
			},
			BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		},
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "AI_SKILL_API_KEY_FORBIDDEN")
}

func TestBuildExecutionRequest_UseModeRequiresResolvedBillingKey(t *testing.T) {
	t.Parallel()
	svc := &AISkillRunService{}
	_, err := svc.buildExecutionRequest(context.Background(),
		&AISkillRun{ID: 1, UserID: 1, Mode: AISkillRunModeUse, Trace: AIWriteTrace{}},
		&AISkill{ID: 1, Type: AISkillTypePromptChat},
		&AISkillVersion{
			ID: 1, SkillID: 1, Type: AISkillTypePromptChat, Status: AISkillVersionStatusApproved,
			ExecutionSpec: AISkillExecutionSpec{
				Type:       AISkillTypePromptChat,
				PromptChat: &AISkillPromptChatSpec{UserPromptTemplate: "hi"},
			},
			BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		},
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "AI_SKILL_API_KEY_REQUIRED")
}

func TestBuildExecutionRequest_TestModePromptRequiresBillingKey(t *testing.T) {
	t.Parallel()
	svc := &AISkillRunService{}
	_, err := svc.buildExecutionRequest(context.Background(),
		&AISkillRun{ID: 1, UserID: 1, Mode: AISkillRunModeTest, Trace: AIWriteTrace{}},
		&AISkill{ID: 1, Type: AISkillTypePromptChat},
		&AISkillVersion{
			ID: 1, SkillID: 1, Type: AISkillTypePromptChat, Status: AISkillVersionStatusApproved,
			ExecutionSpec: AISkillExecutionSpec{
				Type:       AISkillTypePromptChat,
				PromptChat: &AISkillPromptChatSpec{UserPromptTemplate: "hi"},
			},
			BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		},
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "AI_SKILL_API_KEY_REQUIRED")
}

func TestApplyAISkillChatRuntimeParameters_DropsToolsControlPlane(t *testing.T) {
	t.Parallel()
	body := map[string]any{"model": "gpt-4o", "messages": []any{}}
	applyAISkillChatRuntimeParameters(body, map[string]any{
		"temperature":   0.2,
		"tools":         []any{map[string]any{"type": "function"}},
		"tool_choice":   "auto",
		"functions":     []any{map[string]any{"name": "x"}},
		"function_call": "auto",
		"max_tokens":    999999,
	})
	require.Equal(t, 0.2, body["temperature"])
	require.Equal(t, aiSkillChatMaxTokensHardCap, body["max_tokens"])
	require.NotContains(t, body, "tools")
	require.NotContains(t, body, "tool_choice")
	require.NotContains(t, body, "functions")
	require.NotContains(t, body, "function_call")
}

func TestOpenAIWSSessionPreemptCacheHashIncludesAPIKey(t *testing.T) {
	t.Parallel()
	a := openAIWSSessionPreemptCacheHash(11, "sess")
	b := openAIWSSessionPreemptCacheHash(12, "sess")
	require.NotEqual(t, a, b)
	require.Contains(t, a, "11:")
	require.Contains(t, b, "12:")
}
