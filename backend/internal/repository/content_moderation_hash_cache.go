package repository

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// contentModerationFlaggedHashSetKey 为旧的 flagged hash SET 索引（无 TTL，且读取需 SMembers 全量加载）。
// 新实现改以 created-ZSet 为唯一权威索引，仅保留该常量以清理历史遗留数据。
const contentModerationFlaggedHashSetKey = "content_moderation:flagged_hashes"
const contentModerationFlaggedHashCreatedZSetKey = "content_moderation:flagged_hashes:created_at"
const contentModerationFlaggedHashKeyPrefix = "content_moderation:fh:"
const contentModerationFlaggedHashMetaKeyPrefix = "content_moderation:fh_meta:"

// contentModerationFlaggedHashHitsKeyPrefix 为旧的 ZSet 命中记录 key（每命中一个纳秒唯一成员，无界）。
// 已被 contentModerationFlaggedHashHitCountKeyPrefix（按业务日分桶 Hash）取代，仅保留删除路径引用
// 以清理遗留数据；遗留 ZSet 也会靠自身 31 天 TTL 自然过期消亡。
const contentModerationFlaggedHashHitsKeyPrefix = "content_moderation:fh_hits:"
const contentModerationFlaggedHashHitCountKeyPrefix = "content_moderation:fh_hitc:"
const flaggedHashHitDayLayout = "20060102"
const contentModerationFlaggedHashTTL = 90 * 24 * time.Hour
const contentModerationFlaggedHashHitTTL = 31 * 24 * time.Hour
const contentModerationAPIKeyQuotaKeyPrefix = "content_moderation:api_key_quota:"
const contentModerationAPIKeyQuotaTTL = 48 * time.Hour

// flaggedHashPipelineChunkSize 为按 hash 批量读写时单次 pipeline 的分片上限，防止大集合下单个 pipeline
// 命令数无界膨胀（每 hash 最多 5 个删除命令 / 3 个读命令）。
const flaggedHashPipelineChunkSize = 500

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

func (c *contentModerationHashCache) RecordFlaggedInputHash(ctx context.Context, inputHash string, meta service.ContentModerationHashMeta) error {
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
	pipe.Set(ctx, contentModerationFlaggedHashKeyPrefix+inputHash, "1", contentModerationFlaggedHashTTL)
	pipe.HSetNX(ctx, metaKey, "created_at", nowUnix)
	pipe.HSet(ctx, metaKey, "expires_at", expiresUnix)
	// 仅在首次记录该哈希时写入脱敏上下文（HSetNX），保留最初命中的输入内容；调用方需保证字段已脱敏。
	if value := strings.TrimSpace(meta.Excerpt); value != "" {
		pipe.HSetNX(ctx, metaKey, "excerpt", value)
	}
	if value := strings.TrimSpace(meta.Action); value != "" {
		pipe.HSetNX(ctx, metaKey, "action", value)
	}
	if value := strings.TrimSpace(meta.HighestCategory); value != "" {
		pipe.HSetNX(ctx, metaKey, "highest_category", value)
	}
	if value := strings.TrimSpace(meta.MatchedKeyword); value != "" {
		pipe.HSetNX(ctx, metaKey, "matched_keyword", value)
	}
	if value := strings.TrimSpace(meta.Model); value != "" {
		pipe.HSetNX(ctx, metaKey, "model", value)
	}
	if value := strings.TrimSpace(meta.GroupName); value != "" {
		pipe.HSetNX(ctx, metaKey, "group_name", value)
	}
	if value := strings.TrimSpace(meta.UserEmail); value != "" {
		pipe.HSetNX(ctx, metaKey, "user_email", value)
	}
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
			slog.Warn("content_moderation.flagged_hash_hit_metric_failed", "input_hash", inputHash, "error", err)
		}
		return true, nil
	}
	return false, nil
}

