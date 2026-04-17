package logic

import (
	"errors"
	"testing"

	"livepass/services/order-rpc/internal/rush"
)

func TestHandleFinalizeFailureOutcomeTransitionedReleasesFreeze(t *testing.T) {
	releaseFreeze, err := handleFinalizeFailureOutcome(rush.AttemptTransitioned, nil)
	if err != nil {
		t.Fatalf("handleFinalizeFailureOutcome() error = %v", err)
	}
	if !releaseFreeze {
		t.Fatalf("expected transitioned finalize failure to release freeze")
	}
}

func TestHandleFinalizeFailureOutcomeAlreadyFailedIsIdempotent(t *testing.T) {
	releaseFreeze, err := handleFinalizeFailureOutcome(rush.AttemptAlreadyFailed, nil)
	if err != nil {
		t.Fatalf("handleFinalizeFailureOutcome() error = %v", err)
	}
	if releaseFreeze {
		t.Fatalf("expected already-failed finalize failure not to release freeze again")
	}
}

func TestHandleFinalizeFailureOutcomeAlreadySucceededFollowsWinner(t *testing.T) {
	releaseFreeze, err := handleFinalizeFailureOutcome(rush.AttemptAlreadySucceeded, nil)
	if err != nil {
		t.Fatalf("handleFinalizeFailureOutcome() error = %v", err)
	}
	if releaseFreeze {
		t.Fatalf("expected already-succeeded finalize failure to follow winner")
	}
}

func TestHandleFinalizeFailureOutcomeLostOwnershipFollowsWinner(t *testing.T) {
	releaseFreeze, err := handleFinalizeFailureOutcome(rush.AttemptLostOwnership, nil)
	if err != nil {
		t.Fatalf("handleFinalizeFailureOutcome() error = %v", err)
	}
	if releaseFreeze {
		t.Fatalf("expected lost-ownership finalize failure to follow winner")
	}
}

func TestHandleFinalizeFailureOutcomeStateMissingRetries(t *testing.T) {
	releaseFreeze, err := handleFinalizeFailureOutcome(rush.AttemptStateMissing, nil)
	if err == nil {
		t.Fatalf("expected state-missing finalize failure to request retry")
	}
	if releaseFreeze {
		t.Fatalf("expected state-missing finalize failure not to release freeze")
	}
}

func TestFinalizeFailureRetriesWhenScriptErrorLeavesProcessingOwner(t *testing.T) {
	record := &rush.AttemptRecord{
		OrderNumber:      91001,
		State:            rush.AttemptStateProcessing,
		ProcessingEpoch:  3,
		TicketCategoryID: 40001,
	}
	latest := &rush.AttemptRecord{
		OrderNumber:      91001,
		State:            rush.AttemptStateProcessing,
		ProcessingEpoch:  3,
		TicketCategoryID: 40001,
	}

	if !shouldRetryFinalizeFailure(record, latest, errors.New("redis eval failed")) {
		t.Fatalf("expected script error with same processing owner to retry")
	}
}
