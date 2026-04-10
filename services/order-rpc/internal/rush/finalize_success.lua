-- KEYS:
-- 1: attempt record key(hash)
-- 2: user active key(string)
-- 3: user inflight key(string)
-- 4: seat occupied key(set)
-- 5..(4+viewer_count): viewer active keys(string)
-- remaining: viewer inflight keys(string)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: active ttl seconds
-- 3: final attempt ttl seconds
-- 4: seat ids csv
-- 5: viewer_count
-- 6: order_number
-- 7: expected processing epoch

if redis.call("EXISTS", KEYS[1]) == 0 then
    return "state_missing"
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "SUCCESS" then
    return "already_succeeded"
end
if state == "FAILED" then
    return "already_failed"
end
if state ~= "PROCESSING" and state ~= "ACCEPTED" then
    return "lost_ownership"
end

local expectedEpoch = tonumber(ARGV[7]) or 0
if state == "PROCESSING" and expectedEpoch > 0 then
    local currentEpoch = tonumber(redis.call("HGET", KEYS[1], "processing_epoch") or "0")
    if currentEpoch ~= expectedEpoch then
        return "lost_ownership"
    end
end
if state == "ACCEPTED" and expectedEpoch > 0 then
    return "lost_ownership"
end

local nowUnixMs = ARGV[1]
local activeTTL = tonumber(ARGV[2]) or 0
local finalAttemptTTL = tonumber(ARGV[3]) or 0
local seatIDsCSV = ARGV[4] or ""
local viewerCount = tonumber(ARGV[5]) or 0
local orderNo = ARGV[6]
local viewerActiveStart = 5
local viewerActiveEnd = viewerActiveStart + viewerCount - 1
local viewerInflightStart = viewerActiveEnd + 1

redis.call("HSET", KEYS[1],
    "state", "SUCCESS",
    "reason_code", "ORDER_COMMITTED",
    "finished_at", nowUnixMs,
    "last_transition_at", nowUnixMs
)

if activeTTL > 0 then
    redis.call("SETEX", KEYS[2], activeTTL, orderNo)
else
    redis.call("SET", KEYS[2], orderNo)
end
for idx = viewerActiveStart, viewerActiveEnd do
    if activeTTL > 0 then
        redis.call("SETEX", KEYS[idx], activeTTL, orderNo)
    else
        redis.call("SET", KEYS[idx], orderNo)
    end
end

if seatIDsCSV ~= "" then
    for member in string.gmatch(seatIDsCSV, "([^,]+)") do
        if member ~= "" then
            redis.call("SADD", KEYS[4], member)
        end
    end
    if activeTTL > 0 then
        redis.call("EXPIRE", KEYS[4], activeTTL)
    end
end

redis.call("DEL", KEYS[3])
for idx = viewerInflightStart, #KEYS do
    redis.call("DEL", KEYS[idx])
end

if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return "transitioned"
