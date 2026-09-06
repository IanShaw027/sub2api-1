//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newCreationPublicationPGFixture(t *testing.T) (service.CreationPublicationRepository, *service.CreationPublication) {
	t.Helper()
	user := mustCreateUser(t, testEntClient(t), &service.User{
		Email: "publication-" + uuid.NewString() + "@example.com", Concurrency: 1,
	})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})
	requestID := uuid.NewString()
	return NewCreationPublicationRepository(integrationDB), &service.CreationPublication{
		OwnerUserID: user.ID, RequestID: requestID,
		Title: "publication-" + requestID, Prompt: "Private source prompt", Model: "gpt-image-1",
		Kind: "image", MIME: "image/png", Size: 128,
		StorageKey: "publications/" + uuid.NewString() + ".png", StorageProfileID: "test-profile",
		SHA256: strings.Repeat("a", 64),
	}
}

func TestCreationPublicationPostgres_Migration245(t *testing.T) {
	// The harness applies the actual embedded migrations before opening its Ent client.
	ctx := context.Background()
	var applied int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM schema_migrations WHERE filename = $1", "245_creation_publications.sql").Scan(&applied))
	require.Equal(t, 1, applied)
	tx := testTx(t)
	requireColumn(t, tx, "creation_publications", "owner_user_id", "bigint", 0, false)
	requireColumn(t, tx, "creation_publications", "request_id", "uuid", 0, false)
	requireColumn(t, tx, "creation_publications", "status", "character varying", 16, false)
	requireColumnDefaultContains(t, tx, "creation_publications", "status", "pending")
	requireConstraintDefinitionContains(t, tx, "creation_publications",
		"creation_publications_owner_request_unique", "UNIQUE", "owner_user_id", "request_id")
	requireIndex(t, tx, "creation_publications", "creation_publications_gallery_idx")
	requireIndex(t, tx, "creation_publications", "creation_publications_kind_gallery_idx")

	repo, input := newCreationPublicationPGFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(*service.CreationPublication)
	}{
		{"invalid kind", func(p *service.CreationPublication) { p.Kind = "document" }},
		{"invalid MIME", func(p *service.CreationPublication) { p.MIME = "text/html" }},
		{"empty media", func(p *service.CreationPublication) { p.Size = 0 }},
		{"oversized media", func(p *service.CreationPublication) { p.Size = 67108865 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := *input
			invalid.RequestID = uuid.NewString()
			invalid.StorageKey = "publications/" + uuid.NewString()
			tc.mutate(&invalid)
			_, _, err := repo.CreateIfAbsent(ctx, &invalid)
			require.Error(t, err, "PostgreSQL must enforce publication media constraints")
			_, err = repo.GetByRequestID(ctx, invalid.OwnerUserID, invalid.RequestID)
			require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)
		})
	}
}

func TestCreationPublicationPostgres_ConcurrentOwnerRequestIdempotency(t *testing.T) {
	repo, input := newCreationPublicationPGFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const count = 8
	type result struct {
		publication *service.CreationPublication
		created     bool
		err         error
	}
	results := make(chan result, count)
	start := make(chan struct{})
	var ready, workers sync.WaitGroup
	ready.Add(count)
	workers.Add(count)
	for i := range count {
		go func(index int) {
			defer workers.Done()
			candidate := *input
			candidate.Title = fmt.Sprintf("candidate-%d", index)
			candidate.StorageKey = fmt.Sprintf("%s-%d", input.StorageKey, index)
			ready.Done()
			<-start
			publication, created, err := repo.CreateIfAbsent(ctx, &candidate)
			results <- result{publication: publication, created: created, err: err}
		}(i)
	}
	ready.Wait()
	close(start)
	workers.Wait()
	close(results)
	var winner *service.CreationPublication
	createdCount := 0
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.publication)
		if result.created {
			createdCount++
		}
		if winner == nil {
			winner = result.publication
		}
		require.Equal(t, winner.ID, result.publication.ID)
		require.Equal(t, winner.StorageKey, result.publication.StorageKey)
		require.Equal(t, winner.Title, result.publication.Title)
		require.Equal(t, service.CreationPublicationPending, result.publication.Status)
	}
	require.Equal(t, 1, createdCount)
	var storedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM creation_publications WHERE owner_user_id = $1 AND request_id = $2",
		input.OwnerUserID, input.RequestID).Scan(&storedCount))
	require.Equal(t, 1, storedCount)
	replayed, created, err := repo.CreateIfAbsent(ctx, input)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, winner.ID, replayed.ID)
	require.Equal(t, winner.StorageKey, replayed.StorageKey)
}

