package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenKeyPrefix      = "refresh_token:"
	refreshTokenUsedKeyPrefix  = "refresh_token_used:"
	refreshTokenGraceKeyPrefix = "refresh_token_used_grace:"
	userRefreshTokensPrefix    = "user_refresh_tokens:"
	tokenFamilyPrefix          = "token_family:"
)

var consumeRefreshTokenScript = redis.NewScript(`
local value = redis.call('GET', KEYS[1])
if not value then return false end
redis.call('DEL', KEYS[1])
redis.call('PSETEX', KEYS[2], ARGV[1], ARGV[2])
redis.call('PSETEX', KEYS[3], ARGV[3], '1')
return value`)

// refreshTokenKey generates the Redis key for a refresh token.
func refreshTokenKey(tokenHash string) string {
	return refreshTokenKeyPrefix + tokenHash
}

// userRefreshTokensKey generates the Redis key for user's token set.
func userRefreshTokensKey(userID int64) string {
	return fmt.Sprintf("%s%d", userRefreshTokensPrefix, userID)
}

// tokenFamilyKey generates the Redis key for token family set.
func tokenFamilyKey(familyID string) string {
	return tokenFamilyPrefix + familyID
}

type refreshTokenCache struct {
	rdb *redis.Client
}

// NewRefreshTokenCache creates a new RefreshTokenCache implementation.
func NewRefreshTokenCache(rdb *redis.Client) service.RefreshTokenCache {
	return &refreshTokenCache{rdb: rdb}
}

func (c *refreshTokenCache) StoreRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	key := refreshTokenKey(tokenHash)
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal refresh token data: %w", err)
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *refreshTokenCache) GetRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	key := refreshTokenKey(tokenHash)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	var data service.RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token data: %w", err)
	}
	return &data, nil
}

func (c *refreshTokenCache) ConsumeRefreshToken(ctx context.Context, tokenHash, familyID string, usedTTL time.Duration) (*service.RefreshTokenData, error) {
	if strings.TrimSpace(tokenHash) == "" || strings.TrimSpace(familyID) == "" {
		return nil, service.ErrRefreshTokenNotFound
	}
	if usedTTL <= 0 {
		usedTTL = time.Hour
	}
	marker, err := json.Marshal(service.UsedRefreshTokenMarker{
		FamilyID:   familyID,
		ConsumedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal used refresh token marker: %w", err)
	}
	value, err := consumeRefreshTokenScript.Run(
		ctx,
		c.rdb,
		[]string{refreshTokenKey(tokenHash), refreshTokenUsedKey(tokenHash), refreshTokenGraceKey(tokenHash)},
		usedTTL.Milliseconds(),
		string(marker),
		service.RefreshTokenConcurrentReuseGrace.Milliseconds(),
	).Text()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	var data service.RefreshTokenData
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, fmt.Errorf("unmarshal consumed refresh token data: %w", err)
	}
	return &data, nil
}

func (c *refreshTokenCache) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	key := refreshTokenKey(tokenHash)
	return c.rdb.Del(ctx, key).Err()
}

func refreshTokenUsedKey(tokenHash string) string {
	return refreshTokenUsedKeyPrefix + tokenHash
}

func refreshTokenGraceKey(tokenHash string) string {
	return refreshTokenGraceKeyPrefix + tokenHash
}

func (c *refreshTokenCache) GetUsedRefreshTokenFamily(ctx context.Context, tokenHash string) (string, error) {
	marker, err := c.GetUsedRefreshTokenMarker(ctx, tokenHash)
	if err != nil {
		return "", err
	}
	return marker.FamilyID, nil
}

func (c *refreshTokenCache) GetUsedRefreshTokenMarker(ctx context.Context, tokenHash string) (*service.UsedRefreshTokenMarker, error) {
	val, err := c.rdb.Get(ctx, refreshTokenUsedKey(tokenHash)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(val) == "" {
		return nil, service.ErrRefreshTokenNotFound
	}
	var marker service.UsedRefreshTokenMarker
	if err := json.Unmarshal([]byte(val), &marker); err == nil && strings.TrimSpace(marker.FamilyID) != "" {
		return &marker, nil
	}
	// Markers written by the first rotation implementation contained only the
	// family ID. Treat them as outside the concurrency grace window so a real
	// replay retains the previous fail-closed behavior during rolling upgrades.
	return &service.UsedRefreshTokenMarker{FamilyID: val}, nil
}

func (c *refreshTokenCache) IsRefreshTokenReuseGraceActive(ctx context.Context, tokenHash string) (bool, error) {
	count, err := c.rdb.Exists(ctx, refreshTokenGraceKey(tokenHash)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (c *refreshTokenCache) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	// Get all token hashes for this user
	tokenHashes, err := c.GetUserTokenHashes(ctx, userID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get user token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		return nil
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
	}
	keys = append(keys, userRefreshTokensKey(userID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) DeleteTokenFamily(ctx context.Context, familyID string) error {
	// Get all token hashes in this family
	tokenHashes, err := c.GetFamilyTokenHashes(ctx, familyID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get family token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		return nil
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
	}
	keys = append(keys, tokenFamilyKey(familyID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	key := userRefreshTokensKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	key := tokenFamilyKey(familyID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	key := userRefreshTokensKey(userID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SIsMember(ctx, key, tokenHash).Result()
}
