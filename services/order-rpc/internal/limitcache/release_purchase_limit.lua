local ledgerKey = KEYS[1]
if redis.call("EXISTS", ledgerKey) == 0 then
	return -1
end

local orderField = "reservation:" .. ARGV[1]
local ttlSeconds = tonumber(ARGV[2]) or 0
local reservedCount = tonumber(redis.call("HGET", ledgerKey, orderField) or "0")
if reservedCount == 0 then
	if ttlSeconds > 0 then
		redis.call("EXPIRE", ledgerKey, ttlSeconds)
	end
	return 1
end

local activeCount = tonumber(redis.call("HGET", ledgerKey, "active_count") or "0")
local nextCount = activeCount - reservedCount
if nextCount < 0 then
	nextCount = 0
end

redis.call("HSET", ledgerKey, "active_count", nextCount)
redis.call("HDEL", ledgerKey, orderField)
if ttlSeconds > 0 then
	redis.call("EXPIRE", ledgerKey, ttlSeconds)
end

return 1
