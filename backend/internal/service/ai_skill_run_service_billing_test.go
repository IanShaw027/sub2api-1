//go:build unit

package service

import (
	"context"
	"testing"

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
		return &APIKey{ID: id, UserID: s.ownerUserID, User: &User{ID: s.ownerUserID}}, nil
	}
	cloned := *s.key
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
				Type: AISkillTypePromptChat,
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
				Type: AISkillTypePromptChat,
				PromptChat: &AISkillPromptChatSpec{UserPromptTemplate: "hi"},
			},
			BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		},
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "AI_SKILL_API_KEY_REQUIRED")
}

func TestOpenAIWSSessionPreemptCacheHashIncludesAPIKey(t *testing.T) {
	t.Parallel()
	a := openAIWSSessionPreemptCacheHash(11, "sess")
	b := openAIWSSessionPreemptCacheHash(12, "sess")
	require.NotEqual(t, a, b)
	require.Contains(t, a, "11:")
	require.Contains(t, b, "12:")
}
