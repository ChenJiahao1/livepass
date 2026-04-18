local stockKey = KEYS[1]
local availableKey = KEYS[2]
local frozenKey = KEYS[3]

if redis.call("EXISTS", stockKey) == 0 then
	return -1
end

local stockTTLSeconds = tonumber(ARGV[1]) or 0
local seatTTLSeconds = tonumber(ARGV[2]) or 0
local frozenSeats = redis.call("ZRANGE", frozenKey, 0, -1)
if #frozenSeats == 0 then
	if stockTTLSeconds > 0 then
		redis.call("EXPIRE", stockKey, stockTTLSeconds)
	end
	if seatTTLSeconds > 0 then
		redis.call("EXPIRE", availableKey, seatTTLSeconds)
	end
	return 1
end

local function parseSeat(member)
	local seatId, ticketCategoryId, rowCode, colCode, price = string.match(member, "([^|]+)|([^|]+)|([^|]+)|([^|]+)|([^|]+)")
	return tonumber(seatId), tonumber(ticketCategoryId), tonumber(rowCode), tonumber(colCode), tonumber(price)
end

for _, member in ipairs(frozenSeats) do
	local _, _, rowCode, colCode = parseSeat(member)
	redis.call("ZADD", availableKey, rowCode * 1000000 + colCode, member)
end

redis.call("DEL", frozenKey)
redis.call("INCRBY", stockKey, #frozenSeats)
if stockTTLSeconds > 0 then
	redis.call("EXPIRE", stockKey, stockTTLSeconds)
end
if seatTTLSeconds > 0 then
	redis.call("EXPIRE", availableKey, seatTTLSeconds)
end

return 1
