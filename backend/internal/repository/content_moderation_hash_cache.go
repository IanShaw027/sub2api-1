package repository

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const contentModerationFlaggedHashSetKey = "content_moderation:flagged_hashes"
const contentModerationFlaggedHashCreatedZSetKey = "content_moderation:flagged_hashes:created_at"
const contentModerationFlaggedHashKeyPrefix = "content_moderation:fh:"
const contentModerationFlaggedHashMetaKeyPrefix = "content_moderation:fh_meta:"
const contentModerationFlaggedHashHitsKeyPrefix = "content_moderation:fh_hits:"
const contentModerationFlaggedHashTTL = 90 * 24 * time.Hour
const contentModerationFlaggedHashHitTTL = 31 * 24 * time.Hour
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
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return err
	}
	nowUnix := now.Unix()
	expiresUnix := now.Add(contentModerationFlaggedHashTTL).Unix()
	metaKey := contentModerationFlaggedHashMetaKeyPrefix + inputHash
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, contentModerationFlaggedHashSetKey, inputHash)
	pipe.Set(ctx, contentModerationFlaggedHashKeyPrefix+inputHash, "1", contentModerationFlaggedHashTTL)
	pipe.HSetNX(ctx, metaKey, "created_at", nowUnix)
	pipe.HSet(ctx, metaKey, "expires_at", expiresUnix)
	pipe.Expire(ctx, metaKey, contentModerationFlaggedHashTTL)
	pipe.ZAddNX(ctx, contentModerationFlaggedHashCreatedZSetKey, redis.Z{Score: float64(nowUnix), Member: inputHash})
	_, err = pipe.Exec(ctx)
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
		if err := c.recordFlaggedInputHashHit(ctx, inputHash); err != nil {
			return false, err
		}
		return true, nil
	}
	matched, err := c.rdb.SIsMember(ctx, contentModerationFlaggedHashSetKey, inputHash).Result()
	if err != nil || !matched {
		return matched, err
	}
	_, _ = c.DeleteFlaggedInputHash(ctx, inputHash)
	return false, nil
}

func (c *contentModerationHashCache) recordFlaggedInputHashHit(ctx context.Context, inputHash string) error {
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return err
	}
	score := float64(now.Unix())
	member := fmt.Sprintf("%d:%d", now.UnixNano(), time.Now().UnixNano())
	key := contentModerationFlaggedHashHitsKeyPrefix + inputHash
	pipe := c.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(now.Add(-30*24*time.Hour).Unix()-1, 10))
	pipe.Expire(ctx, key, contentModerationFlaggedHashHitTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *contentModerationHashCache) ListFlaggedInputHashes(ctx context.Context, filter service.ContentModerationHashListFilter) ([]service.ContentModerationHashItem, *pagination.PaginationResult, error) {
	if c == nil || c.rdb == nil {
		return nil, &pagination.PaginationResult{Page: 1, PageSize: 20, Pages: 1}, nil
	}
	if err := c.pruneExpiredFlaggedHashes(ctx); err != nil {
		return nil, nil, err
	}
	page := filter.Pagination.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.Pagination.Limit()
	hashes, err := c.rdb.SMembers(ctx, contentModerationFlaggedHashSetKey).Result()
	if err != nil {
		return nil, nil, err
	}
	items := make([]service.ContentModerationHashItem, 0, len(hashes))
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, nil, err
	}
	for _, inputHash := range hashes {
		inputHash = strings.TrimSpace(inputHash)
		if inputHash == "" {
			continue
		}
		item, err := c.flaggedHashItem(ctx, inputHash, now)
		if err != nil {
			return nil, nil, err
		}
		if item.InputHash == "" {
			continue
		}
		items = append(items, item)
	}
	sortFlaggedHashItems(items, filter.Pagination.SortBy, filter.Pagination.NormalizedSortOrder(pagination.SortOrderDesc))
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}
	return items[start:end], &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}, nil
}

