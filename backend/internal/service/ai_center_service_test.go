package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type aiRuntimeKeyServiceStub struct {
	groups []Group
	keys   []APIKey
}

func (s *aiRuntimeKeyServiceStub) GetAvailableRouteGroups(context.Context, int64) ([]Group, error) {
	return append([]Group(nil), s.groups...), nil
}

func (s *aiRuntimeKeyServiceStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	return append([]APIKey(nil), s.keys...), nil
}

func (s *aiRuntimeKeyServiceStub) CheckAPIKeyQuotaAndExpiry(apiKey *APIKey) error {
	if apiKey == nil {
		return ErrAPIKeyNotFound
	}
	return nil
}

func TestAICenterServiceGetRuntimeIncludesKeyNames(t *testing.T) {
	svc := &AICenterService{}
	groupID := int64(12)
	stub := &aiRuntimeKeyServiceStub{
		groups: []Group{
			{
				ID:             groupID,
				Name:           "codex",
				DisplayName:    "Codex",
				Platform:       PlatformOpenAI,
				Status:         StatusActive,
				UserSelectable: true,
				Hydrated:       true,
			},
		},
		keys: []APIKey{
			{ID: 102, Name: "Backup", GroupID: &groupID, Status: StatusActive},
			{ID: 101, Name: "Primary", GroupID: &groupID, Status: StatusActive},
		},
	}

	runtime, err := svc.GetRuntime(context.Background(), 1, stub)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.Len(t, runtime.Lines, 1)
	require.Equal(t, []AIRuntimeKey{
		{ID: 102, Name: "Backup"},
		{ID: 101, Name: "Primary"},
	}, runtime.Lines[0].Keys)
	require.Equal(t, []int64{102, 101}, runtime.Lines[0].KeyIDs)
	require.Equal(t, int64(102), *runtime.Lines[0].DefaultKeyID)
}

func TestAICenterService_NilRepoPublicMethodsReturnUnavailable(t *testing.T) {
	svc := NewAICenterService(nil, nil)
	ctx := context.Background()

	t.Run("CreateSession", func(t *testing.T) {
		session, err := svc.CreateSession(ctx, 1, &AICreateSessionInput{Title: "x"})
		require.Error(t, err)
		require.Nil(t, session)
		require.Contains(t, err.Error(), "AI_CENTER_REPO_UNAVAILABLE")
	})

	t.Run("GetSession", func(t *testing.T) {
		session, err := svc.GetSession(ctx, 1, 2)
		require.Error(t, err)
		require.Nil(t, session)
		require.Contains(t, err.Error(), "AI_CENTER_REPO_UNAVAILABLE")
	})

	t.Run("ListSessions", func(t *testing.T) {
		items, page, err := svc.ListSessions(ctx, 1, pagination.PaginationParams{}, "")
		require.Error(t, err)
		require.Nil(t, items)
		require.Nil(t, page)
		require.Contains(t, err.Error(), "AI_CENTER_REPO_UNAVAILABLE")
	})

	t.Run("CreatePromptTemplate", func(t *testing.T) {
		template, err := svc.CreatePromptTemplate(ctx, 1, &AICreatePromptTemplateInput{
			Title:   "prompt",
			Content: "hello",
		})
		require.Error(t, err)
		require.Nil(t, template)
		require.Contains(t, err.Error(), "AI_CENTER_REPO_UNAVAILABLE")
	})

	t.Run("CreateAssets", func(t *testing.T) {
		err := svc.CreateAssets(ctx, []*AIAsset{{ID: 1}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "AI_CENTER_REPO_UNAVAILABLE")
	})

	t.Run("AdminListAuditLogs", func(t *testing.T) {
		items, page, err := svc.AdminListAuditLogs(ctx, pagination.PaginationParams{}, AIListAuditLogsFilter{})
		require.Error(t, err)
		require.Nil(t, items)
		require.Nil(t, page)
		require.Contains(t, err.Error(), "AI_CENTER_REPO_UNAVAILABLE")
	})
}
