package logic

import (
	"sort"

	"livepass/pkg/xerr"
)

type seatCandidate struct {
	ID               int64
	TicketCategoryID int64
	RowCode          int64
	ColCode          int64
	Price            float64
}

func assignSeats(candidates []seatCandidate, count int) ([]seatCandidate, error) {
	if count <= 0 {
		return nil, xerr.ErrInvalidParam
	}
	if len(candidates) < count {
		return nil, xerr.ErrSeatInventoryInsufficient
	}

	sorted := append([]seatCandidate(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].RowCode != sorted[j].RowCode {
			return sorted[i].RowCode < sorted[j].RowCode
		}
		if sorted[i].ColCode != sorted[j].ColCode {
			return sorted[i].ColCode < sorted[j].ColCode
		}
		return sorted[i].ID < sorted[j].ID
	})

	for start := 0; start < len(sorted); {
		end := start + 1
		for end < len(sorted) && sorted[end].RowCode == sorted[start].RowCode {
			end++
		}

		if seats := firstConsecutiveSeats(sorted[start:end], count); len(seats) == count {
			return seats, nil
		}
		start = end
	}

	return sorted[:count], nil
}

func firstConsecutiveSeats(rowSeats []seatCandidate, count int) []seatCandidate {
	if len(rowSeats) < count {
		return nil
	}

	runStart := 0
	for i := 1; i <= len(rowSeats); i++ {
		if i < len(rowSeats) && rowSeats[i].ColCode == rowSeats[i-1].ColCode+1 {
			continue
		}
		if i-runStart >= count {
			return append([]seatCandidate(nil), rowSeats[runStart:runStart+count]...)
		}
		runStart = i
	}

	return nil
}
