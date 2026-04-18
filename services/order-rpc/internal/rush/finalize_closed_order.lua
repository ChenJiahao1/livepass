-- KEYS:
-- 1: attempt record key(hash)
-- 2: user active key(string)
-- 3: user inflight key(string)
-- 4: quota available key(string)
-- 5..(4+viewer_count): viewer active keys(string)
-- remaining: viewer inflight keys(string)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: ticket_count
-- 3: final attempt ttl seconds
-- 4: viewer_count

if redis.call("EXISTS", KEYS[1]) == 0 then
    return "state_missing"
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "FAILED" then
    return "already_failed"
end
if state ~= "SUCCESS" then
    return "lost_ownership"
end

local viewerCount = tonumber(ARGV[4]) or 0
local viewerActiveStart = 5
local viewerActiveEnd = viewerActiveStart + viewerCount - 1
local viewerInflightStart = viewerActiveEnd + 1

redis.call("HSET", KEYS[1],
    "state", "FAILED",
    "reason_code", "CLOSED_ORDER_RELEASED",
    "finished_at", ARGV[1],
    "last_transition_at", ARGV[1]
)

local ticketCount = tonumber(ARGV[2]) or 0
if ticketCount > 0 then
    redis.call("INCRBY", KEYS[4], ticketCount)
end

redis.call("DEL", KEYS[2])
redis.call("DEL", KEYS[3])
for idx = viewerActiveStart, viewerActiveEnd do
    redis.call("DEL", KEYS[idx])
end
for idx = viewerInflightStart, #KEYS do
    redis.call("DEL", KEYS[idx])
end

local finalAttemptTTL = tonumber(ARGV[3]) or 0
if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return "transitioned"
