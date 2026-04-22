package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	closeTaskDef "livepass/jobs/order-close/taskdef"
	"livepass/pkg/delaytask"
	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	"livepass/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/logx"
)

func TestCloseExpiredOrderClosesOnlyExpiredUnpaidOrder(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8201,
			OrderNumber:     92001,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-one-001",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8921, OrderNumber: 92001, UserID: 3001, TicketUserID: 701, SeatID: 511, SeatRow: 1, SeatCol: 1},
	)
	seedCloseTimeoutOutbox(t, svcCtx, 92001)

	l := logicpkg.NewCloseExpiredOrderLogic(context.Background(), svcCtx)
	resp, err := l.CloseExpiredOrder(&pb.CloseExpiredOrderReq{OrderNumber: 92001})
	if err != nil {
		t.Fatalf("CloseExpiredOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected close expired order success")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 92001) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
	outbox := findCloseTimeoutOutboxState(t, svcCtx, 92001)
	if outbox.TaskStatus != delaytask.OutboxTaskStatusProcessed || outbox.ConsumeAttempts != 1 || !outbox.ProcessedTime.Valid {
		t.Fatalf("outbox = %+v, want processed with one consume attempt", outbox)
	}
}

func TestCloseExpiredOrderMarksNoopOutboxProcessed(t *testing.T) {
	cases := []struct {
		name        string
		orderNumber int64
		status      int64
		seedOrder   bool
	}{
		{name: "cancelled", orderNumber: 92011, status: testOrderStatusCancelled, seedOrder: true},
		{name: "paid", orderNumber: 92012, status: testOrderStatusPaid, seedOrder: true},
		{name: "missing", orderNumber: 92013, seedOrder: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
			resetOrderDomainState(t)

			if tc.seedOrder {
				seedOrderFixtures(
					t,
					svcCtx,
					orderFixture{
						ID:              tc.orderNumber,
						OrderNumber:     tc.orderNumber,
						ProgramID:       10001,
						UserID:          3001,
						OrderStatus:     tc.status,
						FreezeToken:     fmt.Sprintf("freeze-noop-%d", tc.orderNumber),
						OrderExpireTime: "2026-01-01 00:00:00",
					},
				)
			}
			seedCloseTimeoutOutbox(t, svcCtx, tc.orderNumber)

			resp, err := logicpkg.NewCloseExpiredOrderLogic(context.Background(), svcCtx).CloseExpiredOrder(&pb.CloseExpiredOrderReq{
				OrderNumber: tc.orderNumber,
			})
			if err != nil {
				t.Fatalf("CloseExpiredOrder returned error: %v", err)
			}
			if !resp.GetSuccess() {
				t.Fatalf("expected success")
			}
			if programRPC.releaseSeatFreezeCalls != 0 {
				t.Fatalf("expected no release call, got %d", programRPC.releaseSeatFreezeCalls)
			}

			outbox := findCloseTimeoutOutboxState(t, svcCtx, tc.orderNumber)
			if outbox.TaskStatus != delaytask.OutboxTaskStatusProcessed || outbox.ConsumeAttempts != 1 {
				t.Fatalf("outbox = %+v, want processed with one consume attempt", outbox)
			}
		})
	}
}

func TestCloseExpiredOrderKeepsProcessedWhenReleaseSeatFreezeFails(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	programRPC.releaseSeatFreezeErr = fmt.Errorf("program rpc unavailable")

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8211,
			OrderNumber:     92021,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-release-fail",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8931, OrderNumber: 92021, UserID: 3001, TicketUserID: 701, SeatID: 521, SeatRow: 1, SeatCol: 1},
	)
	seedCloseTimeoutOutbox(t, svcCtx, 92021)

	resp, err := logicpkg.NewCloseExpiredOrderLogic(context.Background(), svcCtx).CloseExpiredOrder(&pb.CloseExpiredOrderReq{OrderNumber: 92021})
	if err != nil {
		t.Fatalf("CloseExpiredOrder returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 92021) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed despite release failure")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
	outbox := findCloseTimeoutOutboxState(t, svcCtx, 92021)
	if outbox.TaskStatus != delaytask.OutboxTaskStatusProcessed || outbox.ConsumeAttempts != 1 {
		t.Fatalf("outbox = %+v, want processed with one consume attempt", outbox)
	}
}

func TestCloseExpiredOrderLogsConsumeLifecycleWithActualAttempts(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8212,
			OrderNumber:     92022,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-consume-log",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedCloseTimeoutOutbox(t, svcCtx, 92022)
	setCloseTimeoutConsumeAttempts(t, svcCtx, 92022, 3)

	logs := captureOrderLogx(t)
	resp, err := logicpkg.NewCloseExpiredOrderLogic(context.Background(), svcCtx).CloseExpiredOrder(&pb.CloseExpiredOrderReq{OrderNumber: 92022})
	if err != nil {
		t.Fatalf("CloseExpiredOrder returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success")
	}
	if !strings.Contains(logs.String(), "delay_task_consume_state_transition") ||
		!strings.Contains(logs.String(), "task_type=order.close_timeout") ||
		!strings.Contains(logs.String(), "from_status=1") ||
		!strings.Contains(logs.String(), "to_status=3") ||
		!strings.Contains(logs.String(), "consume_attempts=4") {
		t.Fatalf("unexpected consume lifecycle logs: %s", logs.String())
	}
}

