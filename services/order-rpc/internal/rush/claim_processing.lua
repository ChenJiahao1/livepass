-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: processing ttl seconds

if redis.call("EXISTS", KEYS[1]) == 0 then
    return {-1, 0}
end

local state = redis.call("HGET", KEYS[1], "state")
if state ~= "ACCEPTED" then
    local currentEpoch = tonumber(redis.call("HGET", KEYS[1], "processing_epoch") or "0")
    return {0, currentEpoch}
end

local nextEpoch = tonumber(redis.call("HGET", KEYS[1], "processing_epoch") or "0") + 1
redis.call("HSET", KEYS[1],
    "state", "PROCESSING",
    "processing_epoch", nextEpoch,
    "processing_started_at", ARGV[1],
    "last_transition_at", ARGV[1]
)
local processingTTL = tonumber(ARGV[2]) or 0
if processingTTL > 0 then
    redis.call("EXPIRE", KEYS[1], processingTTL)
end

return {1, nextEpoch}
