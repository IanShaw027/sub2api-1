package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type creationVideoObservationQueue struct{ rdb *redis.Client }

func NewCreationVideoObservationQueue(rdb *redis.Client) service.CreationVideoObservationQueue {
	return &creationVideoObservationQueue{rdb: rdb}
}

var creationVideoQueueKeys = []string{"{creation-local-video}:jobs", "{creation-local-video}:due", "{creation-local-video}:expires"}

const videoQueueClockAndPrune = `
local tm = redis.call('TIME')
local now = tonumber(tm[1]) * 1000 + math.floor(tonumber(tm[2]) / 1000)
local expired = redis.call('ZRANGEBYSCORE', KEYS[3], '-inf', now)
for _, id in ipairs(expired) do
  redis.call('HDEL', KEYS[1], id)
  redis.call('ZREM', KEYS[2], id)
  redis.call('ZREM', KEYS[3], id)
end
`

var reserveCreationVideoScript = redis.NewScript(videoQueueClockAndPrune + `
if redis.call('ZCARD', KEYS[3]) >= tonumber(ARGV[3]) then return 0 end
if redis.call('HEXISTS', KEYS[1], ARGV[1]) == 1 then return 0 end
local job = cjson.decode(ARGV[2])
job.expires_at = now + tonumber(ARGV[4])
redis.call('HSET', KEYS[1], ARGV[1], cjson.encode(job))
redis.call('ZADD', KEYS[2], job.expires_at, ARGV[1])
redis.call('ZADD', KEYS[3], job.expires_at, ARGV[1])
for _, key in ipairs(KEYS) do redis.call('PEXPIRE', key, ARGV[5]) end
return 1
`)

var bindCreationVideoScript = redis.NewScript(videoQueueClockAndPrune + `
local raw = redis.call('HGET', KEYS[1], ARGV[1])
if not raw then return 0 end
local job = cjson.decode(raw)
if job.task_id and job.task_id ~= '' then
  if job.task_id == ARGV[2] then return 1 else return 0 end
end
job.task_id = ARGV[2]
job.expires_at = now + tonumber(ARGV[3])
job.lease_token = ''
redis.call('HSET', KEYS[1], ARGV[1], cjson.encode(job))
redis.call('ZADD', KEYS[2], now, ARGV[1])
redis.call('ZADD', KEYS[3], job.expires_at, ARGV[1])
for _, key in ipairs(KEYS) do redis.call('PEXPIRE', key, ARGV[4]) end
return 1
`)

var cancelCreationVideoScript = redis.NewScript(`
local raw = redis.call('HGET', KEYS[1], ARGV[1])
if not raw then return 1 end
local job = cjson.decode(raw)
if job.task_id and job.task_id ~= '' then return 0 end
redis.call('HDEL', KEYS[1], ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('ZREM', KEYS[3], ARGV[1])
return 1
`)

var claimCreationVideoScript = redis.NewScript(videoQueueClockAndPrune + `
local ids = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', now, 'LIMIT', 0, 1)
if #ids == 0 then return nil end
local id = ids[1]
local raw = redis.call('HGET', KEYS[1], id)
if not raw then redis.call('ZREM', KEYS[2], id); redis.call('ZREM', KEYS[3], id); return nil end
local job = cjson.decode(raw)
if not job.task_id or job.task_id == '' then return nil end
job.lease_token = ARGV[1]
local updated = cjson.encode(job)
redis.call('HSET', KEYS[1], id, updated)
redis.call('ZADD', KEYS[2], now + tonumber(ARGV[2]), id)
return updated
`)

var finishCreationVideoScript = redis.NewScript(`
local raw = redis.call('HGET', KEYS[1], ARGV[1])
if not raw then return 1 end
local job = cjson.decode(raw)
if job.lease_token ~= ARGV[2] then return 0 end
if ARGV[3] == '1' then
  redis.call('HDEL', KEYS[1], ARGV[1])
  redis.call('ZREM', KEYS[2], ARGV[1])
  redis.call('ZREM', KEYS[3], ARGV[1])
else
  local tm = redis.call('TIME')
  local now = tonumber(tm[1]) * 1000 + math.floor(tonumber(tm[2]) / 1000)
  job.lease_token = ''
  redis.call('HSET', KEYS[1], ARGV[1], cjson.encode(job))
  redis.call('ZADD', KEYS[2], now + tonumber(ARGV[4]), ARGV[1])
end
return 1
`)

func (q *creationVideoObservationQueue) Reserve(ctx context.Context, job *service.CreationVideoObservation, capacity int, ttl time.Duration) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	result, err := reserveCreationVideoScript.Run(ctx, q.rdb, creationVideoQueueKeys, job.ID, data, capacity, ttl.Milliseconds(), (service.CreationVideoObservationTTL + 2*time.Minute).Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return service.ErrCreationVideoObservationFull
	}
	return nil
}

func (q *creationVideoObservationQueue) Bind(ctx context.Context, id, taskID string, ttl time.Duration) error {
	result, err := bindCreationVideoScript.Run(ctx, q.rdb, creationVideoQueueKeys, id, taskID, ttl.Milliseconds(), (ttl + 2*time.Minute).Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return service.ErrCreationVideoObservationMissing
	}
	return nil
}

func (q *creationVideoObservationQueue) CancelReservation(ctx context.Context, id string) error {
	return cancelCreationVideoScript.Run(ctx, q.rdb, creationVideoQueueKeys, id).Err()
}

func (q *creationVideoObservationQueue) Claim(ctx context.Context, owner string, lease time.Duration) (*service.CreationVideoObservation, error) {
	data, err := claimCreationVideoScript.Run(ctx, q.rdb, creationVideoQueueKeys, owner, lease.Milliseconds()).Text()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job service.CreationVideoObservation
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (q *creationVideoObservationQueue) Finish(ctx context.Context, id, owner string, terminal bool, delay time.Duration) error {
	done := "0"
	if terminal {
		done = "1"
	}
	result, err := finishCreationVideoScript.Run(ctx, q.rdb, creationVideoQueueKeys, id, owner, done, delay.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return service.ErrCreationVideoObservationMissing
	}
	return nil
}