// recordFlaggedInputHashHit 按业务日分桶累加命中次数：Hash field=业务日(YYYYMMDD)、值=当日次数。
// 旧实现每次命中 ZAdd 一个纳秒唯一成员，30 天窗口内成员基数随重放无上限（违反有界 key 规则）；
// 改为 HINCRBY 后单 key 的 field 数 ≤ key 存活天数(TTL 31 天 → ≤32)，天然有界，且保留精确次数。
// 日界按业务时区算（读写同口径），过期 field 靠整 key TTL 回收 + 读取仅取窗口内 field。
func (c *contentModerationHashCache) recordFlaggedInputHashHit(ctx context.Context, inputHash string) error {
	// 时钟源取 Redis 服务器时间（与读取窗口同源，保证读写日界口径一致，也让测试可用模拟时钟推进），
	// 再经 flaggedHashHitDay 转业务时区算日界。
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return err
	}
	key := contentModerationFlaggedHashHitCountKeyPrefix + inputHash
	pipe := c.rdb.Pipeline()
	pipe.HIncrBy(ctx, key, flaggedHashHitDay(now), 1)
	pipe.Expire(ctx, key, contentModerationFlaggedHashHitTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// flaggedHashHitDay 把绝对时刻映射到业务时区的日界字符串（YYYYMMDD），作为命中计数 Hash 的 field。
func flaggedHashHitDay(t time.Time) string {
	return timezone.StartOfDay(t).Format(flaggedHashHitDayLayout)
}

// flaggedHashHitWindowSum 汇总 [now-windowDays, now] 业务日窗口内的命中次数。
// 命令已在调用方的 pipeline 中排队，这里只把 HMGET 结果按天求和。
func flaggedHashHitWindowFields(now time.Time, windowDays int) []string {
	fields := make([]string, 0, windowDays+1)
	start := timezone.StartOfDay(now.AddDate(0, 0, -windowDays))
	for d := 0; d <= windowDays; d++ {
		fields = append(fields, flaggedHashHitDay(start.AddDate(0, 0, d)))
	}
	return fields
}

func sumRedisIntSlice(values []any) int64 {
	var total int64
	for _, v := range values {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				total += n
			}
		}
	}
	return total
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
	sortOrder := filter.Pagination.NormalizedSortOrder(pagination.SortOrderDesc)
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, nil, err
	}

	// created_at 排序：走 created-ZSet 游标分页，只取当页 pageSize 个 hash id 再拉详情，
	// 每页成本 O(pageSize) 而非 O(总量)。hits_7d/hits_30d 排序需要全量命中数才能排序，
	// 无法靠单一有序索引分页，保留全量路径（admin 低频端点）。
	switch strings.ToLower(strings.TrimSpace(filter.Pagination.SortBy)) {
	case service.ContentModerationHashSortHits7D, service.ContentModerationHashSortHits30D:
		return c.listFlaggedInputHashesFullScan(ctx, now, page, pageSize, filter.Pagination.SortBy, sortOrder)
	default:
		return c.listFlaggedInputHashesByCreated(ctx, now, page, pageSize, sortOrder)
	}
}

// listFlaggedInputHashesByCreated 用 created-ZSet 分页：ZCard 取总数，ZRange/ZRevRange 取当页 hash id，
// 仅对当页拉详情。prune 已在入口清理过期项，故此处 ZSet 与实际存活基本一致；batch 二次剔除作兜底。
func (c *contentModerationHashCache) listFlaggedInputHashesByCreated(ctx context.Context, now time.Time, page, pageSize int, sortOrder string) ([]service.ContentModerationHashItem, *pagination.PaginationResult, error) {
	total, err := c.rdb.ZCard(ctx, contentModerationFlaggedHashCreatedZSetKey).Result()
	if err != nil {
		return nil, nil, err
	}
	start := int64(page-1) * int64(pageSize)
	stop := start + int64(pageSize) - 1
	var pageHashes []string
	if start < total {
		if sortOrder == pagination.SortOrderAsc {
			pageHashes, err = c.rdb.ZRange(ctx, contentModerationFlaggedHashCreatedZSetKey, start, stop).Result()
		} else {
			pageHashes, err = c.rdb.ZRevRange(ctx, contentModerationFlaggedHashCreatedZSetKey, start, stop).Result()
		}
		if err != nil {
			return nil, nil, err
		}
	}
	items, expired, err := c.flaggedHashItemsBatch(ctx, pageHashes, now)
	if err != nil {
		return nil, nil, err
	}
	if _, err := c.deleteFlaggedInputHashesPipelined(ctx, expired); err != nil {
		return nil, nil, err
	}
	// batch 结果顺序可能因分片/剔除与 ZSet 顺序错位，按 created_at 复排以保证页内稳定顺序。
	sortFlaggedHashItems(items, service.ContentModerationHashSortCreatedAt, sortOrder)
	return items, buildFlaggedHashPagination(total, page, pageSize), nil
}

