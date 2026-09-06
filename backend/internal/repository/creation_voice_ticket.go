package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const creationVoiceTicketPrefix = "creation:voice-ticket:"

func (c *gatewayCache) SaveCreationVoiceTicket(ctx context.Context, hash string, record *service.CreationVoiceTicketRecord, ttl time.Duration) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, creationVoiceTicketPrefix+hash, payload, ttl).Err()
}

func (c *gatewayCache) ConsumeCreationVoiceTicket(ctx context.Context, hash string) (*service.CreationVoiceTicketRecord, error) {
	payload, err := c.rdb.GetDel(ctx, creationVoiceTicketPrefix+hash).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrCreationVoiceTicketInvalid
	}
	if err != nil {
		return nil, service.ErrCreationVoiceTicketUnavailable
	}
	var record service.CreationVoiceTicketRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, service.ErrCreationVoiceTicketInvalid
	}
	return &record, nil
}
