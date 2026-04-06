-- KEYS:
-- 1: attempt record key(hash)
-- 2: user inflight key(string)
-- 3: quota available key(string)
-- 4: order progress index(zset)
-- 5: user fingerprint index(hash)
-- 6..n: viewer inflight keys(string)
--
-- ARGV:
-- 1: order_number
-- 2: user_id
-- 3: program_id
-- 4: ticket_category_id
-- 5: ticket_count
-- 6: token_fingerprint
-- 7: commit_cutoff_at(unix ms)
-- 8: user_deadline_at(unix ms)
-- 9: now(unix ms)
-- 10: inflight ttl seconds
-- 11: attempt ttl seconds
-- 12: viewer ids csv

local orderNo = ARGV[1]
local userID = ARGV[2]
local programID = ARGV[3]
local ticketCategoryID = ARGV[4]
local ticketCount = tonumber(ARGV[5]) or 0
local tokenFingerprint = ARGV[6]
local commitCutoffAt = ARGV[7]
local userDeadlineAt = ARGV[8]
local nowUnixMs = ARGV[9]
local inFlightTTL = tonumber(ARGV[10]) or 0
local attemptTTL = tonumber(ARGV[11]) or 0
local viewerIDsCSV = ARGV[12] or ""

if tokenFingerprint ~= "" then
    local reusedOrderNo = redis.call("HGET", KEYS[5], tokenFingerprint)
    if reusedOrderNo and reusedOrderNo ~= "" then
        return {2, reusedOrderNo, 0}
    end
end

local existingOrderNo = redis.call("GET", KEYS[2])
if existingOrderNo and existingOrderNo ~= "" then
    return {0, existingOrderNo, 1001}
end

for idx = 6, #KEYS do
    local viewerInflightOrderNo = redis.call("GET", KEYS[idx])
    if viewerInflightOrderNo and viewerInflightOrderNo ~= "" then
        return {0, viewerInflightOrderNo, 1002}
    end
end

local quota = tonumber(redis.call("GET", KEYS[3]) or "")
if (not quota) or quota < ticketCount then
    return {0, 0, 1003}
end

redis.call("DECRBY", KEYS[3], ticketCount)

redis.call("HSET", KEYS[1],
    "order_number", orderNo,
    "user_id", userID,
    "program_id", programID,
    "ticket_category_id", ticketCategoryID,
    "viewer_ids", viewerIDsCSV,
    "ticket_count", ticketCount,
    "token_fingerprint", tokenFingerprint,
    "state", "PENDING_PUBLISH",
    "reason_code", "",
    "commit_cutoff_at", commitCutoffAt,
    "user_deadline_at", userDeadlineAt,
    "processing_epoch", 0,
    "db_probe_attempts", 0,
    "created_at", nowUnixMs,
    "last_transition_at", nowUnixMs
)
if attemptTTL > 0 then
    redis.call("EXPIRE", KEYS[1], attemptTTL)
end

if inFlightTTL > 0 then
    redis.call("SETEX", KEYS[2], inFlightTTL, orderNo)
    for idx = 6, #KEYS do
        redis.call("SETEX", KEYS[idx], inFlightTTL, orderNo)
    end
end

if tokenFingerprint ~= "" then
    redis.call("HSET", KEYS[5], tokenFingerprint, orderNo)
    if attemptTTL > 0 then
        redis.call("EXPIRE", KEYS[5], attemptTTL)
    end
end

redis.call("ZADD", KEYS[4], tonumber(nowUnixMs), orderNo)

return {1, orderNo, 0}
