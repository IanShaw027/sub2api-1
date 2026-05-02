package service

import (
	"context"
	"testing"

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
