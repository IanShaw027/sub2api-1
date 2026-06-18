package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const contentModerationFlaggedHashSetKey = "content_moderation:flagged_hashes"
const contentModerationFlaggedHashKeyPrefix = "content_moderation:fh:"
const contentModerationFlaggedHashTTL = 7 * 24 * time.Hour
const contentModerationAPIKeyQuotaKeyPrefix = "content_moderation:api_key_quota:"
const contentModerationAPIKeyQuotaTTL = 48 * time.Hour

var contentModerationAPIKeyQuotaAdmitScript = redis.NewScript(`
local now = tonumber(redis.call("TIME")[1])
local rpm_limit = tonumber(ARGV[1]) or 0
local rpd_limit = tonumber(ARGV[2]) or 0
local tpm_limit = tonumber(ARGV[3]) or 0
local token_estimate = tonumber(ARGV[4]) or 1
if token_estimate < 1 then token_estimate = 1 end
local ttl = tonumber(ARGV[5]) or 172800

local rpm_used = tonumber(redis.call("HGET", KEYS[1], "rpm_used") or "0")
local rpd_used = tonumber(redis.call("HGET", KEYS[1], "rpd_used") or "0")
local tpm_used = tonumber(redis.call("HGET", KEYS[1], "tpm_used") or "0")
local rpm_window_start = tonumber(redis.call("HGET", KEYS[1], "rpm_window_start") or "0")
local rpd_window_start = tonumber(redis.call("HGET", KEYS[1], "rpd_window_start") or "0")
local tpm_window_start = tonumber(redis.call("HGET", KEYS[1], "tpm_window_start") or "0")

if rpm_window_start <= 0 or now - rpm_window_start >= 60 then
  rpm_window_start = now
  rpm_used = 0
end
if rpd_window_start <= 0 or now - rpd_window_start >= 86400 then
  rpd_window_start = now
  rpd_used = 0
end
if tpm_window_start <= 0 or now - tpm_window_start >= 60 then
  tpm_window_start = now
  tpm_used = 0
end

local denied_reason = ""
local frozen_until = 0
if rpm_limit > 0 and rpm_used + 1 > rpm_limit then
  denied_reason = "RPM"
  frozen_until = rpm_window_start + 60
elseif rpd_limit > 0 and rpd_used + 1 > rpd_limit then
  denied_reason = "RPD"
  frozen_until = rpd_window_start + 86400
elseif tpm_limit > 0 and tpm_used + token_estimate > tpm_limit then
  denied_reason = "TPM"
  frozen_until = tpm_window_start + 60
end

if denied_reason ~= "" then
  redis.call("HSET", KEYS[1],
    "rpm_used", rpm_used,
    "rpd_used", rpd_used,
    "tpm_used", tpm_used,
    "rpm_window_start", rpm_window_start,
    "rpd_window_start", rpd_window_start,
    "tpm_window_start", tpm_window_start,
    "frozen_until", frozen_until,
    "last_denied_reason", denied_reason
  )
  redis.call("EXPIRE", KEYS[1], ttl)
  return {0, rpm_used, rpd_used, tpm_used, rpm_window_start + 60, rpd_window_start + 86400, tpm_window_start + 60, frozen_until, denied_reason}
end

rpm_used = rpm_used + 1
rpd_used = rpd_used + 1
tpm_used = tpm_used + token_estimate
redis.call("HSET", KEYS[1],
  "rpm_used", rpm_used,
  "rpd_used", rpd_used,
  "tpm_used", tpm_used,
  "rpm_window_start", rpm_window_start,
  "rpd_window_start", rpd_window_start,
  "tpm_window_start", tpm_window_start,
  "frozen_until", 0,
  "last_denied_reason", ""
)
redis.call("EXPIRE", KEYS[1], ttl)
return {1, rpm_used, rpd_used, tpm_used, rpm_window_start + 60, rpd_window_start + 86400, tpm_window_start + 60, 0, ""}
`)

type contentModerationHashCache struct {
	rdb *redis.Client
}

func NewContentModerationHashCache(rdb *redis.Client) service.ContentModerationHashCache {
	return &contentModerationHashCache{rdb: rdb}
}

func (c *contentModerationHashCache) RecordFlaggedInputHash(ctx context.Context, inputHash string) error {
	inputHash = strings.TrimSpace(inputHash)
	if c == nil || c.rdb == nil || inputHash == "" {
		return nil
	}
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, contentModerationFlaggedHashSetKey, inputHash)
	pipe.Set(ctx, contentModerationFlaggedHashKeyPrefix+inputHash, "1", contentModerationFlaggedHashTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *contentModerationHashCache) HasFlaggedInputHash(ctx context.Context, inputHash string) (bool, error) {
	inputHash = strings.TrimSpace(inputHash)
	if c == nil || c.rdb == nil || inputHash == "" {
		return false, nil
	}
	exists, err := c.rdb.Exists(ctx, contentModerationFlaggedHashKeyPrefix+inputHash).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	}
	return c.rdb.SIsMember(ctx, contentModerationFlaggedHashSetKey, inputHash).Result()
}

