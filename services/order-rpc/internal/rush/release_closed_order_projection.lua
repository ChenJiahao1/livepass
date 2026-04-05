-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "COMMITTED" then
    return 0
end

redis.call("HSET", KEYS[1],
    "state", "RELEASED",
    "reason_code", "CLOSED_ORDER_RELEASED",
    "last_transition_at", ARGV[1]
)

return 1
