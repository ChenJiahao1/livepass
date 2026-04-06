local stockKey = KEYS[1]
local availableKey = KEYS[2]
local frozenKey = KEYS[3]
local metaKey = KEYS[4]

if redis.call("EXISTS", stockKey) == 0 then
	return -1
end

local stockTTLSeconds = tonumber(ARGV[1]) or 0
local seatTTLSeconds = tonumber(ARGV[2]) or 0
local ownerOrderNumber = tonumber(ARGV[3]) or 0
local ownerEpoch = tonumber(ARGV[4]) or 0
local metaRaw = redis.call("GET", metaKey)
if metaRaw then
	local meta = cjson.decode(metaRaw)
	local expectedOrderNumber = tonumber(meta["ownerOrderNumber"] or "0")
	local expectedEpoch = tonumber(meta["ownerEpoch"] or "0")
	if ownerOrderNumber > 0 or ownerEpoch > 0 then
		if expectedOrderNumber > 0 and expectedEpoch > 0 and (expectedOrderNumber ~= ownerOrderNumber or expectedEpoch ~= ownerEpoch) then
			return -2
		end
	end
end
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
redis.call("HINCRBY", stockKey, "available_count", #frozenSeats)
if stockTTLSeconds > 0 then
	redis.call("EXPIRE", stockKey, stockTTLSeconds)
end
if seatTTLSeconds > 0 then
	redis.call("EXPIRE", availableKey, seatTTLSeconds)
end

return 1
