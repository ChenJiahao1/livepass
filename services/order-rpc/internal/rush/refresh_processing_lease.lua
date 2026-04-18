-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: processing ttl seconds

local ttl = redis.call("TTL", KEYS[1])
if ttl <= 0 then
    return ttl
end

local state = redis.call("HGET", KEYS[1], "state")
if state ~= "PROCESSING" then
    return 0
end

local processingTTL = tonumber(ARGV[2]) or 0
if processingTTL > 0 then
    redis.call("EXPIRE", KEYS[1], processingTTL)
end

return 1
