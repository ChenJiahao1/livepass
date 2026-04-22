package worker

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/svc"
	"livepass/jobs/rush-inventory-preheat/taskdef"
	orderrpc "livepass/services/order-rpc/orderrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"
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
	lastMarkTaskType         string
	lastMarkTaskKey          string

	processedCalls int
	failedCalls    int
	lastTaskKey    string
	lastTaskErr    string

	markConsumeAttempts      int64
	processedConsumeAttempts int64
	failedConsumeAttempts    int64
}

func (f *fakeRushInventoryShowTimeStore) ListByProgramID(_ context.Context, programID int64) ([]*svc.ShowTimeRecord, error) {
	f.listCalls++
	f.lastListProgramID = programID
	return f.listResp, f.listErr
}

func (f *fakeRushInventoryShowTimeStore) MarkInventoryPreheatedByProgramAndTaskProcessed(_ context.Context, programID int64, expectedOpenTime time.Time, taskType, taskKey string, updatedAt time.Time) (bool, int64, int64, error) {
	f.markCalls++
	f.lastMarkProgramID = programID
	f.lastMarkExpectedOpenTime = expectedOpenTime
	f.lastMarkUpdatedAt = updatedAt
	f.lastMarkTaskType = taskType
	f.lastMarkTaskKey = taskKey
	return f.markUpdated, 1, f.markConsumeAttempts, f.markErr
}

func (f *fakeRushInventoryShowTimeStore) MarkTaskProcessed(_ context.Context, taskType, taskKey string, processedAt time.Time) (int64, int64, error) {
	f.processedCalls++
	f.lastTaskKey = taskKey
	return 1, f.processedConsumeAttempts, nil
}

func (f *fakeRushInventoryShowTimeStore) MarkTaskConsumeFailed(_ context.Context, taskType, taskKey string, failedAt time.Time, consumeErr string) (int64, int64, error) {
	f.failedCalls++
	f.lastTaskKey = taskKey
	f.lastTaskErr = consumeErr
	return 1, f.failedConsumeAttempts, nil
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
	if showTimeStore.lastMarkTaskType != taskdef.TaskTypeRushInventoryPreheat ||
		showTimeStore.lastMarkTaskKey != taskdef.TaskKey(83001, expectedOpenTime) {
		t.Fatalf("unexpected processed task marker: type=%s key=%s", showTimeStore.lastMarkTaskType, showTimeStore.lastMarkTaskKey)
	}
	if !showTimeStore.lastMarkExpectedOpenTime.Equal(expectedOpenTime) {
		t.Fatalf("expected mark with open time %v, got %v", expectedOpenTime, showTimeStore.lastMarkExpectedOpenTime)
	}
}

func TestRushInventoryPreheatTaskLogicLogsActualConsumeAttempts(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(83005, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	logs := captureLogx(t)
	showTimeStore := &fakeRushInventoryShowTimeStore{
		listResp: []*svc.ShowTimeRecord{
			{
				ID:               93006,
				ProgramID:        83005,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime, Valid: true},
			},
		},
		markUpdated:         true,
		markConsumeAttempts: 4,
	}
	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: showTimeStore,
		OrderRpc:      &fakeRushInventoryOrderRPC{},
		ProgramRpc:    &fakeRushInventoryProgramRPC{},
	})

	if err := logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body)); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !strings.Contains(logs.String(), "consume_attempts=4") {
		t.Fatalf("expected consume_attempts=4 in logs, got %s", logs.String())
	}
}

func TestRushInventoryPreheatTaskLogicHandleSkipsOnOpenTimeMismatch(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(83002, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	showTimeStore := &fakeRushInventoryShowTimeStore{
		listResp: []*svc.ShowTimeRecord{
			{
				ID:               93002,
				ProgramID:        83002,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime.Add(30 * time.Minute), Valid: true},
			},
		},
	}
	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: showTimeStore,
		OrderRpc:      &fakeRushInventoryOrderRPC{},
		ProgramRpc:    &fakeRushInventoryProgramRPC{},
	})

	err = logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if showTimeStore.processedCalls != 1 || showTimeStore.lastTaskKey != taskdef.TaskKey(83002, expectedOpenTime) {
		t.Fatalf("expected no-op task processed, calls=%d key=%s", showTimeStore.processedCalls, showTimeStore.lastTaskKey)
	}
}

func TestRushInventoryPreheatTaskLogicHandleMarksFailedWhenPrimeRushRuntimeFails(t *testing.T) {
	expectedOpenTime := time.Date(2026, time.April, 14, 19, 30, 0, 0, time.Local)
	body, err := taskdef.Marshal(83004, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	showTimeStore := &fakeRushInventoryShowTimeStore{
		listResp: []*svc.ShowTimeRecord{
			{
				ID:               93005,
				ProgramID:        83004,
				RushSaleOpenTime: sql.NullTime{Time: expectedOpenTime, Valid: true},
			},
		},
	}
	logic := NewRushInventoryPreheatTaskLogic(&svc.WorkerServiceContext{
		ShowTimeStore: showTimeStore,
		OrderRpc:      &fakeRushInventoryOrderRPC{err: errors.New("runtime unavailable")},
		ProgramRpc:    &fakeRushInventoryProgramRPC{},
	})

	err = logic.Handle(context.Background(), asynq.NewTask(taskdef.TaskTypeRushInventoryPreheat, body))
	if err == nil {
		t.Fatalf("expected PrimeRushRuntime error")
	}
	if showTimeStore.failedCalls != 1 || showTimeStore.lastTaskKey != taskdef.TaskKey(83004, expectedOpenTime) {
		t.Fatalf("expected failed task update, calls=%d key=%s", showTimeStore.failedCalls, showTimeStore.lastTaskKey)
	}
	if showTimeStore.lastTaskErr == "" {
		t.Fatalf("expected last consume error to be recorded")
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

type captureWriter struct {
	buf *bytes.Buffer
}

func captureLogx(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := logx.Reset()
	logx.SetWriter(captureWriter{buf: &buf})
	t.Cleanup(func() {
		logx.Reset()
		if previous != nil {
			logx.SetWriter(previous)
		}
	})
	return &buf
}

func (w captureWriter) Alert(v any) {
	w.write(v, nil)
}

func (w captureWriter) Close() error {
	return nil
}

func (w captureWriter) Debug(v any, fields ...logx.LogField) {
	w.write(v, fields)
}

func (w captureWriter) Error(v any, fields ...logx.LogField) {
	w.write(v, fields)
}

func (w captureWriter) Info(v any, fields ...logx.LogField) {
	w.write(v, fields)
}

func (w captureWriter) Severe(v any) {
	w.write(v, nil)
}

func (w captureWriter) Slow(v any, fields ...logx.LogField) {
	w.write(v, fields)
}

func (w captureWriter) Stack(v any) {
	w.write(v, nil)
}

func (w captureWriter) Stat(v any, fields ...logx.LogField) {
	w.write(v, fields)
}

func (w captureWriter) write(v any, fields []logx.LogField) {
	w.buf.WriteString(fmt.Sprint(v))
	for _, field := range fields {
		w.buf.WriteString(fmt.Sprintf(" %s=%v", field.Key, field.Value))
	}
	w.buf.WriteByte('\n')
}
