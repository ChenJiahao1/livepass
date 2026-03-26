package sharding

import (
	"errors"
	"fmt"
	"time"
)

const (
	dbGeneShift    = workerIDBits + sequenceBits + tableGeneBits
	tableGeneShift = workerIDBits + sequenceBits
	workerIDShift  = sequenceBits
	timePartShift  = dbGeneBits + tableGeneBits + workerIDBits + sequenceBits
)

var (
	ErrInvalidOrderNumber = errors.New("invalid order number")
)

type OrderNumberParts struct {
	TimePart  int64
	DBGene    uint8
	TableGene uint8
	WorkerID  int64
	Sequence  int64
}

func (p OrderNumberParts) LogicSlot() int {
	return int(p.DBGene)<<tableGeneBits | int(p.TableGene)
}

func BuildOrderNumber(userID int64, now time.Time, workerID, seq int64) int64 {
	dbGene, tableGene := genePartsByUserID(userID)
	timePart := now.UTC().Unix() - orderNumberEpoch
	if timePart < 0 || timePart > maxOrderTimePart {
		panic(fmt.Sprintf("order number time out of range: %d", timePart))
	}
	if workerID < 0 || workerID > maxWorkerID {
		panic(fmt.Sprintf("worker id out of range: %d", workerID))
	}
	if seq < 0 || seq > maxSequence {
		panic(fmt.Sprintf("sequence out of range: %d", seq))
	}

	return (timePart << timePartShift) |
		(int64(dbGene) << dbGeneShift) |
		(int64(tableGene) << tableGeneShift) |
		(workerID << workerIDShift) |
		seq
}

func ParseOrderNumber(orderNumber int64) (OrderNumberParts, error) {
	if orderNumber <= 0 {
		return OrderNumberParts{}, ErrInvalidOrderNumber
	}

	timePart := orderNumber >> timePartShift
	if isOldFormatOrderNumber(timePart) {
		return OrderNumberParts{}, ErrInvalidOrderNumber
	}

	return OrderNumberParts{
		TimePart:  timePart,
		DBGene:    uint8((orderNumber >> dbGeneShift) & maxDBGene),
		TableGene: uint8((orderNumber >> tableGeneShift) & maxTableGene),
		WorkerID:  (orderNumber >> workerIDShift) & maxWorkerID,
		Sequence:  orderNumber & maxSequence,
	}, nil
}

func LogicSlotByOrderNumber(orderNumber int64) (int, error) {
	parts, err := ParseOrderNumber(orderNumber)
	if err != nil {
		return 0, err
	}

	return parts.LogicSlot(), nil
}

func isOldFormatOrderNumber(timePart int64) bool {
	if timePart < 0 || timePart > maxOrderTimePart {
		return true
	}

	// Old snowflake IDs encode millisecond timestamps in the high bits.
	// After shifting into this layout they decode to implausibly future seconds.
	nowPart := time.Now().UTC().Unix() - orderNumberEpoch
	return timePart > nowPart+7*24*60*60
}
