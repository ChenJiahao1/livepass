package logic

import (
	"errors"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
)

func handleFinalizeFailureOutcome(outcome rush.AttemptTransitionOutcome, err error) (bool, error) {
	if err != nil {
		return false, err
	}

	switch outcome {
	case rush.AttemptTransitioned:
		return true, nil
	case rush.AttemptAlreadyFailed, rush.AttemptAlreadySucceeded, rush.AttemptLostOwnership:
		return false, nil
	case rush.AttemptStateMissing:
		return false, xerr.ErrInternal
	default:
		return false, xerr.ErrInternal
	}
}

func shouldRetryFinalizeFailure(record, latest *rush.AttemptRecord, err error) bool {
	if err == nil || record == nil || latest == nil {
		return false
	}
	if !errors.Is(err, xerr.ErrInternal) && latest.State != rush.AttemptStateProcessing {
		return false
	}

	return latest.OrderNumber == record.OrderNumber &&
		latest.State == rush.AttemptStateProcessing
}
