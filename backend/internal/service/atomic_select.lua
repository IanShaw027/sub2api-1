-- atomic_select_and_acquire.lua
-- 原子化账号选择和槽位占用
-- 输入参数:
--   KEYS: 无
--   ARGV[1]: 候选账号数量
--   ARGV[2+]: 每个账号的信息 (id, priority, max_concurrency) 三个一组
--   ARGV[last-1]: 请求ID
--   ARGV[last]: 槽位超时时间(秒)
-- 返回值:
--   {accountID, currentConcurrency} 成功选中的账号
--   {0, 0} 所有账号都已满载

local num_candidates = tonumber(ARGV[1])
local request_id = ARGV[#ARGV - 1]
local timeout = tonumber(ARGV[#ARGV])

-- 解析候选账号并计算评分
local candidates = {}
for i = 1, num_candidates do
    local base_idx = 2 + (i - 1) * 3
    local account_id = tonumber(ARGV[base_idx])
    local priority = tonumber(ARGV[base_idx + 1])
    local max_concurrency = tonumber(ARGV[base_idx + 2])

    local current = tonumber(redis.call('HGET', 'account_concurrency', tostring(account_id)) or 0)
    local load_rate = current / max_concurrency

    -- 评分算法: Priority*0.5 + LoadRate*0.3 + Random*0.2
    local score = priority * 0.5 + load_rate * 0.3 + math.random() * 0.2

    table.insert(candidates, {
        id = account_id,
        score = score,
        current = current,
        max_concurrency = max_concurrency
    })
end

-- 按评分排序（评分越小越优先）
table.sort(candidates, function(a, b) return a.score < b.score end)

-- 尝试原子占位
for _, account in ipairs(candidates) do
    if account.current < account.max_concurrency then
        -- 原子递增并发计数
        redis.call('HINCRBY', 'account_concurrency', tostring(account.id), 1)

        -- 设置槽位标记（带TTL自动过期）
        local slot_key = 'slot:' .. tostring(account.id) .. ':' .. request_id
        redis.call('SETEX', slot_key, timeout, '1')

        -- 返回选中的账号ID和当前并发数
        return {account.id, account.current + 1}
    end
end

-- 所有账号都已满载
return {0, 0}
