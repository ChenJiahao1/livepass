package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	workersvc "livepass/jobs/rush-inventory-preheat/internal/svc"
	"livepass/jobs/rush-inventory-preheat/internal/worker"
	"livepass/jobs/rush-inventory-preheat/taskdef"
	orderrpc "livepass/services/order-rpc/orderrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
)

type fakeWorkerShowTimeStore struct {
	listResp      []*workersvc.ShowTimeRecord
	listErr       error
	markCalls     int
	lastProgramID int64
}

func (f *fakeWorkerShowTimeStore) ListByProgramID(_ context.Context, programID int64) ([]*workersvc.ShowTimeRecord, error) {
	f.lastProgramID = programID
	return f.listResp, f.listErr
}

func (f *fakeWorkerShowTimeStore) MarkInventoryPreheatedByProgram(_ context.Context, programID int64, _ time.Time, _ time.Time) (bool, error) {
	f.markCalls++
	f.lastProgramID = programID
	return true, nil
}

type fakeWorkerOrderRPC struct {
	reqs []*orderrpc.PrimeRushRuntimeReq
}

func (f *fakeWorkerOrderRPC) PrimeRushRuntime(_ context.Context, in *orderrpc.PrimeRushRuntimeReq) (*orderrpc.BoolResp, error) {
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
	body, err := taskdef.Marshal(84001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	orderRPC := &fakeWorkerOrderRPC{}
	programRPC := &fakeWorkerProgramRPC{}
	mux := worker.NewServeMux(&workersvc.WorkerServiceContext{
		ShowTimeStore: &fakeWorkerShowTimeStore{
			listResp: []*workersvc.ShowTimeRecord{
				{
					ID:               94001,
					ProgramID:        84001,
					RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime, Valid: true},
				},
				{
					ID:               94002,
					ProgramID:        84001,
					RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime, Valid: true},
				},
			},
		},
		OrderRpc:   orderRPC,
		ProgramRpc: programRPC,
	})

	if err := mux.ProcessTask(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body)); err != nil {
		t.Fatalf("ProcessTask returned error: %v", err)
	}
	if len(orderRPC.reqs) != 1 || orderRPC.reqs[0].GetProgramId() != 84001 {
		t.Fatalf("unexpected order rpc requests: %+v", orderRPC.reqs)
	}
	if len(programRPC.reqs) != 2 {
		t.Fatalf("unexpected program rpc requests: %+v", programRPC.reqs)
	}
}