// listFlaggedInputHashesFullScan 全量加载后按命中数排序分页（hits 排序专用）。
func (c *contentModerationHashCache) listFlaggedInputHashesFullScan(ctx context.Context, now time.Time, page, pageSize int, sortBy, sortOrder string) ([]service.ContentModerationHashItem, *pagination.PaginationResult, error) {
	hashes, err := c.rdb.ZRange(ctx, contentModerationFlaggedHashCreatedZSetKey, 0, -1).Result()
	if err != nil {
		return nil, nil, err
	}
	items, expired, err := c.flaggedHashItemsBatch(ctx, hashes, now)
	if err != nil {
		return nil, nil, err
	}
	if _, err := c.deleteFlaggedInputHashesPipelined(ctx, expired); err != nil {
		return nil, nil, err
	}
	sortFlaggedHashItems(items, sortBy, sortOrder)
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], buildFlaggedHashPagination(total, page, pageSize), nil
}

func buildFlaggedHashPagination(total int64, page, pageSize int) *pagination.PaginationResult {
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}
}

// flaggedHashItemsBatch 一次性批量取多个 flagged hash 的展示项：按分片走 pipeline，每 hash 发
// HMGet(元数据) + ZScore(创建时间兜底) + 两个 ZCount(近 7/30 天命中)，消除逐 hash 的 N+1 往返。
// 返回构建好的 items 与已过期需删除的 hash 列表（由调用方批量删除），与原逐条实现语义一致：
// created_at 缺失回退 ZScore、再回退 now；expires 缺失按 TTL 顺延；已过期项排除并交由调用方清理。
func (c *contentModerationHashCache) flaggedHashItemsBatch(ctx context.Context, hashes []string, now time.Time) ([]service.ContentModerationHashItem, []string, error) {
	items := make([]service.ContentModerationHashItem, 0, len(hashes))
	var toDelete []string
	fields30d := flaggedHashHitWindowFields(now, 30)
	hit7dCutoff := len(fields30d) - 8 // 最近 8 个 field（含今天，覆盖 7×24h 窗口）计入 7d
	if hit7dCutoff < 0 {
		hit7dCutoff = 0
	}

	for start := 0; start < len(hashes); start += flaggedHashPipelineChunkSize {
		end := min(start+flaggedHashPipelineChunkSize, len(hashes))
		chunk := hashes[start:end]

		// 命中计数改按业务日分桶 Hash：取近 30 个业务日的 field（7d 为其尾部子集），单次 HMGET 拉回，
		// 应用层分别对 7d/30d 窗口求和，替代旧的两次 ZCount。
		pipe := c.rdb.Pipeline()
		type hashCmds struct {
			inputHash string
			meta      *redis.SliceCmd
			created   *redis.FloatCmd
			hits      *redis.SliceCmd
		}
		cmds := make([]hashCmds, 0, len(chunk))
		for _, inputHash := range chunk {
			inputHash = strings.TrimSpace(inputHash)
			if inputHash == "" {
				continue
			}
			cmds = append(cmds, hashCmds{
				inputHash: inputHash,
				meta: pipe.HMGet(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash,
					"created_at", "expires_at", "excerpt", "action", "highest_category",
					"matched_keyword", "model", "group_name", "user_email"),
				created: pipe.ZScore(ctx, contentModerationFlaggedHashCreatedZSetKey, inputHash),
				hits:    pipe.HMGet(ctx, contentModerationFlaggedHashHitCountKeyPrefix+inputHash, fields30d...),
			})
		}
		if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
			return nil, nil, err
		}

		for i := range cmds {
			fields, err := cmds[i].meta.Result()
			if err != nil && err != redis.Nil {
				return nil, nil, err
			}
			createdUnix := redisInt64(fields[0])
			if createdUnix <= 0 {
				if score, scoreErr := cmds[i].created.Result(); scoreErr == nil {
					createdUnix = int64(score)
				} else if scoreErr != redis.Nil {
					return nil, nil, scoreErr
				}
			}
			if createdUnix <= 0 {
				createdUnix = now.Unix()
			}
			expiresUnix := redisInt64(fields[1])
			if expiresUnix <= 0 {
				expiresUnix = createdUnix + int64(contentModerationFlaggedHashTTL/time.Second)
			}
			expiresAt := time.Unix(expiresUnix, 0)
			if !expiresAt.After(now) {
				toDelete = append(toDelete, cmds[i].inputHash)
				continue
			}
			hitValues, err := cmds[i].hits.Result()
			if err != nil && err != redis.Nil {
				return nil, nil, err
			}
			hit30d := sumRedisIntSlice(hitValues)
			var hit7d int64
			if len(hitValues) >= hit7dCutoff {
				hit7d = sumRedisIntSlice(hitValues[hit7dCutoff:])
			}
			items = append(items, service.ContentModerationHashItem{
				InputHash:       cmds[i].inputHash,
				InputExcerpt:    redisString(fields[2]),
				Action:          redisString(fields[3]),
				HighestCategory: redisString(fields[4]),
				MatchedKeyword:  redisString(fields[5]),
				Model:           redisString(fields[6]),
				GroupName:       redisString(fields[7]),
				UserEmail:       redisString(fields[8]),
				CreatedAt:       time.Unix(createdUnix, 0),
				ExpiresAt:       expiresAt,
				HitCount7D:      hit7d,
				HitCount30D:     hit30d,
			})
		}
	}
	return items, toDelete, nil
}