func (c *contentModerationHashCache) DeleteFlaggedInputHash(ctx context.Context, inputHash string) (bool, error) {
	inputHash = strings.TrimSpace(inputHash)
	if c == nil || c.rdb == nil || inputHash == "" {
		return false, nil
	}
	pipe := c.rdb.Pipeline()
	srem := pipe.SRem(ctx, contentModerationFlaggedHashSetKey, inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	return srem.Val() > 0, nil
}

func (c *contentModerationHashCache) ClearFlaggedInputHashes(ctx context.Context) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	hashes, err := c.rdb.SMembers(ctx, contentModerationFlaggedHashSetKey).Result()
	if err != nil {
		return 0, err
	}
	if len(hashes) == 0 {
		return 0, nil
	}
	pipe := c.rdb.Pipeline()
	for _, inputHash := range hashes {
		inputHash = strings.TrimSpace(inputHash)
		if inputHash == "" {
			continue
		}
		pipe.Del(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
	}
	pipe.Del(ctx, contentModerationFlaggedHashSetKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return int64(len(hashes)), nil
}

func (c *contentModerationHashCache) CountFlaggedInputHashes(ctx context.Context) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	return c.rdb.SCard(ctx, contentModerationFlaggedHashSetKey).Result()
}

func (c *contentModerationHashCache) AdmitModerationAPIKeyQuota(ctx context.Context, keyHash string, rpmLimit int, rpdLimit int, tpmLimit int, tokenEstimate int) (*service.ContentModerationAPIKeyQuotaState, bool, error) {
	keyHash = strings.TrimSpace(keyHash)
	if c == nil || c.rdb == nil || keyHash == "" {
		return nil, false, nil
	}
	vals, err := contentModerationAPIKeyQuotaAdmitScript.Run(
		ctx,
		c.rdb,
		[]string{contentModerationAPIKeyQuotaKeyPrefix + keyHash},
		rpmLimit,
		rpdLimit,
		tpmLimit,
		tokenEstimate,
		int64(contentModerationAPIKeyQuotaTTL/time.Second),
	).Slice()
	if err != nil {
		return nil, false, err
	}
	if len(vals) != 9 {
		return nil, false, fmt.Errorf("content moderation api key quota admit returned %d values", len(vals))
	}
	state := &service.ContentModerationAPIKeyQuotaState{
		RPMUsed:          redisInt64(vals[1]),
		RPDUsed:          redisInt64(vals[2]),
		TPMUsed:          redisInt64(vals[3]),
		RPMResetAt:       redisUnixTime(vals[4]),
		RPDResetAt:       redisUnixTime(vals[5]),
		TPMResetAt:       redisUnixTime(vals[6]),
		FrozenUntil:      redisUnixTime(vals[7]),
		LastDeniedReason: redisString(vals[8]),
	}
	return state, redisInt64(vals[0]) == 1, nil
}

func (c *contentModerationHashCache) GetModerationAPIKeyQuotaState(ctx context.Context, keyHash string) (*service.ContentModerationAPIKeyQuotaState, error) {
	keyHash = strings.TrimSpace(keyHash)
	if c == nil || c.rdb == nil || keyHash == "" {
		return nil, nil
	}
	fields, err := c.rdb.HMGet(ctx,
		contentModerationAPIKeyQuotaKeyPrefix+keyHash,
		"rpm_used",
		"rpd_used",
		"tpm_used",
		"rpm_window_start",
		"rpd_window_start",
		"tpm_window_start",
		"frozen_until",
		"last_denied_reason",
	).Result()
	if err != nil {
		return nil, err
	}
	if len(fields) != 8 {
		return nil, nil
	}
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, err
	}
	nowUnix := now.Unix()
	rpmWindowStart := redisInt64(fields[3])
	rpdWindowStart := redisInt64(fields[4])
	tpmWindowStart := redisInt64(fields[5])
	state := &service.ContentModerationAPIKeyQuotaState{
		RPMUsed:          redisInt64(fields[0]),
		RPDUsed:          redisInt64(fields[1]),
		TPMUsed:          redisInt64(fields[2]),
		RPMResetAt:       windowResetAt(rpmWindowStart, 60, nowUnix),
		RPDResetAt:       windowResetAt(rpdWindowStart, 86400, nowUnix),
		TPMResetAt:       windowResetAt(tpmWindowStart, 60, nowUnix),
		FrozenUntil:      redisUnixTime(fields[6]),
		LastDeniedReason: redisString(fields[7]),
	}
	if !state.RPMResetAt.After(now) {
		state.RPMUsed = 0
	}
	if !state.RPDResetAt.After(now) {
		state.RPDUsed = 0
	}
	if !state.TPMResetAt.After(now) {
		state.TPMUsed = 0
	}
	if !state.FrozenUntil.After(now) {
		state.FrozenUntil = time.Time{}
		state.LastDeniedReason = ""
	}
	return state, nil
}

func redisInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(v), 10, 64)
		return n
	default:
		return 0
	}
}

func redisString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		return ""
	}
}

func redisUnixTime(value any) time.Time {
	unix := redisInt64(value)
	if unix <= 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

func windowResetAt(windowStart int64, windowSeconds int64, nowUnix int64) time.Time {
	if windowStart <= 0 {
		return time.Time{}
	}
	resetAt := time.Unix(windowStart+windowSeconds, 0)
	if windowStart+windowSeconds <= nowUnix {
		return time.Time{}
	}
	return resetAt
}
