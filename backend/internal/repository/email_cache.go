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
	passwordResetSentAtKeyPrefix = "password_reset_sent:"
	verifyCodeSendKeyPrefix      = "verify_code_send:"
	passwordResetClaimKeyPrefix  = "password_reset_claim:"
	notifyCodeUserRateKeyPrefix  = "notify_code_user_rate:"
)

var verifyAndConsumeCodeScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return 0 end
local ok, data = pcall(cjson.decode, raw)
if not ok then return redis.error_reply('invalid verification code data') end
local attempts = tonumber(data.Attempts) or 0
local max_attempts = tonumber(ARGV[2])
if attempts >= max_attempts then return 3 end
if tostring(data.Code) == ARGV[1] then
  redis.call('DEL', KEYS[1])
  return 1
end
attempts = attempts + 1
data.Attempts = attempts
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then
  redis.call('DEL', KEYS[1])
  return 0
end
redis.call('SET', KEYS[1], cjson.encode(data), 'PX', ttl)
if attempts >= max_attempts then return 3 end
return 2
`)

var reserveScript = redis.NewScript(`
if redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2], 'NX') then return 1 end
return 0
`)

var releaseReservationScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0
`)

var getOrCreateResetTokenScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if raw then return raw end
if redis.call('EXISTS', KEYS[2]) == 1 then
  return redis.error_reply('password reset token is currently claimed')
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
return ARGV[1]
`)

var claimResetTokenScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
local raw = redis.call('GET', KEYS[1])
if not raw then return 0 end
local ok, data = pcall(cjson.decode, raw)
if not ok then return redis.error_reply('invalid password reset token data') end
if tostring(data.Token) ~= ARGV[1] then return 0 end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then return 0 end
local claim = cjson.encode({claim_id = ARGV[2], token_data = raw})
redis.call('SET', KEYS[2], claim, 'PX', ttl)
redis.call('DEL', KEYS[1])
return 1
`)

var finalizeResetTokenClaimScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return 1 end
local ok, claim = pcall(cjson.decode, raw)
if not ok then return redis.error_reply('invalid password reset claim data') end
if tostring(claim.claim_id) ~= ARGV[1] then return 0 end
redis.call('DEL', KEYS[1])
return 1
`)

var restoreResetTokenScript = redis.NewScript(`
local raw = redis.call('GET', KEYS[2])
if not raw then return 1 end
local ok, claim = pcall(cjson.decode, raw)
if not ok then return redis.error_reply('invalid password reset claim data') end
if tostring(claim.claim_id) ~= ARGV[1] then return 0 end
if redis.call('EXISTS', KEYS[1]) == 0 then
  local ttl = redis.call('PTTL', KEYS[2])
  if ttl > 0 then redis.call('SET', KEYS[1], claim.token_data, 'PX', ttl) end
end
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

// passwordResetSentAtKey generates the Redis key for password reset email sent timestamp.
func passwordResetSentAtKey(email string) string {
	return passwordResetSentAtKeyPrefix + strings.ToLower(email)
}

func verifyCodeSendKey(email string) string {
	return verifyCodeSendKeyPrefix + strings.ToLower(email)
}

func passwordResetClaimKey(email string) string {
	return passwordResetClaimKeyPrefix + strings.ToLower(email)
}

type emailCache struct {
	rdb *redis.Client
}

var _ service.EmailAtomicCache = (*emailCache)(nil)

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

func (c *emailCache) VerifyAndConsumeVerificationCode(ctx context.Context, email, code string, maxAttempts int) (service.VerificationCodeCheckResult, error) {
	result, err := verifyAndConsumeCodeScript.Run(ctx, c.rdb, []string{verifyCodeKey(email)}, code, maxAttempts).Int()
	if err != nil {
		return service.VerificationCodeMissing, err
	}
	switch result {
	case 0:
		return service.VerificationCodeMissing, nil
	case 1:
		return service.VerificationCodeAccepted, nil
	case 2:
		return service.VerificationCodeRejected, nil
	case 3:
		return service.VerificationCodeLocked, nil
	default:
		return service.VerificationCodeMissing, fmt.Errorf("unexpected verification result: %d", result)
	}
}

func (c *emailCache) ReserveVerificationCodeSend(ctx context.Context, email, reservationID string, ttl time.Duration) (bool, error) {
	return c.reserve(ctx, verifyCodeSendKey(email), reservationID, ttl)
}

func (c *emailCache) ReleaseVerificationCodeSend(ctx context.Context, email, reservationID string) error {
	return c.releaseReservation(ctx, verifyCodeSendKey(email), reservationID)
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

func (c *emailCache) GetOrCreatePasswordResetToken(ctx context.Context, email string, candidate *service.PasswordResetTokenData, ttl time.Duration) (*service.PasswordResetTokenData, error) {
	raw, err := json.Marshal(candidate)
	if err != nil {
		return nil, err
	}
	result, err := getOrCreateResetTokenScript.Run(ctx, c.rdb, []string{passwordResetKey(email), passwordResetClaimKey(email)}, raw, ttl.Milliseconds()).Text()
	if err != nil {
		return nil, err
	}
	var data service.PasswordResetTokenData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) ClaimPasswordResetToken(ctx context.Context, email, token, claimID string) (bool, error) {
	result, err := claimResetTokenScript.Run(ctx, c.rdb, []string{passwordResetKey(email), passwordResetClaimKey(email)}, token, claimID).Int()
	return result == 1, err
}

func (c *emailCache) FinalizePasswordResetTokenClaim(ctx context.Context, email, claimID string) error {
	result, err := finalizeResetTokenClaimScript.Run(ctx, c.rdb, []string{passwordResetClaimKey(email)}, claimID).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return fmt.Errorf("password reset claim ownership mismatch")
	}
	return nil
}

func (c *emailCache) RestorePasswordResetTokenClaim(ctx context.Context, email, claimID string) error {
	result, err := restoreResetTokenScript.Run(ctx, c.rdb, []string{passwordResetKey(email), passwordResetClaimKey(email)}, claimID).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return fmt.Errorf("password reset claim ownership mismatch")
	}
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

func (c *emailCache) ReservePasswordResetEmailSend(ctx context.Context, email, reservationID string, ttl time.Duration) (bool, error) {
	return c.reserve(ctx, passwordResetSentAtKey(email), reservationID, ttl)
}

func (c *emailCache) ReleasePasswordResetEmailSend(ctx context.Context, email, reservationID string) error {
	return c.releaseReservation(ctx, passwordResetSentAtKey(email), reservationID)
}

func (c *emailCache) reserve(ctx context.Context, key, reservationID string, ttl time.Duration) (bool, error) {
	result, err := reserveScript.Run(ctx, c.rdb, []string{key}, reservationID, ttl.Milliseconds()).Int()
	return result == 1, err
}

func (c *emailCache) releaseReservation(ctx context.Context, key, reservationID string) error {
	return releaseReservationScript.Run(ctx, c.rdb, []string{key}, reservationID).Err()
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
