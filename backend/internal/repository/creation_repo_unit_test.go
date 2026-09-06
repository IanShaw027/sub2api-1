//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
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

func TestCreationImageRepositoryPersistsStorageAndRejectsStaleProgress(t *testing.T) {
	_, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "creation-image@test.com")
	repo := NewCreationImageJobRepository(client)
	taskID := "imgtask_persisted"
	job := &service.CreationImageJob{
		UserID: user.ID, GroupID: 3, Model: "gpt-image-1",
		Status: service.CreationImageJobStatusProcessing, ProviderTaskID: &taskID,
	}
	require.NoError(t, repo.Create(ctx, job))
	stale := *job
	job.Status = service.CreationImageJobStatusCompleted
	job.MediaURL = "https://cdn.test/images/result.png?signature=old"
	job.StorageID, job.StorageKey = "bucket-a", "images/result.png"
	require.NoError(t, repo.Update(ctx, job.ID, job))
	require.NoError(t, repo.Update(ctx, job.ID, &stale))
	stale.Status = service.CreationImageJobStatusFailed
	require.NoError(t, repo.Update(ctx, job.ID, &stale))
	got, err := repo.GetByProviderTaskID(ctx, user.ID, taskID)
	require.NoError(t, err)
	require.Equal(t, job.Status, got.Status)
	require.Equal(t, job.StorageID, got.StorageID)
	require.Equal(t, job.StorageKey, got.StorageKey)
	require.Equal(t, job.MediaURL, got.MediaURL)
	_, err = repo.GetForUser(ctx, user.ID+1, job.ID)
	require.ErrorIs(t, err, service.ErrCreationImageNotFound)
	failed := &service.CreationImageJob{UserID: user.ID, GroupID: 3, Status: service.CreationImageJobStatusFailed}
	require.NoError(t, repo.Create(ctx, failed))
	lateCompletion := *failed
	lateCompletion.Status = service.CreationImageJobStatusCompleted
	require.NoError(t, repo.Update(ctx, failed.ID, &lateCompletion))
	persisted, err := repo.GetByID(ctx, failed.ID)
	require.NoError(t, err)
	require.Equal(t, service.CreationImageJobStatusFailed, persisted.Status)
}

func creationExchangeFixture(t *testing.T) (*dbent.Client, service.CreationMessageRepository, service.CreateCreationExchangeInput) {
	t.Helper()
	_, client := newAPIKeyRepoSQLite(t)
	session, err := client.CreationSession.Create().SetUserID(7).SetGroupID(3).SetUpdatedAt(time.Now().Add(-time.Hour)).Save(context.Background())
	require.NoError(t, err)
	input, output := 42, 17
	return client, NewCreationMessageRepository(client), service.CreateCreationExchangeInput{
		UserID: 7, SessionID: session.ID, RequestID: "exchange-1", UserContent: "<hello>&", AssistantContent: "answer",
		Model: "gpt-4o", InputTokens: &input, OutputTokens: &output,
	}
}

func TestCreationExchangeRepositoryAtomicAndIdempotent(t *testing.T) {
	client, repo, input := creationExchangeFixture(t)
	ctx := context.Background()
	before, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	first, err := repo.CreateExchange(ctx, input)
	require.NoError(t, err)
	require.Less(t, first.User.ID, first.Assistant.ID)
	require.Equal(t, input.InputTokens, first.Assistant.InputTokens)
	require.Equal(t, input.OutputTokens, first.Assistant.OutputTokens)
	require.Nil(t, first.User.InputTokens)
	require.Equal(t, input.RequestID, *first.User.ExchangeRequestID)
	require.Equal(t, input.RequestID, *first.Assistant.ExchangeRequestID)
	history, err := repo.ListBySession(ctx, input.SessionID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, input.RequestID, *history[0].ExchangeRequestID)
	require.Equal(t, input.RequestID, *history[1].ExchangeRequestID)
	touched, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, touched.UpdatedAt.After(before.UpdatedAt))
	second, err := repo.CreateExchange(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.User.ID, second.User.ID)
	require.Equal(t, first.Assistant.ID, second.Assistant.ID)
	afterReplay, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, touched.UpdatedAt.Equal(afterReplay.UpdatedAt))
	count, err := client.CreationMessage.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	for _, mutate := range []func(*service.CreateCreationExchangeInput){
		func(v *service.CreateCreationExchangeInput) { v.UserContent = "different" },
		func(v *service.CreateCreationExchangeInput) { v.AssistantContent = "different" },
		func(v *service.CreateCreationExchangeInput) { v.Model = "different" },
		func(v *service.CreateCreationExchangeInput) { v.InputTokens = nil },
		func(v *service.CreateCreationExchangeInput) { value := 99; v.OutputTokens = &value },
	} {
		changed := input
		mutate(&changed)
		_, err := repo.CreateExchange(ctx, changed)
		require.ErrorIs(t, err, service.ErrCreationExchangeConflict)
	}
	foreign := input
	foreign.UserID++
	_, err = repo.CreateExchange(ctx, foreign)
	require.ErrorIs(t, err, service.ErrCreationSessionNotFound)
	_, err = client.CreationMessage.Delete().Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, client.CreationSession.DeleteOneID(input.SessionID).Exec(ctx))
	_, err = repo.CreateExchange(ctx, input)
	require.ErrorIs(t, err, service.ErrCreationSessionNotFound)
}

func TestCreationExchangeRepositoryRollsBackPartialInsert(t *testing.T) {
	client, repo, input := creationExchangeFixture(t)
	ctx := context.Background()
	before, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	client.CreationMessage.Use(func(next dbent.Mutator) dbent.Mutator {
		return dbent.MutateFunc(func(ctx context.Context, m dbent.Mutation) (dbent.Value, error) {
			if mutation, ok := m.(*dbent.CreationMessageMutation); ok {
				if role, _ := mutation.Role(); role == service.CreationMessageRoleAssistant {
					return nil, errors.New("assistant insert failed")
				}
			}
			return next.Mutate(ctx, m)
		})
	})
	_, err = repo.CreateExchange(ctx, input)
	require.ErrorContains(t, err, "assistant insert failed")
	count, err := client.CreationMessage.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	after, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, before.UpdatedAt.Equal(after.UpdatedAt))
}

func TestCreationExchangeContentMatchesJSONBNormalization(t *testing.T) {
	require.True(t, creationContentMatches(json.RawMessage(`"<hello>&"`), "<hello>&"))
	require.True(t, creationContentMatches(json.RawMessage(`"\u003chello\u003e\u0026"`), "<hello>&"))
}
