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
	verifyCodeKeyPrefix          = "verify_code:"
	notifyVerifyKeyPrefix        = "notify_verify:"
	passwordResetKeyPrefix       = "password_reset:"
	passwordResetClaimKeyPrefix  = "password_reset_claim:"
	passwordResetSentAtKeyPrefix = "password_reset_sent:"
	notifyCodeUserRateKeyPrefix  = "notify_code_user_rate:"
)

var claimPasswordResetTokenScript = redis.NewScript(`
local value = redis.call('GET', KEYS[1])
if not value then return 0 end
local ok, data = pcall(cjson.decode, value)
if not ok or not data or data.Token ~= ARGV[1] then return -1 end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then return 0 end
if not redis.call('SET', KEYS[2], value, 'PX', ttl, 'NX') then return -2 end
redis.call('DEL', KEYS[1])
return 1
`)

var restorePasswordResetTokenScript = redis.NewScript(`
local value = redis.call('GET', KEYS[2])
if not value then return 0 end
local ttl = redis.call('PTTL', KEYS[2])
if ttl <= 0 then
    redis.call('DEL', KEYS[2])
    return 0
end
if redis.call('EXISTS', KEYS[1]) == 1 then
    redis.call('DEL', KEYS[2])
    return -1
end
redis.call('SET', KEYS[1], value, 'PX', ttl)
redis.call('DEL', KEYS[2])
return 1
`)

// verifyCodeKey generates the Redis key for email verification code.
// Email is lowercased for case-insensitive consistency.
func verifyCodeKey(email string) string {
	return verifyCodeKeyPrefix + strings.ToLower(email)
}

// notifyVerifyKey generates the Redis key for notify email verification code.
// Email is lowercased to prevent case-sensitive key mismatch (the business layer
// uses strings.EqualFold for comparison).
func notifyVerifyKey(email string) string {
	return notifyVerifyKeyPrefix + strings.ToLower(email)
}

// passwordResetKey generates the Redis key for password reset token.
func passwordResetKey(email string) string {
	return passwordResetKeyPrefix + strings.ToLower(email)
}

func passwordResetClaimKey(email, claimID string) string {
	return passwordResetClaimKeyPrefix + strings.ToLower(email) + ":" + claimID
}

// passwordResetSentAtKey generates the Redis key for password reset email sent timestamp.
func passwordResetSentAtKey(email string) string {
	return passwordResetSentAtKeyPrefix + strings.ToLower(email)
}

type emailCache struct {
	rdb *redis.Client
}

func NewEmailCache(rdb *redis.Client) service.EmailCache {
	return &emailCache{rdb: rdb}
}

func (c *emailCache) GetVerificationCode(ctx context.Context, email string) (*service.VerificationCodeData, error) {
	key := verifyCodeKey(email)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.VerificationCodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) SetVerificationCode(ctx context.Context, email string, data *service.VerificationCodeData, ttl time.Duration) error {
	key := verifyCodeKey(email)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *emailCache) DeleteVerificationCode(ctx context.Context, email string) error {
	key := verifyCodeKey(email)
	return c.rdb.Del(ctx, key).Err()
}

// Password reset token methods

func (c *emailCache) GetPasswordResetToken(ctx context.Context, email string) (*service.PasswordResetTokenData, error) {
	key := passwordResetKey(email)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.PasswordResetTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) SetPasswordResetToken(ctx context.Context, email string, data *service.PasswordResetTokenData, ttl time.Duration) error {
	key := passwordResetKey(email)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *emailCache) DeletePasswordResetToken(ctx context.Context, email string) error {
	key := passwordResetKey(email)
	return c.rdb.Del(ctx, key).Err()
}

func (c *emailCache) ClaimPasswordResetToken(ctx context.Context, email, token, claimID string) error {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(claimID) == "" {
		return service.ErrInvalidResetToken
	}
	result, err := claimPasswordResetTokenScript.Run(
		ctx,
		c.rdb,
		[]string{passwordResetKey(email), passwordResetClaimKey(email, claimID)},
		token,
	).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return service.ErrInvalidResetToken
	}
	return nil
}

func (c *emailCache) CompletePasswordResetToken(ctx context.Context, email, claimID string) error {
	if strings.TrimSpace(claimID) == "" {
		return service.ErrInvalidResetToken
	}
	affected, err := c.rdb.Del(ctx, passwordResetClaimKey(email, claimID)).Result()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrInvalidResetToken
	}
	return nil
}

func (c *emailCache) RestorePasswordResetToken(ctx context.Context, email, claimID string) error {
	if strings.TrimSpace(claimID) == "" {
		return service.ErrInvalidResetToken
	}
	result, err := restorePasswordResetTokenScript.Run(
		ctx,
		c.rdb,
		[]string{passwordResetKey(email), passwordResetClaimKey(email, claimID)},
	).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return service.ErrInvalidResetToken
	}
	// -1 means a newer reset token already exists. The old claim is discarded
	// instead of overwriting the newer credential.
	return nil
}

// Password reset email cooldown methods

func (c *emailCache) IsPasswordResetEmailInCooldown(ctx context.Context, email string) bool {
	key := passwordResetSentAtKey(email)
	exists, err := c.rdb.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

func (c *emailCache) SetPasswordResetEmailCooldown(ctx context.Context, email string, ttl time.Duration) error {
	key := passwordResetSentAtKey(email)
	return c.rdb.Set(ctx, key, "1", ttl).Err()
}

// Notify email verification code methods

func (c *emailCache) GetNotifyVerifyCode(ctx context.Context, email string) (*service.VerificationCodeData, error) {
	key := notifyVerifyKey(email)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.VerificationCodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) SetNotifyVerifyCode(ctx context.Context, email string, data *service.VerificationCodeData, ttl time.Duration) error {
	key := notifyVerifyKey(email)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *emailCache) DeleteNotifyVerifyCode(ctx context.Context, email string) error {
	key := notifyVerifyKey(email)
	return c.rdb.Del(ctx, key).Err()
}

// User-level rate limiting for notify email verification codes

func notifyCodeUserRateKey(userID int64) string {
	return notifyCodeUserRateKeyPrefix + fmt.Sprintf("%d", userID)
}

func (c *emailCache) IncrNotifyCodeUserRate(ctx context.Context, userID int64, window time.Duration) (int64, error) {
	key := notifyCodeUserRateKey(userID)
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Always set TTL (idempotent) to avoid orphan keys if process crashes between INCR and EXPIRE.
	if err := c.rdb.Expire(ctx, key, window).Err(); err != nil {
		return count, fmt.Errorf("expire notify code rate key: %w", err)
	}
	return count, nil
}

func (c *emailCache) GetNotifyCodeUserRate(ctx context.Context, userID int64) (int64, error) {
	key := notifyCodeUserRateKey(userID)
	count, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}
