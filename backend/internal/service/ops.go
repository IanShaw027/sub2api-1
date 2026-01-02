package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrorLog represents an ops error log item for list queries.
//
// Field naming matches docs/API-运维监控中心2.0.md (L3 根因追踪 - 错误日志列表).
type ErrorLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	Level        string `json:"level,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	APIPath      string `json:"api_path,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	HTTPCode     int    `json:"http_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	DurationMs *int `json:"duration_ms,omitempty"`
	RetryCount *int `json:"retry_count,omitempty"`
	Stream     bool `json:"stream,omitempty"`
}

// ErrorLogFilter describes optional filters and pagination for listing ops error logs.
type ErrorLogFilter struct {
	StartTime *time.Time
	EndTime   *time.Time

	ErrorCode *int
	Provider  string
	AccountID *int64

	Page     int
	PageSize int
}

func (f *ErrorLogFilter) normalize() (page, pageSize int) {
	page = 1
	pageSize = 20
	if f == nil {
		return page, pageSize
	}

	if f.Page > 0 {
		page = f.Page
	}
	if f.PageSize > 0 {
		pageSize = f.PageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

type ErrorLogListResponse struct {
	Errors   []*ErrorLog `json:"errors"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func (s *OpsService) GetErrorLogs(ctx context.Context, filter *ErrorLogFilter) (*ErrorLogListResponse, error) {
	if s == nil || s.repo == nil {
		return &ErrorLogListResponse{
			Errors:   []*ErrorLog{},
			Total:    0,
			Page:     1,
			PageSize: 20,
		}, nil
	}

	page, pageSize := filter.normalize()
	if filter == nil {
		filter = &ErrorLogFilter{}
	}
	filter.Page = page
	filter.PageSize = pageSize

	items, total, err := s.repo.ListErrorLogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*ErrorLog{}
	}

	return &ErrorLogListResponse{
		Errors:   items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

const (
	// ops:metrics:latest 最新指标快照（JSON），短 TTL 用于降低数据库查询压力。
	opsLatestMetricsKey = "ops:metrics:latest"

	// ops:qps:{minute} 每分钟请求计数器（minute 为 Unix minute，即 Unix()/60）。
	opsQPSKeyPrefix = "ops:qps:"
	// ops:tps:{minute} 每分钟 token 计数器（minute 为 Unix minute，即 Unix()/60）。
	opsTPSKeyPrefix = "ops:tps:"

	opsLatestMetricsTTL = 10 * time.Second
	// 计数器只用于 1 分钟窗口计算，保留 2 分钟即可避免 Redis key 无限增长。
	opsCounterTTL = 2 * time.Minute
)

// OpsMetricsCache Redis 缓存层，用于缓存运维监控指标，降低数据库查询压力。
//
// Key 命名规范：
// - ops:metrics:latest - 最新指标快照（JSON，TTL=10s）
// - ops:qps:{minute} - QPS 计数器（按分钟）
// - ops:tps:{minute} - TPS 计数器（按分钟）
//
// 线程安全：
// - go-redis 客户端本身并发安全。
// - 本结构体不维护可变共享状态，天然并发安全。
type OpsMetricsCache struct {
	client *redis.Client
}

// NewOpsMetricsCache 创建运维指标缓存实例。
func NewOpsMetricsCache(client *redis.Client) *OpsMetricsCache {
	return &OpsMetricsCache{client: client}
}

// GetLatestMetrics 获取最新指标（TTL=10s）。
//
// 返回值说明：
// - 若缓存不存在（redis.Nil），返回 (nil, nil) 代表缓存 miss。
// - 其他错误返回 (nil, err)。
func (c *OpsMetricsCache) GetLatestMetrics(ctx context.Context) (*OpsMetrics, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.client == nil {
		return nil, nil
	}

	data, err := c.client.Get(ctx, opsLatestMetricsKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get latest ops metrics: %w", err)
	}

	var metrics OpsMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("unmarshal latest ops metrics: %w", err)
	}
	return &metrics, nil
}

// SetLatestMetrics 设置最新指标（TTL=10s）。
func (c *OpsMetricsCache) SetLatestMetrics(ctx context.Context, metrics *OpsMetrics) error {
	if metrics == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.client == nil {
		return nil
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("marshal latest ops metrics: %w", err)
	}
	return c.client.Set(ctx, opsLatestMetricsKey, data, opsLatestMetricsTTL).Err()
}

func opsMinuteBucket(now time.Time) (unixMinute int64, secondInMinute int64) {
	unix := now.Unix()
	return unix / 60, unix % 60
}

// IncrementQPS 增加 QPS 计数器（按分钟分桶）。
func (c *OpsMetricsCache) IncrementQPS(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.client == nil {
		return nil
	}

	minute, _ := opsMinuteBucket(time.Now())
	key := fmt.Sprintf("%s%d", opsQPSKeyPrefix, minute)

	pipe := c.client.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, opsCounterTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// IncrementTPS 增加 TPS 计数器（按分钟分桶累计 tokens）。
func (c *OpsMetricsCache) IncrementTPS(ctx context.Context, tokens int64) error {
	if tokens <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.client == nil {
		return nil
	}

	minute, _ := opsMinuteBucket(time.Now())
	key := fmt.Sprintf("%s%d", opsTPSKeyPrefix, minute)

	pipe := c.client.TxPipeline()
	pipe.IncrBy(ctx, key, tokens)
	pipe.Expire(ctx, key, opsCounterTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// GetRealtimeQPS 获取实时 QPS（1分钟窗口）。
//
// 由于计数器按分钟分桶，这里使用当前/上一分钟分桶并按 “当前分钟已过去比例” 做简单加权，
// 以近似过去 60 秒窗口的请求总数，再除以 60 得到 QPS。
func (c *OpsMetricsCache) GetRealtimeQPS(ctx context.Context) (float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.client == nil {
		return 0, nil
	}

	minute, secondInMinute := opsMinuteBucket(time.Now())
	curKey := fmt.Sprintf("%s%d", opsQPSKeyPrefix, minute)
	prevKey := fmt.Sprintf("%s%d", opsQPSKeyPrefix, minute-1)

	pipe := c.client.Pipeline()
	curCmd := pipe.Get(ctx, curKey)
	prevCmd := pipe.Get(ctx, prevKey)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("redis get realtime qps: %w", err)
	}

	curCount := int64(0)
	if v, err := curCmd.Int64(); err == nil {
		curCount = v
	} else if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("redis parse realtime qps (cur): %w", err)
	}

	prevCount := int64(0)
	if v, err := prevCmd.Int64(); err == nil {
		prevCount = v
	} else if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("redis parse realtime qps (prev): %w", err)
	}

	weightCur := float64(secondInMinute) / 60.0
	weightPrev := 1.0 - weightCur
	estimatedLastMinute := float64(curCount)*weightCur + float64(prevCount)*weightPrev
	return estimatedLastMinute / 60.0, nil
}

// GetRealtimeTPS 获取实时 TPS（1分钟窗口）。
//
// 逻辑同 GetRealtimeQPS：按分钟分桶的 tokens 计数做窗口近似，再除以 60 得到 TPS。
func (c *OpsMetricsCache) GetRealtimeTPS(ctx context.Context) (float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.client == nil {
		return 0, nil
	}

	minute, secondInMinute := opsMinuteBucket(time.Now())
	curKey := fmt.Sprintf("%s%d", opsTPSKeyPrefix, minute)
	prevKey := fmt.Sprintf("%s%d", opsTPSKeyPrefix, minute-1)

	pipe := c.client.Pipeline()
	curCmd := pipe.Get(ctx, curKey)
	prevCmd := pipe.Get(ctx, prevKey)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("redis get realtime tps: %w", err)
	}

	curCount := int64(0)
	if v, err := curCmd.Int64(); err == nil {
		curCount = v
	} else if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("redis parse realtime tps (cur): %w", err)
	}

	prevCount := int64(0)
	if v, err := prevCmd.Int64(); err == nil {
		prevCount = v
	} else if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("redis parse realtime tps (prev): %w", err)
	}

	weightCur := float64(secondInMinute) / 60.0
	weightPrev := 1.0 - weightCur
	estimatedLastMinute := float64(curCount)*weightCur + float64(prevCount)*weightPrev
	return estimatedLastMinute / 60.0, nil
}
