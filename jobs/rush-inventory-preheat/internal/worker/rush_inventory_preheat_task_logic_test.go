package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/svc"
	"livepass/jobs/rush-inventory-preheat/taskdef"
	orderrpc "livepass/services/order-rpc/orderrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
)

type fakeRushInventoryShowTimeStore struct {
	listResp          []*svc.ShowTimeRecord
	listErr           error
	listCalls         int
	lastListProgramID int64

	markUpdated              bool
	markErr                  error
	markCalls                int
	lastMarkProgramID        int64
	lastMarkExpectedOpenTime time.Time
	lastMarkUpdatedAt        time.Time
}

func (f *fakeRushInventoryShowTimeStore) ListByProgramID(_ context.Context, programID int64) ([]*svc.ShowTimeRecord, error) {
	f.listCalls++
	f.lastListProgramID = programID
	return f.listResp, f.listErr
}

func (f *fakeRushInventoryShowTimeStore) MarkInventoryPreheatedByProgram(_ context.Context, programID int64, expectedOpenTime time.Time, updatedAt time.Time) (bool, error) {
	f.markCalls++
	f.lastMarkProgramID = programID
	f.lastMarkExpectedOpenTime = expectedOpenTime
	f.lastMarkUpdatedAt = updatedAt
	return f.markUpdated, f.markErr
}

type fakeRushInventoryOrderRPC struct {
	primeCalls int
	lastReq    *orderrpc.PrimeRushRuntimeReq
	err        error
}

func (f *fakeRushInventoryOrderRPC) PrimeRushRuntime(_ context.Context, in *orderrpc.PrimeRushRuntimeReq) (*orderrpc.BoolResp, error) {
	f.primeCalls++
	f.lastReq = in
	return &orderrpc.BoolResp{Success: true}, f.err
}

type fakeRushInventoryProgramRPC struct {
	primeCalls int
	lastReq    *programrpc.PrimeSeatLedgerReq
	err        error
}

func (f *fakeRushInventoryProgramRPC) PrimeSeatLedger(_ context.Context, in *programrpc.PrimeSeatLedgerReq) (*programrpc.BoolResp, error) {
	f.primeCalls++
	f.lastReq = in
	return &programrpc.BoolResp{Success: true}, f.err
}

func TestRushInventoryPreheatTaskLogicHandleCallsBothRPCsAndMarksCompleted(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(83001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	showTimeStore := &fakeRushInventoryShowTimeStore{
		listResp: []*svc.ShowTimeRecord{
			{
				ID:                     93001,
				ProgramID:              83001,
				RushSaleOpenTime:       sql.NullTime{Time: expectedOpenTime, Valid: true},
				InventoryPreheatStatus: 1,
			},
			{
				ID:                     93002,
				ProgramID:              83001,
				RushSaleOpenTime:       sql.NullTime{Time: expectedOpenTime, Valid: true},
				InventoryPreheatStatus: 1,
			},
		},
		markUpdated: true,
	}
	orderRPC := &fakeRushInventoryOrderRPC{}
	programRPC := &fakeRushInventoryProgramRPC{}
	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: showTimeStore,
		OrderRpc:      orderRPC,
		ProgramRpc:    programRPC,
	})

	err = logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if orderRPC.primeCalls != 1 || orderRPC.lastReq == nil || orderRPC.lastReq.ProgramId != 83001 {
		t.Fatalf("unexpected PrimeRushRuntime calls: calls=%d req=%+v", orderRPC.primeCalls, orderRPC.lastReq)
	}
	if programRPC.primeCalls != 2 {
		t.Fatalf("unexpected PrimeSeatLedger calls: calls=%d req=%+v", programRPC.primeCalls, programRPC.lastReq)
	}
	if showTimeStore.markCalls != 1 || showTimeStore.lastMarkProgramID != 83001 {
		t.Fatalf("unexpected MarkInventoryPreheatedByProgram calls: calls=%d programId=%d", showTimeStore.markCalls, showTimeStore.lastMarkProgramID)
	}
	if !showTimeStore.lastMarkExpectedOpenTime.Equal(expectedOpenTime) {
		t.Fatalf("expected mark with open time %v, got %v", expectedOpenTime, showTimeStore.lastMarkExpectedOpenTime)
	}
}

func TestRushInventoryPreheatTaskLogicHandleSkipsOnOpenTimeMismatch(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(83002, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: &fakeRushInventoryShowTimeStore{
			listResp: []*svc.ShowTimeRecord{
				{
					ID:               93002,
					ProgramID:        83002,
					RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime.Add(30 * time.Minute), Valid: true},
				},
			},
		},
		OrderRpc:   &fakeRushInventoryOrderRPC{},
		ProgramRpc: &fakeRushInventoryProgramRPC{},
	})

	err = logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
}

func TestRushInventoryPreheatTaskLogicHandleSkipsWholeProgramWhenAnyOpenTimeMismatches(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(83003, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	showTimeStore := &fakeRushInventoryShowTimeStore{
		listResp: []*svc.ShowTimeRecord{
			{
				ID:               93003,
				ProgramID:        83003,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime, Valid: true},
			},
			{
				ID:               93004,
				ProgramID:        83003,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime.Add(30 * time.Minute), Valid: true},
			},
		},
	}
	orderRPC := &fakeRushInventoryOrderRPC{}
	programRPC := &fakeRushInventoryProgramRPC{}
	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: showTimeStore,
		OrderRpc:      orderRPC,
		ProgramRpc:    programRPC,
	})

	err = logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if orderRPC.primeCalls != 0 {
		t.Fatalf("expected no PrimeRushRuntime calls, got %d", orderRPC.primeCalls)
	}
	if programRPC.primeCalls != 0 {
		t.Fatalf("expected no PrimeSeatLedger calls, got %d", programRPC.primeCalls)
	}
	if showTimeStore.markCalls != 0 {
		t.Fatalf("expected no MarkInventoryPreheatedByProgram calls, got %d", showTimeStore.markCalls)
	}
}

func TestRushInventoryPreheatTaskLogicHandleSkipsRetryOnBadPayload(t *testing.T) {
	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: &fakeRushInventoryShowTimeStore{},
		OrderRpc:      &fakeRushInventoryOrderRPC{},
		ProgramRpc:    &fakeRushInventoryProgramRPC{},
	})

	err := logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, []byte("{bad json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
}
