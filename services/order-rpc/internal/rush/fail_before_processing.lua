-- KEYS:
-- 1: attempt record key(hash)
-- 2: user inflight hash key(field=user_id,value=order_number)
-- 3: viewer inflight hash key(field=viewer_id,value=order_number)
-- 4: quota hash key(field=ticket_category_id,value=available)
--
-- ARGV:
-- 1: reason_code
-- 2: user_id
-- 3: viewer ids csv
-- 4: ticket_category_id
-- 5: ticket_count
-- 6: now(unix ms)
-- 7: final attempt ttl seconds
-- 8: projection ttl seconds
-- 9: inflight ttl seconds

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
if state == "FAILED" then
    return "already_failed"
end
if state == "SUCCESS" or state == "PROCESSING" then
    return "lost_ownership"
end
if state ~= "PENDING" then
    return "state_missing"
end

redis.call("HSET", KEYS[1],
    "state", "FAILED",
    "reason_code", ARGV[1],
    "finished_at", ARGV[6],
    "last_transition_at", ARGV[6]
)

local ticketCount = tonumber(ARGV[5]) or 0
if ticketCount > 0 then
    redis.call("HINCRBY", KEYS[4], ARGV[4], ticketCount)
end

redis.call("HDEL", KEYS[2], ARGV[2])
for _, viewerID in ipairs(csv_values(ARGV[3])) do
    redis.call("HDEL", KEYS[3], viewerID)
end

local projectionTTL = tonumber(ARGV[8]) or 0
if projectionTTL > 0 then
    redis.call("EXPIRE", KEYS[4], projectionTTL)
end
local inflightTTL = tonumber(ARGV[9]) or 0
if inflightTTL > 0 then
    redis.call("EXPIRE", KEYS[2], inflightTTL)
    redis.call("EXPIRE", KEYS[3], inflightTTL)
end
local finalAttemptTTL = tonumber(ARGV[7]) or 0
if finalAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], finalAttemptTTL)
end

return "transitioned"
