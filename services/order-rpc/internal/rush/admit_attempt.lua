-- KEYS:
-- 1: attempt record key(hash)
-- 2: user active key(string)
-- 3: user inflight key(string)
-- 4: quota available key(string)
-- 5: user fingerprint index(hash)
-- 6..(5+viewer_count): viewer active keys(string)
-- remaining: viewer inflight keys(string)
--
-- ARGV:
-- 1: order_number
-- 2: user_id
-- 3: program_id
-- 4: show_time_id
-- 5: ticket_category_id
-- 6: ticket_count
-- 7: token_fingerprint
-- 8: sale_window_end_at(unix ms)
-- 9: show_end_at(unix ms)
-- 10: now(unix ms)
-- 11: inflight ttl seconds
-- 12: accepted attempt ttl seconds
-- 13: viewer ids csv
-- 14: viewer_count

local orderNo = ARGV[1]
local userID = ARGV[2]
local programID = ARGV[3]
local showTimeID = ARGV[4]
local ticketCategoryID = ARGV[5]
local ticketCount = tonumber(ARGV[6]) or 0
local tokenFingerprint = ARGV[7]
local saleWindowEndAt = ARGV[8]
local showEndAt = ARGV[9]
local nowUnixMs = ARGV[10]
local inFlightTTL = tonumber(ARGV[11]) or 0
local acceptedAttemptTTL = tonumber(ARGV[12]) or 0
local viewerIDsCSV = ARGV[13] or ""
local viewerCount = tonumber(ARGV[14]) or 0
local viewerActiveStart = 6
local viewerActiveEnd = viewerActiveStart + viewerCount - 1
local viewerInflightStart = viewerActiveEnd + 1

if tokenFingerprint ~= "" then
    local reusedOrderNo = redis.call("HGET", KEYS[5], tokenFingerprint)
    if reusedOrderNo and reusedOrderNo ~= "" then
        return {2, reusedOrderNo, 0}
    end
end

local activeOrderNo = redis.call("GET", KEYS[2])
if activeOrderNo and activeOrderNo ~= "" then
    return {0, activeOrderNo, 1001}
end

for idx = viewerActiveStart, viewerActiveEnd do
    local viewerActiveOrderNo = redis.call("GET", KEYS[idx])
    if viewerActiveOrderNo and viewerActiveOrderNo ~= "" then
        return {0, viewerActiveOrderNo, 1002}
    end
end

local existingOrderNo = redis.call("GET", KEYS[3])
if existingOrderNo and existingOrderNo ~= "" then
    return {0, existingOrderNo, 1001}
end

for idx = viewerInflightStart, #KEYS do
    local viewerInflightOrderNo = redis.call("GET", KEYS[idx])
    if viewerInflightOrderNo and viewerInflightOrderNo ~= "" then
        return {0, viewerInflightOrderNo, 1002}
    end
end

local quota = tonumber(redis.call("GET", KEYS[4]) or "")
if (not quota) or quota < ticketCount then
    return {0, 0, 1003}
end

redis.call("DECRBY", KEYS[4], ticketCount)

redis.call("HSET", KEYS[1],
    "order_number", orderNo,
    "user_id", userID,
    "program_id", programID,
    "show_time_id", showTimeID,
    "ticket_category_id", ticketCategoryID,
    "viewer_ids", viewerIDsCSV,
    "ticket_count", ticketCount,
    "sale_window_end_at", saleWindowEndAt,
    "token_fingerprint", tokenFingerprint,
    "state", "ACCEPTED",
    "reason_code", "",
    "accepted_at", nowUnixMs,
    "finished_at", 0,
    "publish_attempts", 0,
    "show_end_at", showEndAt,
    "processing_epoch", 0,
    "created_at", nowUnixMs,
    "last_transition_at", nowUnixMs
)
if acceptedAttemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], acceptedAttemptTTL)
end

if inFlightTTL > 0 then
    redis.call("SETEX", KEYS[3], inFlightTTL, orderNo)
    for idx = viewerInflightStart, #KEYS do
        redis.call("SETEX", KEYS[idx], inFlightTTL, orderNo)
    end
end

if tokenFingerprint ~= "" then
    redis.call("HSET", KEYS[5], tokenFingerprint, orderNo)
    if acceptedAttemptTTL > 0 then
        redis.call("EXPIRE", KEYS[5], acceptedAttemptTTL)
    end
end

return {1, orderNo, 0}
