package sharding

import "time"

const (
	dbGeneBits       = 5
	tableGeneBits    = 5
	workerIDBits     = 10
	sequenceBits     = 12
	logicSlotBits    = dbGeneBits + tableGeneBits
	logicSlotCount   = 1 << logicSlotBits
	maxDBGene        = (1 << dbGeneBits) - 1
	maxTableGene     = (1 << tableGeneBits) - 1
	maxWorkerID      = (1 << workerIDBits) - 1
	maxSequence      = (1 << sequenceBits) - 1
	maxOrderTimePart = (1 << 31) - 1
)

var orderNumberEpoch = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

func LogicSlotByUserID(userID int64) int {
	dbGene, tableGene := genePartsByUserID(userID)
	return int(dbGene)<<tableGeneBits | int(tableGene)
}

func genePartsByUserID(userID int64) (uint8, uint8) {
	mixed := splitMix64(uint64(userID))
	return uint8(mixed & maxDBGene), uint8((mixed >> dbGeneBits) & maxTableGene)
}

func splitMix64(v uint64) uint64 {
	v += 0x9e3779b97f4a7c15
	v = (v ^ (v >> 30)) * 0xbf58476d1ce4e5b9
	v = (v ^ (v >> 27)) * 0x94d049bb133111eb
	return v ^ (v >> 31)
}
