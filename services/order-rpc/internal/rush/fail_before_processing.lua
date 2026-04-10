-- KEYS:
-- 1: attempt record key(hash)
-- 2: user inflight key(string)
-- 3: quota available key(string)
-- 4..: viewer inflight keys(string)
--
-- ARGV:
-- 1: reason_code
-- 2: ticket_count
-- 3: now(unix ms)
-- 4: final attempt ttl seconds

if redis.call("EXISTS", KEYS[1]) == 0 then
    return "state_missing"
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "FAILED" then
    return "already_failed"
end
if state == "SUCCESS" or state == "PROCESSING" then
    return "lost_ownership"
end
if state ~= "ACCEPTED" then
    return "state_missing"
end

redis.call("HSET", KEYS[1],
    "state", "FAILED",
    "reason_code", ARGV[1],
    "finished_at", ARGV[3],
    "last_transition_at", ARGV[3]
)

local ticketCount = tonumber(ARGV[2]) or 0
if ticketCount > 0 then
    redis.call("INCRBY", KEYS[3], ticketCount)
end

redis.call("DEL", KEYS[2])
for idx = 4, #KEYS do
    redis.call("DEL", KEYS[idx])
end

local finalAttemptTTL = tonumber(ARGV[4]) or 0
if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return "transitioned"
