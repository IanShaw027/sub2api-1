package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCreationVoiceTicketStoreConsumesAtomicallyAndExpires(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := &gatewayCache{rdb: client}
	ctx := context.Background()
	record := &service.CreationVoiceTicketRecord{UserID: 10, GroupID: 20}
	require.NoError(t, store.SaveCreationVoiceTicket(ctx, "hash", record, time.Minute))
	got, err := store.ConsumeCreationVoiceTicket(ctx, "hash")
	require.NoError(t, err)
	require.Equal(t, record, got)
	_, err = store.ConsumeCreationVoiceTicket(ctx, "hash")
	require.ErrorIs(t, err, service.ErrCreationVoiceTicketInvalid)
	require.NoError(t, store.SaveCreationVoiceTicket(ctx, "hash", record, time.Minute))
	server.FastForward(time.Minute)
	_, err = store.ConsumeCreationVoiceTicket(ctx, "hash")
	require.ErrorIs(t, err, service.ErrCreationVoiceTicketInvalid)
}