func TestCloseExpiredOrderFinalizesCommittedAttemptAsClosedReleased(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	store := rebindOrderTestAttemptStore(t, svcCtx)
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, _ := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := sharding.BuildOrderNumber(userID, now, 1, 3)

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	record, shouldProcess, err := store.PrepareAttemptForConsume(ctx, programID, orderNumber, now.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("PrepareAttemptForConsume() error = %v", err)
	}
	if !shouldProcess || record == nil {
		t.Fatalf("expected claim processing success, got shouldProcess=%t record=%+v", shouldProcess, record)
	}
	if err := store.FinalizeSuccess(ctx, record, now.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("FinalizeSuccess() error = %v", err)
	}

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8302,
			OrderNumber:     orderNumber,
			ProgramID:       programID,
			UserID:          userID,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-rush-attempt",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8922, OrderNumber: orderNumber, UserID: userID, TicketUserID: viewerIDs[0], SeatID: 512, SeatRow: 1, SeatCol: 1},
	)

	resp, err := logicpkg.NewCloseExpiredOrderLogic(ctx, svcCtx).CloseExpiredOrder(&pb.CloseExpiredOrderReq{
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("CloseExpiredOrder() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected close expired order success")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, orderNumber) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed")
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() after close error = %v", err)
	}
	if record.State != rush.AttemptStateFailed || record.ReasonCode != rush.AttemptReasonClosedOrderReleased {
		t.Fatalf("expected closed order to release rush attempt projection, got %+v", record)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected closed committed attempt to release quota exactly once, got ok=%t available=%d", ok, available)
	}
}

type closeTimeoutOutboxState struct {
	TaskStatus      int64
	ConsumeAttempts int64
	ProcessedTime   sql.NullTime
}

func seedCloseTimeoutOutbox(t *testing.T, svcCtx *svc.ServiceContext, orderNumber int64) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	now := time.Now()
	_, err := db.Exec(
		`INSERT INTO d_delay_task_outbox (
			id, task_type, task_key, payload, execute_at, task_status, publish_attempts,
			consume_attempts, last_publish_error, last_consume_error, published_time,
			processed_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, 0, 0, '', '', NULL, NULL, ?, ?, 1)`,
		orderNumber,
		closeTaskDef.TaskTypeCloseTimeout,
		closeTaskDef.TaskKey(orderNumber),
		fmt.Sprintf(`{"orderNumber":%d}`, orderNumber),
		now,
		delaytask.OutboxTaskStatusPublished,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert close timeout outbox error: %v", err)
	}
}

func findCloseTimeoutOutboxState(t *testing.T, svcCtx *svc.ServiceContext, orderNumber int64) closeTimeoutOutboxState {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	var state closeTimeoutOutboxState
	err := db.QueryRow(
		`SELECT task_status, consume_attempts, processed_time
		FROM d_delay_task_outbox
		WHERE task_type = ? AND task_key = ?
		LIMIT 1`,
		closeTaskDef.TaskTypeCloseTimeout,
		closeTaskDef.TaskKey(orderNumber),
	).Scan(&state.TaskStatus, &state.ConsumeAttempts, &state.ProcessedTime)
	if err != nil {
		t.Fatalf("query close timeout outbox error: %v", err)
	}
	return state
}

func setCloseTimeoutConsumeAttempts(t *testing.T, svcCtx *svc.ServiceContext, orderNumber int64, attempts int64) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	_, err := db.Exec(
		`UPDATE d_delay_task_outbox SET consume_attempts = ? WHERE task_type = ? AND task_key = ?`,
		attempts,
		closeTaskDef.TaskTypeCloseTimeout,
		closeTaskDef.TaskKey(orderNumber),
	)
	if err != nil {
		t.Fatalf("update close timeout consume attempts error: %v", err)
	}
}

type orderCaptureWriter struct {
	buf *bytes.Buffer
}

func captureOrderLogx(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := logx.Reset()
	logx.SetWriter(orderCaptureWriter{buf: &buf})
	t.Cleanup(func() {
		logx.Reset()
		if previous != nil {
			logx.SetWriter(previous)
		}
	})
	return &buf
}

func (w orderCaptureWriter) Alert(v any)                          { w.write(v, nil) }
func (w orderCaptureWriter) Close() error                         { return nil }
func (w orderCaptureWriter) Debug(v any, fields ...logx.LogField) { w.write(v, fields) }
func (w orderCaptureWriter) Error(v any, fields ...logx.LogField) { w.write(v, fields) }
func (w orderCaptureWriter) Info(v any, fields ...logx.LogField)  { w.write(v, fields) }
func (w orderCaptureWriter) Severe(v any)                         { w.write(v, nil) }
func (w orderCaptureWriter) Slow(v any, fields ...logx.LogField)  { w.write(v, fields) }
func (w orderCaptureWriter) Stack(v any)                          { w.write(v, nil) }
func (w orderCaptureWriter) Stat(v any, fields ...logx.LogField)  { w.write(v, fields) }

func (w orderCaptureWriter) write(v any, fields []logx.LogField) {
	w.buf.WriteString(fmt.Sprint(v))
	for _, field := range fields {
		w.buf.WriteString(fmt.Sprintf(" %s=%v", field.Key, field.Value))
	}
	w.buf.WriteByte('\n')
}
