-- KEYS:
-- 1: attempt record key(hash)
--
-- ARGV:
-- 1: now(unix ms)
-- 2: processing ttl seconds

if redis.call("EXISTS", KEYS[1]) == 0 then
    return {-1}
end

local state = redis.call("HGET", KEYS[1], "state") or ""
local shouldProcess = 0

if state == "ACCEPTED" then
    state = "PROCESSING"
    redis.call("HSET", KEYS[1],
        "state", state,
        "processing_started_at", ARGV[1],
        "last_transition_at", ARGV[1]
    )
    local processingTTL = tonumber(ARGV[2]) or 0
    if processingTTL > 0 then
        redis.call("EXPIRE", KEYS[1], processingTTL)
    end
    shouldProcess = 1
end

return {
    shouldProcess,
    state,
    redis.call("HGET", KEYS[1], "order_number") or "",
    redis.call("HGET", KEYS[1], "user_id") or "",
    redis.call("HGET", KEYS[1], "program_id") or "",
    redis.call("HGET", KEYS[1], "show_time_id") or "",
    redis.call("HGET", KEYS[1], "ticket_category_id") or "",
    redis.call("HGET", KEYS[1], "viewer_ids") or "",
    redis.call("HGET", KEYS[1], "ticket_count") or "",
    redis.call("HGET", KEYS[1], "token_fingerprint") or "",
    redis.call("HGET", KEYS[1], "sale_window_end_at") or "",
    redis.call("HGET", KEYS[1], "show_end_at") or "",
    redis.call("HGET", KEYS[1], "reason_code") or "",
    redis.call("HGET", KEYS[1], "accepted_at") or "",
    redis.call("HGET", KEYS[1], "finished_at") or "",
    redis.call("HGET", KEYS[1], "processing_started_at") or "",
    redis.call("HGET", KEYS[1], "created_at") or "",
    redis.call("HGET", KEYS[1], "last_transition_at") or ""
}
