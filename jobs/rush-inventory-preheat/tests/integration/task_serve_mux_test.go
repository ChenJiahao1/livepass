package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	workersvc "damai-go/jobs/rush-inventory-preheat/internal/svc"
	"damai-go/jobs/rush-inventory-preheat/internal/worker"
	"damai-go/jobs/rush-inventory-preheat/taskdef"
	orderrpc "damai-go/services/order-rpc/orderrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
)

type fakeWorkerShowTimeStore struct {
	findResp   *workersvc.ShowTimeRecord
	findErr    error
	markCalls  int
	lastShowID int64
}

func (f *fakeWorkerShowTimeStore) FindOne(_ context.Context, showTimeID int64) (*workersvc.ShowTimeRecord, error) {
	f.lastShowID = showTimeID
	return f.findResp, f.findErr
}

func (f *fakeWorkerShowTimeStore) MarkInventoryPreheated(_ context.Context, showTimeID int64, _ time.Time, _ time.Time) (bool, error) {
	f.markCalls++
	f.lastShowID = showTimeID
	return true, nil
}

type fakeWorkerOrderRPC struct {
	reqs []*orderrpc.PrimeAdmissionQuotaReq
}

func (f *fakeWorkerOrderRPC) PrimeAdmissionQuota(_ context.Context, in *orderrpc.PrimeAdmissionQuotaReq) (*orderrpc.BoolResp, error) {
	f.reqs = append(f.reqs, in)
	return &orderrpc.BoolResp{Success: true}, nil
}

type fakeWorkerProgramRPC struct {
	reqs []*programrpc.PrimeSeatLedgerReq
}

func (f *fakeWorkerProgramRPC) PrimeSeatLedger(_ context.Context, in *programrpc.PrimeSeatLedgerReq) (*programrpc.BoolResp, error) {
	f.reqs = append(f.reqs, in)
	return &programrpc.BoolResp{Success: true}, nil
}

func TestWorkerServeMuxRoutesRushInventoryPreheatTask(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(94001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	orderRPC := &fakeWorkerOrderRPC{}
	programRPC := &fakeWorkerProgramRPC{}
	mux := worker.NewServeMux(&workersvc.WorkerServiceContext{
		ShowTimeStore: &fakeWorkerShowTimeStore{
			findResp: &workersvc.ShowTimeRecord{
				ID:               94001,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime, Valid: true},
			},
		},
		OrderRpc:   orderRPC,
		ProgramRpc: programRPC,
	})

	if err := mux.ProcessTask(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body)); err != nil {
		t.Fatalf("ProcessTask returned error: %v", err)
	}
	if len(orderRPC.reqs) != 1 || orderRPC.reqs[0].GetShowTimeId() != 94001 {
		t.Fatalf("unexpected order rpc requests: %+v", orderRPC.reqs)
	}
	if len(programRPC.reqs) != 1 || programRPC.reqs[0].GetShowTimeId() != 94001 {
		t.Fatalf("unexpected program rpc requests: %+v", programRPC.reqs)
	}
}
