-- KEYS:
-- 1: attempt record key(hash)
-- 2: user inflight key(string)
-- 3: user fingerprint index key(hash)
-- 4..n: viewer inflight keys(string)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: token_fingerprint

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state ~= "COMMITTED" then
    redis.call("HSET", KEYS[1],
        "state", "COMMITTED",
        "reason_code", "ORDER_COMMITTED",
        "last_transition_at", ARGV[1]
    )
end

redis.call("DEL", KEYS[2])
for idx = 4, #KEYS do
    redis.call("DEL", KEYS[idx])
end

if ARGV[2] and ARGV[2] ~= "" then
    redis.call("HDEL", KEYS[3], ARGV[2])
end

return 1
