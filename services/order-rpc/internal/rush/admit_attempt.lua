-- KEYS:
-- 1: attempt record key(hash)
-- 2: user active key(string)
-- 3: user inflight key(string)
-- 4: quota available key(string)
-- 5: order progress index(zset)
-- 6: user fingerprint index(hash)
-- 7..(6+viewer_count): viewer active keys(string)
-- remaining: viewer inflight keys(string)
--
-- ARGV:
-- 1: order_number
-- 2: user_id
-- 3: program_id
-- 4: show_time_id
-- 5: ticket_category_id
-- 6: ticket_count
-- 7: generation
-- 8: token_fingerprint
-- 9: sale_window_end_at(unix ms)
-- 10: commit_cutoff_at(unix ms)
-- 11: user_deadline_at(unix ms)
-- 12: show_end_at(unix ms)
-- 13: now(unix ms)
-- 14: inflight ttl seconds
-- 15: attempt ttl seconds
-- 16: viewer ids csv
-- 17: viewer_count

local orderNo = ARGV[1]
local userID = ARGV[2]
local programID = ARGV[3]
local showTimeID = ARGV[4]
local ticketCategoryID = ARGV[5]
local ticketCount = tonumber(ARGV[6]) or 0
local generation = ARGV[7]
local tokenFingerprint = ARGV[8]
local saleWindowEndAt = ARGV[9]
local commitCutoffAt = ARGV[10]
local userDeadlineAt = ARGV[11]
local showEndAt = ARGV[12]
local nowUnixMs = ARGV[13]
local inFlightTTL = tonumber(ARGV[14]) or 0
local attemptTTL = tonumber(ARGV[15]) or 0
local viewerIDsCSV = ARGV[16] or ""
local viewerCount = tonumber(ARGV[17]) or 0
local viewerActiveStart = 7
local viewerActiveEnd = viewerActiveStart + viewerCount - 1
local viewerInflightStart = viewerActiveEnd + 1

if tokenFingerprint ~= "" then
    local reusedOrderNo = redis.call("HGET", KEYS[6], tokenFingerprint)
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
    "generation", generation,
    "sale_window_end_at", saleWindowEndAt,
    "token_fingerprint", tokenFingerprint,
    "state", "PENDING_PUBLISH",
    "reason_code", "",
    "commit_cutoff_at", commitCutoffAt,
    "user_deadline_at", userDeadlineAt,
    "show_end_at", showEndAt,
    "processing_epoch", 0,
    "db_probe_attempts", 0,
    "created_at", nowUnixMs,
    "last_transition_at", nowUnixMs
)
if attemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], attemptTTL)
end

if inFlightTTL > 0 then
    redis.call("SETEX", KEYS[3], inFlightTTL, orderNo)
    for idx = viewerInflightStart, #KEYS do
        redis.call("SETEX", KEYS[idx], inFlightTTL, orderNo)
    end
end

if tokenFingerprint ~= "" then
    redis.call("HSET", KEYS[6], tokenFingerprint, orderNo)
    if attemptTTL > 0 then
        redis.call("EXPIRE", KEYS[6], attemptTTL)
    end
end

redis.call("ZADD", KEYS[5], tonumber(nowUnixMs), orderNo)

return {1, orderNo, 0}
