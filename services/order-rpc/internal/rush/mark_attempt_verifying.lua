-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)

if redis.call("EXISTS", KEYS[1]) == 0 then
    return -1
end

local state = redis.call("HGET", KEYS[1], "state")
if state == "COMMITTED" or state == "RELEASED" then
    return 0
end

local verifyStartedAt = redis.call("HGET", KEYS[1], "verify_started_at")

local updates = {
    "state", "VERIFYING"
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
