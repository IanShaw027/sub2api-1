//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreationMessageRepositoryCreateTouchesSession(t *testing.T) {
	_, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "creation-message-touch@test.com")
	group, err := client.Group.Create().
		SetName("creation-message-group").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		Save(ctx)
	require.NoError(t, err)

	oldUpdatedAt := time.Now().Add(-time.Hour).UTC()
	session, err := client.CreationSession.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetTitle("Chat").
		SetModel("gpt-4o").
		SetMode(service.CreationSessionModeChat).
		SetStatus(service.CreationSessionStatusActive).
		SetUpdatedAt(oldUpdatedAt).
		Save(ctx)
	require.NoError(t, err)

	repo := &creationMessageRepository{client: client}
	message := &service.CreationMessage{
		SessionID: session.ID,
		Role:      service.CreationMessageRoleUser,
		Content:   json.RawMessage(`"hello"`),
	}
	require.NoError(t, repo.Create(ctx, message))
	require.NotZero(t, message.ID)

	updated, err := client.CreationSession.Get(ctx, session.ID)
	require.NoError(t, err)
	require.True(t, updated.UpdatedAt.After(oldUpdatedAt))

	missingSessionMessage := &service.CreationMessage{
		SessionID: session.ID + 999,
		Role:      service.CreationMessageRoleUser,
		Content:   json.RawMessage(`"not persisted"`),
	}
	require.Error(t, repo.Create(ctx, missingSessionMessage))
	count, err := client.CreationMessage.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
