-- KEYS:
-- 1: attempt record key(hash)
-- 2: user inflight key(string)
-- 3: quota available key(string)
-- 4: user fingerprint index key(hash)
-- 5..n: viewer inflight keys(string)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: ticket_count
-- 3: token_fingerprint

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "RELEASED" then
    return 0
end

redis.call("HSET", KEYS[1],
    "state", "RELEASED",
    "reason_code", "CLOSED_ORDER_RELEASED",
    "last_transition_at", ARGV[1]
)

local ticketCount = tonumber(ARGV[2]) or 0
if ticketCount > 0 then
    redis.call("INCRBY", KEYS[3], ticketCount)
end

redis.call("DEL", KEYS[2])
for idx = 5, #KEYS do
    redis.call("DEL", KEYS[idx])
end

if ARGV[3] and ARGV[3] ~= "" then
    redis.call("HDEL", KEYS[4], ARGV[3])
end

return 1
