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

local viewerCount = tonumber(ARGV[6]) or 0
local viewerActiveStart = 7
local viewerActiveEnd = viewerActiveStart + viewerCount - 1
local viewerInflightStart = viewerActiveEnd + 1

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
        redis.call("INCRBY", KEYS[4], ticketCount)
    end
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

if ARGV[5] and ARGV[5] ~= "" then
    redis.call("HDEL", KEYS[6], ARGV[5])
end

local finalAttemptTTL = tonumber(ARGV[4]) or 0
if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return 1
