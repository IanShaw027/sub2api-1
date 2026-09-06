//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/creationmessage"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newCreationExchangePGFixture(t *testing.T) (*dbent.Client, service.CreationMessageRepository, service.CreateCreationExchangeInput) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	suffix := time.Now().UnixNano()
	user := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("creation-exchange-%d@example.com", suffix), Concurrency: 1,
	})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name: fmt.Sprintf("creation-exchange-%d", suffix), RateMultiplier: 1,
	})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
		require.NoError(t, err)
	})
	session, err := client.CreationSession.Create().
		SetUserID(user.ID).SetGroupID(group.ID).
		SetUpdatedAt(time.Now().Add(-time.Hour)).Save(ctx)
	require.NoError(t, err)
	inputTokens, outputTokens := 42, 17
	return client, NewCreationMessageRepository(client), service.CreateCreationExchangeInput{
		UserID: user.ID, SessionID: session.ID, RequestID: "exchange-1",
		UserContent: "<hello>&", AssistantContent: "answer", Model: "gpt-4o",
		InputTokens: &inputTokens, OutputTokens: &outputTokens,
	}
}

func countCreationExchangePGMessages(t *testing.T, client *dbent.Client, sessionID int64) int {
	t.Helper()
	count, err := client.CreationMessage.Query().Where(creationmessage.SessionIDEQ(sessionID)).Count(context.Background())
	require.NoError(t, err)
	return count
}

func TestCreationExchangePostgres_ConcurrentIdempotency(t *testing.T) {
	client, repo, input := newCreationExchangePGFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	before, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	type result struct {
		exchange *service.CreationExchange
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var ready, workers sync.WaitGroup
	ready.Add(2)
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			ready.Done()
			<-start
			exchange, err := repo.CreateExchange(ctx, input)
			results <- result{exchange: exchange, err: err}
		}()
	}
	ready.Wait()
	close(start)
	workers.Wait()
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotNil(t, first.exchange)
	require.NotNil(t, second.exchange)
	require.Equal(t, first.exchange.User.ID, second.exchange.User.ID)
	require.Equal(t, first.exchange.Assistant.ID, second.exchange.Assistant.ID)
	require.Less(t, first.exchange.User.ID, first.exchange.Assistant.ID)
	require.Equal(t, input.InputTokens, first.exchange.Assistant.InputTokens)
	require.Equal(t, input.OutputTokens, first.exchange.Assistant.OutputTokens)
	require.Equal(t, 2, countCreationExchangePGMessages(t, client, input.SessionID))
	committed, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, committed.UpdatedAt.After(before.UpdatedAt))
	replayed, err := repo.CreateExchange(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.exchange.User.ID, replayed.User.ID)
	require.Equal(t, first.exchange.Assistant.ID, replayed.Assistant.ID)
	afterReplay, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, committed.UpdatedAt.Equal(afterReplay.UpdatedAt), "replays must not reorder sessions")
}

func TestCreationExchangePostgres_RequestPayloadConflict(t *testing.T) {
	client, repo, input := newCreationExchangePGFixture(t)
	ctx := context.Background()
	first, err := repo.CreateExchange(ctx, input)
	require.NoError(t, err)
	before, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func(*service.CreateCreationExchangeInput)
	}{
		{"user content", func(v *service.CreateCreationExchangeInput) { v.UserContent = "changed question" }},
		{"assistant content", func(v *service.CreateCreationExchangeInput) { v.AssistantContent = "changed answer" }},
		{"model", func(v *service.CreateCreationExchangeInput) { v.Model = "different-model" }},
		{"input tokens", func(v *service.CreateCreationExchangeInput) { v.InputTokens = nil }},
		{"output tokens", func(v *service.CreateCreationExchangeInput) { n := 99; v.OutputTokens = &n }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := input
			tc.mutate(&changed)
			_, err := repo.CreateExchange(ctx, changed)
			require.ErrorIs(t, err, service.ErrCreationExchangeConflict)
			require.Equal(t, 2, countCreationExchangePGMessages(t, client, input.SessionID))
		})
	}
	after, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, before.UpdatedAt.Equal(after.UpdatedAt))
	replayed, err := repo.CreateExchange(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.User.ID, replayed.User.ID)
	require.Equal(t, first.Assistant.ID, replayed.Assistant.ID)
}

func TestCreationExchangePostgres_AssistantFailureRollsBack(t *testing.T) {
	client, repo, input := newCreationExchangePGFixture(t)
	ctx := context.Background()
	before, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	// Keep hooks private; closing this client would close the harness's shared DB.
	isolated := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, integrationDB)))
	insertFailure := errors.New("injected assistant insert failure")
	userInserted := false
	isolated.CreationMessage.Use(func(next dbent.Mutator) dbent.Mutator {
		return dbent.MutateFunc(func(ctx context.Context, mutation dbent.Mutation) (dbent.Value, error) {
			message, ok := mutation.(*dbent.CreationMessageMutation)
			if !ok {
				return next.Mutate(ctx, mutation)
			}
			role, _ := message.Role()
			if role == service.CreationMessageRoleAssistant {
				return nil, insertFailure
			}
			value, err := next.Mutate(ctx, mutation)
			if role == service.CreationMessageRoleUser && err == nil {
				userInserted = true
			}
			return value, err
		})
	})
	_, err = NewCreationMessageRepository(isolated).CreateExchange(ctx, input)
	require.ErrorIs(t, err, insertFailure)
	require.True(t, userInserted, "failure must occur after the user row reached PostgreSQL")
	require.Zero(t, countCreationExchangePGMessages(t, client, input.SessionID))
	after, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "failed exchanges must roll back the session touch")
	_, err = repo.CreateExchange(ctx, input)
	require.NoError(t, err, "retry must not inherit the failed transaction or isolated hook")
	require.Equal(t, 2, countCreationExchangePGMessages(t, client, input.SessionID))
}

func TestCreationExchangePostgres_OwnerIsolation(t *testing.T) {
	client, repo, input := newCreationExchangePGFixture(t)
	_, otherRepo, otherInput := newCreationExchangePGFixture(t)
	ctx := context.Background()
	foreign := input
	foreign.UserID = otherInput.UserID
	_, err := repo.CreateExchange(ctx, foreign)
	require.ErrorIs(t, err, service.ErrCreationSessionNotFound)
	require.Zero(t, countCreationExchangePGMessages(t, client, input.SessionID))
	first, err := repo.CreateExchange(ctx, input)
	require.NoError(t, err)
	before, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	_, err = otherRepo.CreateExchange(ctx, foreign)
	require.ErrorIs(t, err, service.ErrCreationSessionNotFound, "a known request ID must not expose another owner's exchange")
	require.Equal(t, 2, countCreationExchangePGMessages(t, client, input.SessionID))
	after, err := client.CreationSession.Get(ctx, input.SessionID)
	require.NoError(t, err)
	require.True(t, before.UpdatedAt.Equal(after.UpdatedAt))
	other, err := otherRepo.CreateExchange(ctx, otherInput)
	require.NoError(t, err, "different sessions may reuse the same request ID")
	require.NotEqual(t, first.User.ID, other.User.ID)
	require.NotEqual(t, first.Assistant.ID, other.Assistant.ID)
	require.Equal(t, 2, countCreationExchangePGMessages(t, client, otherInput.SessionID))
}