func TestCreationPublicationPostgres_OwnerIsolationAndExplicitActivation(t *testing.T) {
	repo, input := newCreationPublicationPGFixture(t)
	_, other := newCreationPublicationPGFixture(t)
	ctx := context.Background()
	pending, created, err := repo.CreateIfAbsent(ctx, input)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, service.CreationPublicationPending, pending.Status)
	_, err = repo.GetPublic(ctx, pending.ID)
	require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)
	items, total, err := repo.List(ctx, service.CreationPublicationFilter{Page: 1, PageSize: 20, Search: input.Title})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
	_, err = repo.GetByRequestID(ctx, other.OwnerUserID, input.RequestID)
	require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)
	_, err = repo.Withdraw(ctx, other.OwnerUserID, pending.ID)
	require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)

	other.RequestID = input.RequestID
	otherPending, created, err := repo.CreateIfAbsent(ctx, other)
	require.NoError(t, err)
	require.True(t, created, "idempotency must be scoped to the owner, not global UUID")
	require.NotEqual(t, pending.ID, otherPending.ID)
	owned, err := repo.GetByRequestID(ctx, other.OwnerUserID, input.RequestID)
	require.NoError(t, err)
	require.Equal(t, otherPending.ID, owned.ID)

	published, err := repo.Activate(ctx, pending.ID)
	require.NoError(t, err)
	require.Equal(t, service.CreationPublicationPublished, published.Status)
	public, err := repo.GetPublic(ctx, pending.ID)
	require.NoError(t, err)
	require.Equal(t, input.OwnerUserID, public.OwnerUserID)
	items, total, err = repo.List(ctx, service.CreationPublicationFilter{Page: 1, PageSize: 20, Search: input.Title})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, pending.ID, items[0].ID)
	_, err = repo.GetPublic(ctx, otherPending.ID)
	require.ErrorIs(t, err, service.ErrCreationPublicationNotFound, "activating one owner's request cannot activate another's")
}

func TestCreationPublicationPostgres_WithdrawCannotBeReactivated(t *testing.T) {
	for _, activateFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending upload", true: "published upload"}[activateFirst], func(t *testing.T) {
			repo, input := newCreationPublicationPGFixture(t)
			ctx := context.Background()
			publication, _, err := repo.CreateIfAbsent(ctx, input)
			require.NoError(t, err)
			if activateFirst {
				_, err = repo.Activate(ctx, publication.ID)
				require.NoError(t, err)
			}
			withdrawn, err := repo.Withdraw(ctx, input.OwnerUserID, publication.ID)
			require.NoError(t, err)
			require.NotNil(t, withdrawn.WithdrawnAt)
			repeated, err := repo.Withdraw(ctx, input.OwnerUserID, publication.ID)
			require.NoError(t, err)
			require.Equal(t, withdrawn.WithdrawnAt, repeated.WithdrawnAt)
			_, err = repo.Activate(ctx, publication.ID)
			require.ErrorIs(t, err, service.ErrCreationPublicationNotFound, "late storage completion must not resurrect a withdrawn record")
			_, err = repo.GetPublic(ctx, publication.ID)
			require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)
			replayed, created, err := repo.CreateIfAbsent(ctx, input)
			require.NoError(t, err)
			require.False(t, created)
			require.Equal(t, publication.ID, replayed.ID)
			require.Equal(t, withdrawn.WithdrawnAt, replayed.WithdrawnAt)
			items, total, err := repo.List(ctx, service.CreationPublicationFilter{Page: 1, PageSize: 20, Search: input.Title})
			require.NoError(t, err)
			require.Zero(t, total)
			require.Empty(t, items)
		})
	}
}

func TestCreationPublicationPostgres_SearchAndStablePagination(t *testing.T) {
	repo, base := newCreationPublicationPGFixture(t)
	ctx := context.Background()
	search := "gallery-" + uuid.NewString()
	titleSearch := "title-" + uuid.NewString()
	modelSearch := "model-" + uuid.NewString()
	var publishedIDs []int64
	for i := range 5 {
		input := *base
		input.RequestID = uuid.NewString()
		input.StorageKey = "publications/" + uuid.NewString()
		input.Title = fmt.Sprintf("Example %d", i)
		input.Prompt = search
		if i == 0 {
			input.Title = titleSearch
		}
		if i == 1 {
			input.Model = modelSearch
		}
		if i == 2 {
			input.Kind, input.MIME = "video", "video/mp4"
		}
		publication, _, err := repo.CreateIfAbsent(ctx, &input)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "UPDATE creation_publications SET created_at = $1 WHERE id = $2",
			time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), publication.ID)
		require.NoError(t, err)
		if i == 3 {
			continue // Pending storage uploads are never gallery entries.
		}
		_, err = repo.Activate(ctx, publication.ID)
		require.NoError(t, err)
		if i == 4 {
			_, err = repo.Withdraw(ctx, input.OwnerUserID, publication.ID)
			require.NoError(t, err)
			continue
		}
		publishedIDs = append(publishedIDs, publication.ID)
	}
	filter := service.CreationPublicationFilter{Page: 1, PageSize: 2, Search: strings.ToUpper(search)}
	first, total, err := repo.List(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, first, 2)
	require.Equal(t, []int64{publishedIDs[2], publishedIDs[1]}, []int64{first[0].ID, first[1].ID})
	filter.Page = 2
	second, total, err := repo.List(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, second, 1)
	require.Equal(t, publishedIDs[0], second[0].ID)
	filter.Page = 3
	empty, total, err := repo.List(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Empty(t, empty)

	filter.Page, filter.Kind = 1, "image"
	images, total, err := repo.List(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, images, 2)
	for _, image := range images {
		require.Equal(t, "image", image.Kind)
	}
	filter.Kind = "video"
	videos, total, err := repo.List(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, videos, 1)
	require.Equal(t, publishedIDs[2], videos[0].ID)
	for _, term := range []string{titleSearch, modelSearch} {
		matches, matched, err := repo.List(ctx, service.CreationPublicationFilter{Page: 1, PageSize: 20, Search: strings.ToUpper(term)})
		require.NoError(t, err)
		require.EqualValues(t, 1, matched)
		require.Len(t, matches, 1)
	}
}
