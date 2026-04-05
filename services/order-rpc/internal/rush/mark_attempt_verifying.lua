-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: next_db_probe_at(unix ms)

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "COMMITTED" or state == "RELEASED" then
    return 0
end

local verifyStartedAt = redis.call("HGET", KEYS[1], "verify_started_at")
local probeAttempts = tonumber(redis.call("HGET", KEYS[1], "db_probe_attempts") or "0") + 1

local updates = {
    "state", "VERIFYING",
    "last_db_probe_at", ARGV[1],
    "db_probe_attempts", probeAttempts,
    "next_db_probe_at", ARGV[2]
}

if not verifyStartedAt or verifyStartedAt == "" or verifyStartedAt == "0" then
    table.insert(updates, "verify_started_at")
    table.insert(updates, ARGV[1])
end

if state ~= "VERIFYING" then
    table.insert(updates, "last_transition_at")
    table.insert(updates, ARGV[1])
end

redis.call("HSET", KEYS[1], unpack(updates))

return 1
