package integration_test

import (
	"context"
	"errors"
	"testing"

	"damai-go/pkg/xerr"
)

func TestPurchaseLimitStoreRejectsMissingLedgerAndSchedulesLoad(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          8101,
		OrderNumber: 9101,
		ProgramID:   10001,
		UserID:      3001,
		TicketCount: 2,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-ledger-load-001",
	})
	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)

	err := svcCtx.PurchaseLimitStore.Reserve(context.Background(), 3001, 10001, 9201, 1, 4)
	if !errors.Is(err, xerr.ErrOrderLimitLedgerNotReady) {
		t.Fatalf("expected ledger not ready, got %v", err)
	}

	snapshot := waitPurchaseLimitLedgerReady(t, svcCtx, 3001, 10001, 2)
	if snapshot.Loading {
		t.Fatalf("expected loading marker to be cleared after async load")
	}
}

func TestPurchaseLimitStoreReservesWithinLimit(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 1, nil)

	if err := svcCtx.PurchaseLimitStore.Reserve(context.Background(), 3001, 10001, 9202, 2, 3); err != nil {
		t.Fatalf("reserve purchase limit error: %v", err)
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if !snapshot.Ready {
		t.Fatalf("expected purchase limit ledger to be ready")
	}
	if snapshot.ActiveCount != 3 {
		t.Fatalf("expected active count 3, got %d", snapshot.ActiveCount)
	}
	if snapshot.Reservations[9202] != 2 {
		t.Fatalf("expected reservation count 2, got %+v", snapshot.Reservations)
	}
}

func TestPurchaseLimitStoreRejectsOverLimit(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 2, nil)

	err := svcCtx.PurchaseLimitStore.Reserve(context.Background(), 3001, 10001, 9203, 2, 3)
	if !errors.Is(err, xerr.ErrOrderPurchaseLimitExceeded) {
		t.Fatalf("expected purchase limit exceeded, got %v", err)
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 2 {
		t.Fatalf("expected active count to stay 2, got %d", snapshot.ActiveCount)
	}
	if len(snapshot.Reservations) != 0 {
		t.Fatalf("expected no reservations to be recorded, got %+v", snapshot.Reservations)
	}
}

func TestPurchaseLimitStoreReleaseIsIdempotent(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)
	seedPurchaseLimitLedger(t, svcCtx, 3001, 10001, 3, map[int64]int64{
		9204: 2,
		9205: 1,
	})

	if err := svcCtx.PurchaseLimitStore.Release(context.Background(), 3001, 10001, 9204); err != nil {
		t.Fatalf("release purchase limit error: %v", err)
	}
	if err := svcCtx.PurchaseLimitStore.Release(context.Background(), 3001, 10001, 9204); err != nil {
		t.Fatalf("release purchase limit should be idempotent, got %v", err)
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 1 {
		t.Fatalf("expected active count 1 after release, got %d", snapshot.ActiveCount)
	}
	if _, ok := snapshot.Reservations[9204]; ok {
		t.Fatalf("expected released reservation to be deleted, got %+v", snapshot.Reservations)
	}
	if snapshot.Reservations[9205] != 1 {
		t.Fatalf("expected other reservations to stay intact, got %+v", snapshot.Reservations)
	}
}

func TestPurchaseLimitStoreLoaderRestoresUnpaidOrderReservations(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx,
		orderFixture{
			ID:          8102,
			OrderNumber: 9102,
			ProgramID:   10001,
			UserID:      3001,
			TicketCount: 2,
			OrderStatus: testOrderStatusUnpaid,
			FreezeToken: "freeze-loader-reservation-001",
		},
		orderFixture{
			ID:           8103,
			OrderNumber:  9103,
			ProgramID:    10001,
			UserID:       3001,
			TicketCount:  1,
			OrderStatus:  testOrderStatusPaid,
			FreezeToken:  "freeze-loader-reservation-002",
			PayOrderTime: "2026-01-01 01:00:00",
		},
	)
	clearPurchaseLimitLedger(t, svcCtx, 3001, 10001)

	err := svcCtx.PurchaseLimitStore.Reserve(context.Background(), 3001, 10001, 9206, 1, 5)
	if !errors.Is(err, xerr.ErrOrderLimitLedgerNotReady) {
		t.Fatalf("expected ledger not ready, got %v", err)
	}
	waitPurchaseLimitLedgerReady(t, svcCtx, 3001, 10001, 3)

	if err := svcCtx.PurchaseLimitStore.Release(context.Background(), 3001, 10001, 9102); err != nil {
		t.Fatalf("release loaded unpaid reservation error: %v", err)
	}

	snapshot := requirePurchaseLimitSnapshot(t, svcCtx, 3001, 10001)
	if snapshot.ActiveCount != 1 {
		t.Fatalf("expected only paid tickets to remain active after release, got %d", snapshot.ActiveCount)
	}
	if _, ok := snapshot.Reservations[9102]; ok {
		t.Fatalf("expected unpaid reservation to be removed after release, got %+v", snapshot.Reservations)
	}
}
