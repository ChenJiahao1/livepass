local ledgerKey = KEYS[1]
if redis.call("EXISTS", ledgerKey) == 0 then
	return -1
end

local orderField = "reservation:" .. ARGV[1]
local existing = tonumber(redis.call("HGET", ledgerKey, orderField) or "0")
local ttlSeconds = tonumber(ARGV[4]) or 0
if existing > 0 then
	if ttlSeconds > 0 then
		redis.call("EXPIRE", ledgerKey, ttlSeconds)
	end
	return 1
end

local ticketCount = tonumber(ARGV[2]) or 0
local limit = tonumber(ARGV[3]) or 0
local activeCount = tonumber(redis.call("HGET", ledgerKey, "active_count") or "0")
if activeCount + ticketCount > limit then
	return 0
end

redis.call("HINCRBY", ledgerKey, "active_count", ticketCount)
redis.call("HSET", ledgerKey, orderField, ticketCount)
if ttlSeconds > 0 then
	redis.call("EXPIRE", ledgerKey, ttlSeconds)
end

return 1
