package logic

import (
	"errors"
	"testing"

	"damai-go/pkg/xerr"
)

func TestSeatAssignmentPrefersSameRowConsecutiveSeats(t *testing.T) {
	got, err := assignSeats([]seatCandidate{
		{ID: 1, TicketCategoryID: 101, RowCode: 1, ColCode: 1, Price: 299},
		{ID: 2, TicketCategoryID: 101, RowCode: 1, ColCode: 2, Price: 299},
		{ID: 3, TicketCategoryID: 101, RowCode: 2, ColCode: 1, Price: 299},
	}, 2)
	if err != nil {
		t.Fatalf("assignSeats returned error: %v", err)
	}

	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("expected consecutive seats [1 2], got %+v", got)
	}
}

func TestSeatAssignmentFallsBackToRowMajorOrderWhenNoConsecutiveBlock(t *testing.T) {
	got, err := assignSeats([]seatCandidate{
		{ID: 1, TicketCategoryID: 101, RowCode: 1, ColCode: 1, Price: 299},
		{ID: 2, TicketCategoryID: 101, RowCode: 1, ColCode: 3, Price: 299},
		{ID: 3, TicketCategoryID: 101, RowCode: 2, ColCode: 2, Price: 299},
	}, 2)
	if err != nil {
		t.Fatalf("assignSeats returned error: %v", err)
	}

	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("expected row-major seats [1 2], got %+v", got)
	}
}

func TestSeatAssignmentReturnsInsufficientWhenCandidateCountTooSmall(t *testing.T) {
	_, err := assignSeats([]seatCandidate{
		{ID: 1, TicketCategoryID: 101, RowCode: 1, ColCode: 1, Price: 299},
	}, 2)
	if !errors.Is(err, xerr.ErrSeatInventoryInsufficient) {
		t.Fatalf("expected ErrSeatInventoryInsufficient, got %v", err)
	}
}

func TestSeatAssignmentRejectsNonPositiveCount(t *testing.T) {
	_, err := assignSeats([]seatCandidate{
		{ID: 1, TicketCategoryID: 101, RowCode: 1, ColCode: 1, Price: 299},
	}, 0)
	if err == nil {
		t.Fatalf("expected error for non-positive count")
	}
}
