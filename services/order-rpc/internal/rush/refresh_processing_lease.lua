-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: processing_epoch
-- 2: now(unix ms)
-- 3: processing ttl seconds

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state ~= "PROCESSING" then
    return 0
end

local currentEpoch = tonumber(redis.call("HGET", KEYS[1], "processing_epoch") or "0")
local expectedEpoch = tonumber(ARGV[1]) or 0
if expectedEpoch <= 0 or currentEpoch ~= expectedEpoch then
    return 0
end

local processingTTL = tonumber(ARGV[3]) or 0
if processingTTL > 0 then
    redis.call("EXPIRE", KEYS[1], processingTTL)
end

return 1
