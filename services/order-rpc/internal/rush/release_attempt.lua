-- KEYS:
-- 1: attempt record key(hash)
-- 2: user inflight key(string)
-- 3: quota available key(string)
-- 4: user fingerprint index key(hash)
-- 5..n: viewer inflight keys(string)
--
-- ARGV:
-- 1: reason_code
-- 2: ticket_count
-- 3: now(unix ms)
-- 4: token_fingerprint

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "COMMITTED" then
    return 0
end

if state ~= "RELEASED" then
    redis.call("HSET", KEYS[1],
        "state", "RELEASED",
        "reason_code", ARGV[1],
        "last_transition_at", ARGV[3]
    )
    local ticketCount = tonumber(ARGV[2]) or 0
    if ticketCount > 0 then
        redis.call("INCRBY", KEYS[3], ticketCount)
    end
end

redis.call("DEL", KEYS[2])
for idx = 5, #KEYS do
    redis.call("DEL", KEYS[idx])
end

if ARGV[4] and ARGV[4] ~= "" then
    redis.call("HDEL", KEYS[4], ARGV[4])
end

return 1
