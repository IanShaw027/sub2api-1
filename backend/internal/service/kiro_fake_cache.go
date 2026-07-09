package service

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

// kiroFakeCache 是 Kiro 假 prompt-cache 的进程内存储，封装 ristretto 提供有界容量 +
// O(1) 摊还淘汰 + 逐条 TTL。替换掉此前的 patrickmn/go-cache(只有 TTL、无淘汰上限，
// 稳态条目 = 到达率×TTL，是随流量增长的无界内存足迹，违反项目 L1 有界规则)。
//
// 接口刻意与 go-cache 对齐(Get/Set/Flush)，调用方无需改动。ristretto 写入是异步的：
// 生产路径「本轮写、下轮读」跨请求间隔远大于写缓冲 flush 延迟，无影响；需要「写后立即读」
// 的测试显式调用 wait()。ristretto 的概率驱逐可能丢弃未过期项，对 fake cache 语义安全
// (命中省 token、丢失只是少省一次，不影响正确性)。
type kiroFakeCache struct {
	cache *ristretto.Cache
}

func newKiroFakeCache(maxEntries int64) *kiroFakeCache {
	if maxEntries <= 0 {
		maxEntries = 1
	}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: maxEntries * 10,
		MaxCost:     maxEntries,
		BufferItems: 64,
	})
	if err != nil {
		return nil
	}
	return &kiroFakeCache{cache: cache}
}

func (c *kiroFakeCache) Get(key string) (any, bool) {
	if c == nil || c.cache == nil {
		return nil, false
	}
	return c.cache.Get(key)
}

// Set 写入一条，cost=1(每条按条数计入 MaxCost)，携带独立 TTL。ttl<=0 时不写(与
// go-cache 的 NoExpiration 语义不同，但 fake cache 所有写入都带正 TTL)。
func (c *kiroFakeCache) Set(key string, value any, ttl time.Duration) {
	if c == nil || c.cache == nil || ttl <= 0 {
		return
	}
	c.cache.SetWithTTL(key, value, 1, ttl)
}

func (c *kiroFakeCache) Flush() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Clear()
}

// wait 阻塞到已提交的异步写入对后续 Get 可见，仅供需要「写后立即读」的测试使用。
func (c *kiroFakeCache) wait() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Wait()
}