func sortFlaggedHashItems(items []service.ContentModerationHashItem, sortBy string, sortOrder string) {
	desc := sortOrder != pagination.SortOrderAsc
	sort.SliceStable(items, func(i, j int) bool {
		cmp := 0
		switch strings.ToLower(strings.TrimSpace(sortBy)) {
		case service.ContentModerationHashSortHits7D:
			if items[i].HitCount7D != items[j].HitCount7D {
				cmp = compareInt64(items[i].HitCount7D, items[j].HitCount7D)
				break
			}
		case service.ContentModerationHashSortHits30D:
			if items[i].HitCount30D != items[j].HitCount30D {
				cmp = compareInt64(items[i].HitCount30D, items[j].HitCount30D)
				break
			}
		}
		if cmp == 0 {
			cmp = compareTime(items[i].CreatedAt, items[j].CreatedAt)
		}
		if cmp == 0 {
			cmp = strings.Compare(items[i].InputHash, items[j].InputHash)
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareTime(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}

func (c *contentModerationHashCache) DeleteFlaggedInputHash(ctx context.Context, inputHash string) (bool, error) {
	inputHash = strings.TrimSpace(inputHash)
	if c == nil || c.rdb == nil || inputHash == "" {
		return false, nil
	}
	exists, err := c.rdb.Exists(ctx, contentModerationFlaggedHashKeyPrefix+inputHash).Result()
	if err != nil {
		return false, err
	}
	pipe := c.rdb.Pipeline()
	pipe.SRem(ctx, contentModerationFlaggedHashSetKey, inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashHitsKeyPrefix+inputHash)
	pipe.Del(ctx, contentModerationFlaggedHashHitCountKeyPrefix+inputHash)
	pipe.ZRem(ctx, contentModerationFlaggedHashCreatedZSetKey, inputHash)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (c *contentModerationHashCache) DeleteFlaggedInputHashes(ctx context.Context, inputHashes []string) (int64, error) {
	if c == nil || c.rdb == nil || len(inputHashes) == 0 {
		return 0, nil
	}
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(inputHashes))
	for _, inputHash := range inputHashes {
		inputHash = strings.TrimSpace(inputHash)
		if inputHash == "" {
			continue
		}
		if _, ok := seen[inputHash]; ok {
			continue
		}
		seen[inputHash] = struct{}{}
		unique = append(unique, inputHash)
	}
	return c.deleteFlaggedInputHashesPipelined(ctx, unique)
}

// deleteFlaggedInputHashesPipelined 批量删除多个 flagged hash 的全部关联 key（内容/元数据/命中统计/
// 创建时间索引），并顺手清理遗留 SET 成员；按 flaggedHashPipelineChunkSize 分片走单次 pipeline，
// 返回实际存在并删除的 hash 条数。替代逐 hash 多命令往返的 N+1。
func (c *contentModerationHashCache) deleteFlaggedInputHashesPipelined(ctx context.Context, inputHashes []string) (int64, error) {
	if c == nil || c.rdb == nil || len(inputHashes) == 0 {
		return 0, nil
	}
	var deleted int64
	for start := 0; start < len(inputHashes); start += flaggedHashPipelineChunkSize {
		end := start + flaggedHashPipelineChunkSize
		if end > len(inputHashes) {
			end = len(inputHashes)
		}
		chunk := inputHashes[start:end]
		pipe := c.rdb.Pipeline()
		existsCmds := make([]*redis.IntCmd, len(chunk))
		for i, inputHash := range chunk {
			existsCmds[i] = pipe.Exists(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
			pipe.SRem(ctx, contentModerationFlaggedHashSetKey, inputHash)
			pipe.Del(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
			pipe.Del(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash)
			pipe.Del(ctx, contentModerationFlaggedHashHitsKeyPrefix+inputHash)
			pipe.Del(ctx, contentModerationFlaggedHashHitCountKeyPrefix+inputHash)
			pipe.ZRem(ctx, contentModerationFlaggedHashCreatedZSetKey, inputHash)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return deleted, err
		}
		for _, exists := range existsCmds {
			if exists.Val() > 0 {
				deleted++
			}
		}
	}
	return deleted, nil
}

func (c *contentModerationHashCache) ClearFlaggedInputHashes(ctx context.Context) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	hashes, err := c.rdb.ZRange(ctx, contentModerationFlaggedHashCreatedZSetKey, 0, -1).Result()
	if err != nil {
		return 0, err
	}
	if len(hashes) == 0 {
		_, _ = c.rdb.Del(ctx, contentModerationFlaggedHashSetKey).Result()
		return 0, nil
	}
	deleted, err := c.deleteFlaggedInputHashesPipelined(ctx, hashes)
	if err != nil {
		return 0, err
	}
	_, _ = c.rdb.Del(ctx, contentModerationFlaggedHashSetKey).Result()
	return deleted, nil
}

func (c *contentModerationHashCache) CountFlaggedInputHashes(ctx context.Context) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	if err := c.pruneExpiredFlaggedHashes(ctx); err != nil {
		return 0, err
	}
	return c.rdb.ZCard(ctx, contentModerationFlaggedHashCreatedZSetKey).Result()
}

func (c *contentModerationHashCache) pruneExpiredFlaggedHashes(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return err
	}
	hashes, err := c.rdb.ZRange(ctx, contentModerationFlaggedHashCreatedZSetKey, 0, -1).Result()
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		_, _ = c.rdb.Del(ctx, contentModerationFlaggedHashSetKey).Result()
		return nil
	}

	// 批量探测：一次 pipeline（按分片）拿到每个 hash 的存活标记(Exists)与元数据过期时刻(HGet)，
	// 消除逐 hash 往返；待删项收集后再一次批量删除。
	var toDelete []string
	for start := 0; start < len(hashes); start += flaggedHashPipelineChunkSize {
		end := start + flaggedHashPipelineChunkSize
		if end > len(hashes) {
			end = len(hashes)
		}
		chunk := hashes[start:end]
		pipe := c.rdb.Pipeline()
		existsCmds := make([]*redis.IntCmd, len(chunk))
		expiresCmds := make([]*redis.StringCmd, len(chunk))
		for i, inputHash := range chunk {
			existsCmds[i] = pipe.Exists(ctx, contentModerationFlaggedHashKeyPrefix+inputHash)
			expiresCmds[i] = pipe.HGet(ctx, contentModerationFlaggedHashMetaKeyPrefix+inputHash, "expires_at")
		}
		if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
			return err
		}
		for i, inputHash := range chunk {
			exists, err := existsCmds[i].Result()
			if err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			expiresUnix, err := expiresCmds[i].Int64()
			if err != nil && err != redis.Nil {
				return err
			}
			if expiresUnix <= 0 || expiresUnix <= now.Unix() {
				toDelete = append(toDelete, inputHash)
			}
		}
	}
	if _, err := c.deleteFlaggedInputHashesPipelined(ctx, toDelete); err != nil {
		return err
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
