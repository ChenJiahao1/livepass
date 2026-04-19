-- KEYS:
-- 1: attempt record key(hash)
-- 2: user active hash key(field=user_id,value=order_number)
-- 3: user inflight hash key(field=user_id,value=order_number)
-- 4: viewer active hash key(field=viewer_id,value=order_number)
-- 5: viewer inflight hash key(field=viewer_id,value=order_number)
--
-- ARGV:
-- 1: user_id
-- 2: viewer ids csv
-- 3: now(unix ms)
-- 4: active ttl seconds
-- 5: final attempt ttl seconds
-- 6: order_number

local function csv_values(csv)
    local values = {}
    for value in string.gmatch(csv or "", "([^,]+)") do
        table.insert(values, value)
    end
    return values
end

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
if state ~= "PROCESSING" then
    return "lost_ownership"
end

local nowUnixMs = ARGV[3]
local activeTTL = tonumber(ARGV[4]) or 0
local finalAttemptTTL = tonumber(ARGV[5]) or 0
local orderNo = ARGV[6]

redis.call("HSET", KEYS[1],
    "state", "SUCCESS",
    "reason_code", "ORDER_COMMITTED",
    "finished_at", nowUnixMs,
    "last_transition_at", nowUnixMs
)

redis.call("HSET", KEYS[2], ARGV[1], orderNo)
for _, viewerID in ipairs(csv_values(ARGV[2])) do
    redis.call("HSET", KEYS[4], viewerID, orderNo)
end
redis.call("HDEL", KEYS[3], ARGV[1])
for _, viewerID in ipairs(csv_values(ARGV[2])) do
    redis.call("HDEL", KEYS[5], viewerID)
end

if activeTTL > 0 then
    redis.call("EXPIRE", KEYS[2], activeTTL)
    redis.call("EXPIRE", KEYS[4], activeTTL)
end
if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return "transitioned"
