-- KEYS:
-- 1: attempt record key(hash)
-- 2: user active key(string)
-- 3: user inflight key(string)
-- 4: quota available key(string)
-- 5: seat occupied key(set)
-- 6: user fingerprint index key(hash)
-- 7..(6+viewer_count): viewer active keys(string)
-- remaining: viewer inflight keys(string)
--
-- ARGV:
-- 1: reason_code
-- 2: ticket_count
-- 3: now(unix ms)
-- 4: final attempt ttl seconds
-- 5: token_fingerprint
-- 6: viewer_count
-- 7: expected processing epoch

if redis.call("EXISTS", KEYS[1]) == 0 then
    return "state_missing"
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "FAILED" then
    return "already_failed"
end
if state == "SUCCESS" then
    return "already_succeeded"
end
if state ~= "PROCESSING" then
    return "lost_ownership"
end

local currentEpoch = tonumber(redis.call("HGET", KEYS[1], "processing_epoch") or "0")
local expectedEpoch = tonumber(ARGV[7]) or 0
if expectedEpoch <= 0 or currentEpoch ~= expectedEpoch then
    return "lost_ownership"
end

local viewerCount = tonumber(ARGV[6]) or 0
local viewerActiveStart = 7
local viewerActiveEnd = viewerActiveStart + viewerCount - 1
local viewerInflightStart = viewerActiveEnd + 1

redis.call("HSET", KEYS[1],
    "state", "FAILED",
    "reason_code", ARGV[1],
    "finished_at", ARGV[3],
    "last_transition_at", ARGV[3]
)

local ticketCount = tonumber(ARGV[2]) or 0
if ticketCount > 0 then
    redis.call("INCRBY", KEYS[4], ticketCount)
end

redis.call("DEL", KEYS[2])
redis.call("DEL", KEYS[3])
redis.call("DEL", KEYS[5])
for idx = viewerActiveStart, viewerActiveEnd do
    redis.call("DEL", KEYS[idx])
end
for idx = viewerInflightStart, #KEYS do
    redis.call("DEL", KEYS[idx])
end

local finalAttemptTTL = tonumber(ARGV[4]) or 0
if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return "transitioned"
