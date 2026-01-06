package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisLeaderUnlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

var redisLeaderRenewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

type RedisLeaderLockOptions struct {
	Enabled bool
	Redis   *redis.Client

	Key string
	TTL time.Duration

	LogPrefix string

	WarnNoRedisOnce *sync.Once
	OnSkip          func()

	LogAcquired bool
	LogReleased bool

	MinRenewInterval time.Duration
}

func TryAcquireRedisLeaderLock(ctx context.Context, opts RedisLeaderLockOptions) (func(), bool) {
	if !opts.Enabled {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	key := strings.TrimSpace(opts.Key)
	if key == "" {
		return nil, true
	}

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	if opts.Redis == nil {
		if opts.WarnNoRedisOnce != nil {
			opts.WarnNoRedisOnce.Do(func() {
				log.Printf("%s distributed lock enabled but redis client is nil; proceeding without leader lock (key=%q)", opts.LogPrefix, key)
			})
		}
		return nil, true
	}

	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	token := generateLeaderLockToken()
	ok, err := opts.Redis.SetNX(lockCtx, key, token, ttl).Result()
	if err != nil {
		log.Printf("%s failed to acquire leader lock (key=%q): %v", opts.LogPrefix, key, err)
		return nil, false
	}
	if !ok {
		if opts.OnSkip != nil {
			opts.OnSkip()
		}
		return nil, false
	}

	if opts.LogAcquired {
		log.Printf("%s acquired leader lock (key=%q ttl=%s token=%s)", opts.LogPrefix, key, ttl, shortenLockToken(token))
	}

	renewEvery := ttl / 2
	minRenew := opts.MinRenewInterval
	if minRenew <= 0 {
		minRenew = 10 * time.Second
	}
	if renewEvery < minRenew {
		renewEvery = minRenew
	}
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = 1
	}

	renewCtx, cancelRenew := context.WithCancel(context.Background())
	renewDone := make(chan struct{})
	go func() {
		defer close(renewDone)

		ticker := time.NewTicker(renewEvery)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				ctxRenew, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				res, err := redisLeaderRenewScript.Run(ctxRenew, opts.Redis, []string{key}, token, ttlMillis).Int()
				cancel()

				if err != nil {
					log.Printf("%s leader lock renewal failed (key=%q token=%s): %v", opts.LogPrefix, key, shortenLockToken(token), err)
					continue
				}
				if res == 0 {
					log.Printf("%s leader lock no longer owned; stop renewing (key=%q token=%s)", opts.LogPrefix, key, shortenLockToken(token))
					return
				}
			}
		}
	}()

	release := func() {
		cancelRenew()

		select {
		case <-renewDone:
		case <-time.After(2 * time.Second):
			log.Printf("%s leader lock renewal goroutine did not stop in time (key=%q)", opts.LogPrefix, key)
		}

		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if _, err := redisLeaderUnlockScript.Run(releaseCtx, opts.Redis, []string{key}, token).Int(); err != nil {
			log.Printf("%s failed to release leader lock (key=%q token=%s): %v", opts.LogPrefix, key, shortenLockToken(token), err)
			return
		}

		if opts.LogReleased {
			log.Printf("%s released leader lock (key=%q token=%s)", opts.LogPrefix, key, shortenLockToken(token))
		}
	}

	return release, true
}

func generateLeaderLockToken() string {
	host, _ := os.Hostname()
	pid := os.Getpid()

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s:%d:%d", host, pid, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s:%d:%s", host, pid, hex.EncodeToString(buf))
}

func shortenLockToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	const maxLen = 10
	if len(token) <= maxLen {
		return token
	}
	return token[:maxLen]
}
