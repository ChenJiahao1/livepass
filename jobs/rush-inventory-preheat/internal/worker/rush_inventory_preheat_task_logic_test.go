package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"damai-go/jobs/rush-inventory-preheat/internal/svc"
	"damai-go/jobs/rush-inventory-preheat/taskdef"
	orderrpc "damai-go/services/order-rpc/orderrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
)

type fakeRushInventoryShowTimeStore struct {
	findResp           *svc.ShowTimeRecord
	findErr            error
	findCalls          int
	lastFindShowTimeID int64

	markUpdated              bool
	markErr                  error
	markCalls                int
	lastMarkShowTimeID       int64
	lastMarkExpectedOpenTime time.Time
	lastMarkUpdatedAt        time.Time
}

func (f *fakeRushInventoryShowTimeStore) FindOne(_ context.Context, showTimeID int64) (*svc.ShowTimeRecord, error) {
	f.findCalls++
	f.lastFindShowTimeID = showTimeID
	return f.findResp, f.findErr
}

func (f *fakeRushInventoryShowTimeStore) MarkInventoryPreheated(_ context.Context, showTimeID int64, expectedOpenTime time.Time, updatedAt time.Time) (bool, error) {
	f.markCalls++
	f.lastMarkShowTimeID = showTimeID
	f.lastMarkExpectedOpenTime = expectedOpenTime
	f.lastMarkUpdatedAt = updatedAt
	return f.markUpdated, f.markErr
}

type fakeRushInventoryOrderRPC struct {
	primeCalls int
	lastReq    *orderrpc.PrimeAdmissionQuotaReq
	err        error
}

func (f *fakeRushInventoryOrderRPC) PrimeAdmissionQuota(_ context.Context, in *orderrpc.PrimeAdmissionQuotaReq) (*orderrpc.BoolResp, error) {
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
	body, err := taskdef.Marshal(93001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	showTimeStore := &fakeRushInventoryShowTimeStore{
		findResp: &svc.ShowTimeRecord{
			ID:                     93001,
			RushSaleOpenTime:       sql.NullTime{Time: expectedOpenTime, Valid: true},
			InventoryPreheatStatus: 1,
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
	if orderRPC.primeCalls != 1 || orderRPC.lastReq == nil || orderRPC.lastReq.ShowTimeId != 93001 {
		t.Fatalf("unexpected PrimeAdmissionQuota calls: calls=%d req=%+v", orderRPC.primeCalls, orderRPC.lastReq)
	}
	if programRPC.primeCalls != 1 || programRPC.lastReq == nil || programRPC.lastReq.ShowTimeId != 93001 {
		t.Fatalf("unexpected PrimeSeatLedger calls: calls=%d req=%+v", programRPC.primeCalls, programRPC.lastReq)
	}
	if showTimeStore.markCalls != 1 || showTimeStore.lastMarkShowTimeID != 93001 {
		t.Fatalf("unexpected MarkInventoryPreheated calls: calls=%d showTimeId=%d", showTimeStore.markCalls, showTimeStore.lastMarkShowTimeID)
	}
	if !showTimeStore.lastMarkExpectedOpenTime.Equal(expectedOpenTime) {
		t.Fatalf("expected mark with open time %v, got %v", expectedOpenTime, showTimeStore.lastMarkExpectedOpenTime)
	}
}

func TestRushInventoryPreheatTaskLogicHandleSkipsOnOpenTimeMismatch(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(93002, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: &fakeRushInventoryShowTimeStore{
			findResp: &svc.ShowTimeRecord{
				ID:               93002,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime.Add(30 * time.Minute), Valid: true},
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