func (c *contentModerationHashCache) flaggedHashItem(ctx context.Context, inputHash string, now time.Time) (service.ContentModerationHashItem, error) {
	metaKey := contentModerationFlaggedHashMetaKeyPrefix + inputHash
	fields, err := c.rdb.HMGet(ctx, metaKey, "created_at", "expires_at").Result()
	if err != nil {
		return service.ContentModerationHashItem{}, err
	}
	createdUnix := redisInt64(fields[0])
	expiresUnix := redisInt64(fields[1])
	if createdUnix <= 0 {
		score, err := c.rdb.ZScore(ctx, contentModerationFlaggedHashCreatedZSetKey, inputHash).Result()
		if err == nil {
			createdUnix = int64(score)
		}
	}
	if createdUnix <= 0 {
		createdUnix = now.Unix()
	}
	if expiresUnix <= 0 {
		expiresUnix = createdUnix + int64(contentModerationFlaggedHashTTL/time.Second)
	}
	expiresAt := time.Unix(expiresUnix, 0)
	if !expiresAt.After(now) {
		_, _ = c.DeleteFlaggedInputHash(ctx, inputHash)
		return service.ContentModerationHashItem{}, nil
	}
	hitsKey := contentModerationFlaggedHashHitsKeyPrefix + inputHash
	hit7d, err := c.rdb.ZCount(ctx, hitsKey, strconv.FormatInt(now.Add(-7*24*time.Hour).Unix(), 10), "+inf").Result()
	if err != nil {
		return service.ContentModerationHashItem{}, err
	}
	hit30d, err := c.rdb.ZCount(ctx, hitsKey, strconv.FormatInt(now.Add(-30*24*time.Hour).Unix(), 10), "+inf").Result()
	if err != nil {
		return service.ContentModerationHashItem{}, err
	}
	return service.ContentModerationHashItem{
		InputHash:   inputHash,
		CreatedAt:   time.Unix(createdUnix, 0),
		ExpiresAt:   expiresAt,
		HitCount7D:  hit7d,
		HitCount30D: hit30d,
	}, nil
}

func sortFlaggedHashItems(items []service.ContentModerationHashItem, sortBy string, sortOrder string) {
	desc := sortOrder != pagination.SortOrderAsc
	sort.SliceStable(items, func(i, j int) bool {
		var less bool
		switch strings.ToLower(strings.TrimSpace(sortBy)) {
		case service.ContentModerationHashSortHits7D:
			if items[i].HitCount7D != items[j].HitCount7D {
				less = items[i].HitCount7D < items[j].HitCount7D
				break
			}
			less = items[i].CreatedAt.Before(items[j].CreatedAt)
		case service.ContentModerationHashSortHits30D:
			if items[i].HitCount30D != items[j].HitCount30D {
				less = items[i].HitCount30D < items[j].HitCount30D
				break
			}
			less = items[i].CreatedAt.Before(items[j].CreatedAt)
		default:
			if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
				less = items[i].CreatedAt.Before(items[j].CreatedAt)
				break
			}
			less = items[i].InputHash < items[j].InputHash
		}
		if desc {
			return !less
		}
		return less
	})
}

func (c *contentModerationHashCache) DeleteFlaggedInputHash(ctx context.Context, inputHash string) (bool, error) {
	inputHash = strings.TrimSpace(inputHash)
	if c == nil || c.rdb == nil || inputHash == "" {
		return false, nil
	}
	pipe := c.rdb.Pipeline()
	srem := pipe.SRem(ctx, contentModerationFlaggedHashSetKey, inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashHitsKeyPrefix+inputHash)
	pipe.ZRem(ctx, contentModerationFlaggedHashCreatedZSetKey, inputHash)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	return srem.Val() > 0, nil
}

func (c *contentModerationHashCache) DeleteFlaggedInputHashes(ctx context.Context, inputHashes []string) (int64, error) {
	if c == nil || c.rdb == nil || len(inputHashes) == 0 {
		return 0, nil
	}
	seen := map[string]struct{}{}
	var deleted int64
	for _, inputHash := range inputHashes {
		inputHash = strings.TrimSpace(inputHash)
		if inputHash == "" {
			continue
		}
		if _, ok := seen[inputHash]; ok {
			continue
		}
		seen[inputHash] = struct{}{}
		ok, err := c.DeleteFlaggedInputHash(ctx, inputHash)
		if err != nil {
			return deleted, err
		}
		if ok {
			deleted++
		}
	}
	return deleted, nil
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
		pipe.Del(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash)
		pipe.Del(ctx, contentModerationFlaggedHashHitsKeyPrefix+inputHash)
	}
	pipe.Del(ctx, contentModerationFlaggedHashSetKey)
	pipe.Del(ctx, contentModerationFlaggedHashCreatedZSetKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return int64(len(hashes)), nil
}

func (c *contentModerationHashCache) CountFlaggedInputHashes(ctx context.Context) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	if err := c.pruneExpiredFlaggedHashes(ctx); err != nil {
		return 0, err
	}
	return c.rdb.SCard(ctx, contentModerationFlaggedHashSetKey).Result()
}

func (c *contentModerationHashCache) pruneExpiredFlaggedHashes(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return err
	}
	hashes, err := c.rdb.SMembers(ctx, contentModerationFlaggedHashSetKey).Result()
	if err != nil {
		return err
	}
	for _, inputHash := range hashes {
		exists, err := c.rdb.Exists(ctx, contentModerationFlaggedHashKeyPrefix+inputHash).Result()
		if err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		expiresUnix, err := c.rdb.HGet(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash, "expires_at").Int64()
		if err != nil && err != redis.Nil {
			return err
		}
		if expiresUnix <= 0 || expiresUnix <= now.Unix() {
			if _, err := c.DeleteFlaggedInputHash(ctx, inputHash); err != nil {
				return err
			}
		}
	}
	return nil
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
