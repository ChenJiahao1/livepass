-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "SUCCESS" or state == "FAILED" then
    return 0
end

local publishAttempts = tonumber(redis.call("HGET", KEYS[1], "publish_attempts") or "0") + 1
if state ~= "ACCEPTED" then
    redis.call("HSET", KEYS[1],
        "state", "ACCEPTED",
        "publish_attempts", publishAttempts,
        "last_transition_at", ARGV[1]
    )
    return 1
end

redis.call("HSET", KEYS[1], "publish_attempts", publishAttempts)

return 1
