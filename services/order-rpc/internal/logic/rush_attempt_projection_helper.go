package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
)

const defaultRushReconcileLimit int64 = 100

func syncClosedRushAttemptProjection(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64, now time.Time) error {
	if svcCtx == nil || svcCtx.AttemptStore == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	record, err := svcCtx.AttemptStore.Get(ctx, orderNumber)
	if err != nil {
		if errors.Is(err, xerr.ErrOrderNotFound) {
			return nil
		}
		return err
	}

	return svcCtx.AttemptStore.ReleaseClosedOrderProjection(ctx, record, now)
}

func shouldReconcileRushAttempt(record *rush.AttemptRecord, now time.Time) bool {
	if record == nil {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}

	switch record.State {
	case rush.AttemptStateReleased:
		return false
	case rush.AttemptStateCommitted:
		return true
	case rush.AttemptStateVerifying:
		return record.NextDBProbeAt.IsZero() || !now.Before(record.NextDBProbeAt)
	case rush.AttemptStatePendingPublish, rush.AttemptStateQueued, rush.AttemptStateProcessing:
		return record.UserDeadlineAt.IsZero() || !now.Before(record.UserDeadlineAt)
	default:
		return false
	}
}

func advanceRushAttemptProjection(ctx context.Context, svcCtx *svc.ServiceContext, record *rush.AttemptRecord, now time.Time) (bool, error) {
	if svcCtx == nil || svcCtx.AttemptStore == nil || svcCtx.OrderRepository == nil {
		return false, xerr.ErrInternal
	}
	if record == nil || record.OrderNumber <= 0 {
		return false, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	order, err := svcCtx.OrderRepository.FindOrderByNumber(ctx, record.OrderNumber)
	if err == nil && order != nil {
		if order.OrderStatus == orderStatusCancelled {
			return true, svcCtx.AttemptStore.ReleaseClosedOrderProjection(ctx, record, now)
		}
		if record.State == rush.AttemptStateCommitted {
			return false, nil
		}
		return true, svcCtx.AttemptStore.CommitProjection(ctx, record, now)
	}
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return false, err
	}
	if record.State == rush.AttemptStateCommitted {
		return false, nil
	}
	if !record.CommitCutoffAt.IsZero() && !now.Before(record.CommitCutoffAt) {
		return true, svcCtx.AttemptStore.Release(ctx, record, rush.AttemptReasonCommitCutoffExceed, now)
	}

	return true, svcCtx.AttemptStore.MarkVerifying(ctx, record.OrderNumber, now, nextRushAttemptProbeAt(svcCtx, now, record.CommitCutoffAt))
}

func nextRushAttemptProbeAt(svcCtx *svc.ServiceContext, now, cutoffAt time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}

	interval := 500 * time.Millisecond
	if svcCtx != nil && svcCtx.Config.Kafka.RetryBackoff > 0 {
		interval = svcCtx.Config.Kafka.RetryBackoff
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	next := now.Add(interval)
	if !cutoffAt.IsZero() && next.After(cutoffAt) {
		next = cutoffAt
	}
	if next.Before(now) {
		next = now
	}

	return next
}
