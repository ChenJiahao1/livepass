local stockKey = KEYS[1]
local availableKey = KEYS[2]
local frozenKey = KEYS[3]

if redis.call("EXISTS", stockKey) == 0 then
	return {-1}
end

local count = tonumber(ARGV[1]) or 0
local stockTTLSeconds = tonumber(ARGV[2]) or 0
local seatTTLSeconds = tonumber(ARGV[3]) or 0
local availableCount = tonumber(redis.call("HGET", stockKey, "available_count") or "0")
if availableCount < count then
	return {0}
end

local existingFrozen = redis.call("ZRANGE", frozenKey, 0, -1)
if #existingFrozen > 0 then
	if stockTTLSeconds > 0 then
		redis.call("EXPIRE", stockKey, stockTTLSeconds)
	end
	if seatTTLSeconds > 0 then
		redis.call("EXPIRE", frozenKey, seatTTLSeconds)
		redis.call("EXPIRE", availableKey, seatTTLSeconds)
	end

	local resp = {1}
	for i = 1, #existingFrozen do
		table.insert(resp, existingFrozen[i])
	end
	return resp
end

local availableSeats = redis.call("ZRANGE", availableKey, 0, -1)
if #availableSeats < count then
	return {0}
end

local function parseSeat(member)
	local seatId, ticketCategoryId, rowCode, colCode, price = string.match(member, "([^|]+)|([^|]+)|([^|]+)|([^|]+)|([^|]+)")
	return tonumber(seatId), tonumber(ticketCategoryId), tonumber(rowCode), tonumber(colCode), tonumber(price)
end

local function firstConsecutiveSeats(rowSeats, need)
	if #rowSeats < need then
		return nil
	end

	local runStart = 1
	for i = 2, #rowSeats + 1 do
		local sameRun = false
		if i <= #rowSeats then
			local _, _, currentRow, currentCol = parseSeat(rowSeats[i])
			local _, _, prevRow, prevCol = parseSeat(rowSeats[i - 1])
			sameRun = currentRow == prevRow and currentCol == prevCol + 1
		end
		if not sameRun then
			if i - runStart >= need then
				local selected = {}
				for j = runStart, runStart + need - 1 do
					table.insert(selected, rowSeats[j])
				end
				return selected
			end
			runStart = i
		end
	end

	return nil
end

local selectedSeats = nil
local startIndex = 1
while startIndex <= #availableSeats do
	local _, _, rowCode = parseSeat(availableSeats[startIndex])
	local endIndex = startIndex
	while endIndex + 1 <= #availableSeats do
		local _, _, nextRowCode = parseSeat(availableSeats[endIndex + 1])
		if nextRowCode ~= rowCode then
			break
		end
		endIndex = endIndex + 1
	end

	local rowSeats = {}
	for i = startIndex, endIndex do
		table.insert(rowSeats, availableSeats[i])
	end

	local consecutiveSeats = firstConsecutiveSeats(rowSeats, count)
	if consecutiveSeats ~= nil then
		selectedSeats = consecutiveSeats
		break
	end
	startIndex = endIndex + 1
end

if selectedSeats == nil then
	selectedSeats = {}
	for i = 1, count do
		table.insert(selectedSeats, availableSeats[i])
	end
end

for _, member in ipairs(selectedSeats) do
	local _, _, rowCode, colCode = parseSeat(member)
	redis.call("ZREM", availableKey, member)
	redis.call("ZADD", frozenKey, rowCode * 1000000 + colCode, member)
end

redis.call("HINCRBY", stockKey, "available_count", -count)
if stockTTLSeconds > 0 then
	redis.call("EXPIRE", stockKey, stockTTLSeconds)
end
if seatTTLSeconds > 0 then
	redis.call("EXPIRE", frozenKey, seatTTLSeconds)
	redis.call("EXPIRE", availableKey, seatTTLSeconds)
end

local resp = {1}
for i = 1, #selectedSeats do
	table.insert(resp, selectedSeats[i])
end
return resp
