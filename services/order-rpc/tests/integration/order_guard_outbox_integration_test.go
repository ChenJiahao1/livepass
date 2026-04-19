package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	orderevent "livepass/services/order-rpc/internal/event"
	logicpkg "livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	"livepass/services/order-rpc/sharding"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"
)

func TestCreateOrderTransactionPersistsOrderAndGuards(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	rebindOrderTestAttemptStore(t, svcCtx)

	orderNumber := sharding.BuildOrderNumber(3001, time.Date(2026, time.April, 5, 11, 0, 0, 0, time.UTC), 1, 1)
	event := testOrderCreateEvent(orderNumber, "2026-04-05 11:00:00", "2026-04-05 11:15:00")
	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	admitTestOrderCreateEvent(t, svcCtx, event)
	if err := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	route, err := svcCtx.OrderRepository.RouteByOrderNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("RouteByOrderNumber returned error: %v", err)
	}

	orderTable := "d_order_" + route.TableSuffix
	ticketTable := "d_order_ticket_user_" + route.TableSuffix
	userGuardTable := "d_order_user_guard"
	viewerGuardTable := "d_order_viewer_guard"
	seatGuardTable := "d_order_seat_guard"
	requireOrderCoreFields(t, svcCtx.Config.MySQL.DataSource, orderTable, orderNumber, 10001, 10001, 3001)
	requireTicketUsersAndSeats(t, svcCtx.Config.MySQL.DataSource, ticketTable, orderNumber, 10001, []int64{701, 702}, []int64{501, 502})
	requireUserGuard(t, svcCtx.Config.MySQL.DataSource, userGuardTable, orderNumber, 10001, 10001, 3001)
	requireViewerGuards(t, svcCtx.Config.MySQL.DataSource, viewerGuardTable, orderNumber, 10001, 10001, []int64{701, 702})
	requireSeatGuards(t, svcCtx.Config.MySQL.DataSource, seatGuardTable, orderNumber, 10001, 10001, []int64{501, 502})
}

func TestCloseExpiredOrderDeletesGuards(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	rebindOrderTestAttemptStore(t, svcCtx)

	orderNumber := sharding.BuildOrderNumber(3001, time.Date(2026, time.April, 5, 11, 5, 0, 0, time.UTC), 1, 2)
	event := testOrderCreateEvent(orderNumber, "2026-04-05 11:05:00", "2026-01-01 00:00:00")
	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}
	admitTestOrderCreateEvent(t, svcCtx, event)
	if err := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	route, err := svcCtx.OrderRepository.RouteByOrderNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("RouteByOrderNumber returned error: %v", err)
	}
	userGuardTable := "d_order_user_guard"
	viewerGuardTable := "d_order_viewer_guard"
	seatGuardTable := "d_order_seat_guard"
	orderTable := "d_order_" + route.TableSuffix

	closeResp, err := logicpkg.NewCloseExpiredOrderLogic(context.Background(), svcCtx).CloseExpiredOrder(&pb.CloseExpiredOrderReq{
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("CloseExpiredOrder returned error: %v", err)
	}
	if !closeResp.GetSuccess() {
		t.Fatalf("expected close expired order success")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected release freeze called once, got %d", programRPC.releaseSeatFreezeCalls)
	}

	if got := findOrderStatusFromTable(t, svcCtx.Config.MySQL.DataSource, orderTable, orderNumber); got != testOrderStatusCancelled {
		t.Fatalf("order status = %d, want %d", got, testOrderStatusCancelled)
	}
	requireOrderGuardDeleted(t, svcCtx.Config.MySQL.DataSource, userGuardTable, orderNumber)
	requireOrderGuardDeleted(t, svcCtx.Config.MySQL.DataSource, viewerGuardTable, orderNumber)
	requireOrderGuardDeleted(t, svcCtx.Config.MySQL.DataSource, seatGuardTable, orderNumber)
}

func TestCreateOrderWritesDelayTaskOutbox(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	rebindOrderTestAttemptStore(t, svcCtx)

	orderNumber := sharding.BuildOrderNumber(3001, time.Date(2026, time.April, 5, 11, 10, 0, 0, time.UTC), 1, 3)
	event := testOrderCreateEvent(orderNumber, "2026-04-05 11:10:00", "2026-04-05 11:25:00")
	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	admitTestOrderCreateEvent(t, svcCtx, event)
	if err := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	requireDelayTaskOutbox(t, svcCtx.Config.MySQL.DataSource, "d_delay_task_outbox", orderNumber, "2026-04-05 11:25:00")
}

func TestCreateOrderGuardsRejectDuplicateSeatAcrossOrderSuffixes(t *testing.T) {
	svcCtx, programRPC, userRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	rebindOrderTestAttemptStore(t, svcCtx)

	firstUserID := mustFindOrderTestUserIDByLogicSlot(t, 10)
	secondUserID := mustFindOrderTestUserIDByLogicSlot(t, 11)
	firstRoute := orderRouteForUser(t, svcCtx, firstUserID)
	secondRoute := orderRouteForUser(t, svcCtx, secondUserID)
	if firstRoute.TableSuffix == secondRoute.TableSuffix {
		t.Fatalf("expected different order suffixes, got %s and %s", firstRoute.TableSuffix, secondRoute.TableSuffix)
	}

	firstOrderNumber := sharding.BuildOrderNumber(firstUserID, time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC), 1, 11)
	secondOrderNumber := sharding.BuildOrderNumber(secondUserID, time.Date(2026, time.April, 5, 12, 0, 1, 0, time.UTC), 1, 12)
	programID := int64(10001)
	ticketCategoryID := int64(40001)

	firstEvent := buildTestOrderCreateEvent(firstOrderNumber, firstUserID, []int64{701}, []int64{601})
	admitTestOrderCreateEvent(t, svcCtx, firstEvent)
	if err := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(mustMarshalOrderCreateEvent(t, firstEvent)); err != nil {
		t.Fatalf("first Consume returned error: %v", err)
	}

	ctx := context.Background()
	producer, ok := svcCtx.OrderCreateProducer.(*fakeOrderCreateProducer)
	if !ok {
		t.Fatalf("expected fake order create producer, got %T", svcCtx.OrderCreateProducer)
	}
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, programID, ticketCategoryID, 1); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(programID, ticketCategoryID, 1, 4, 299)
	userRPC.getUserAndTicketUserListResp = buildTestUserAndTicketUsers(
		secondUserID,
		&userrpc.TicketUserInfo{Id: 801, UserId: secondUserID, RelName: "观演人-801", IdType: 1, IdNumber: "110101199001010801"},
	)
	programRPC.autoAssignAndFreezeSeatsResp = &programrpc.AutoAssignAndFreezeSeatsResp{
		FreezeToken: "freeze-guard-conflict-second-order",
		ExpireTime:  "2026-12-31 19:45:00",
		Seats: []*programrpc.SeatInfo{
			{SeatId: 601, TicketCategoryId: ticketCategoryID, RowCode: 1, ColCode: 1, Price: 299},
		},
	}

	claims := rush.PurchaseTokenClaims{
		OrderNumber:      secondOrderNumber,
		UserID:           secondUserID,
		ProgramID:        programID,
		ShowTimeID:       programID,
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    []int64{801},
		TicketCount:      1,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		ExpireAt:         time.Now().Add(2 * time.Minute).Unix(),
	}
	resp, err := logicpkg.NewCreateOrderLogic(ctx, svcCtx).CreateOrder(&pb.CreateOrderReq{
		UserId:        secondUserID,
		PurchaseToken: mustIssueRushPurchaseToken(t, svcCtx, claims),
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	waitOrderCreateSendCalls(t, producer, 1)
	if err := logicpkg.NewCreateOrderConsumerLogic(ctx, svcCtx).Consume(producer.lastBody); err != nil {
		t.Fatalf("second Consume should convert guard conflict to business failure, got error: %v", err)
	}

	if _, findErr := svcCtx.OrderRepository.FindOrderByNumber(ctx, secondOrderNumber); !errors.Is(findErr, model.ErrNotFound) {
		t.Fatalf("expected second order insert rolled back, got err=%v", findErr)
	}
	record, err := svcCtx.AttemptStore.Get(ctx, resp.GetOrderNumber())
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateFailed || record.ReasonCode != rush.AttemptReasonAlreadyHasActiveOrder {
		t.Fatalf("expected failed attempt with ALREADY_HAS_ACTIVE_ORDER, got %+v", record)
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected guard conflict release seat freeze once, got %d", programRPC.releaseSeatFreezeCalls)
	}
	if programRPC.lastReleaseSeatFreezeReq == nil || programRPC.lastReleaseSeatFreezeReq.GetFreezeToken() != "freeze-guard-conflict-second-order" {
		t.Fatalf("expected release freeze token freeze-guard-conflict-second-order, got %+v", programRPC.lastReleaseSeatFreezeReq)
	}
}

func testOrderCreateEvent(orderNumber int64, occurredAt, freezeExpireTime string) *orderevent.OrderCreateEvent {
	return &orderevent.OrderCreateEvent{
		EventID:          fmt.Sprintf("evt-%d", orderNumber),
		Version:          orderevent.OrderCreateEventVersion,
		OrderNumber:      orderNumber,
		RequestNo:        fmt.Sprintf("req-%d", orderNumber),
		OccurredAt:       occurredAt,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       10001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701, 702},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		FreezeToken:      fmt.Sprintf("freeze-%d", orderNumber),
		FreezeExpireTime: freezeExpireTime,
		ProgramSnapshot: orderevent.ProgramSnapshot{
			Title:            "测试演出",
			ItemPicture:      "https://example.com/program.jpg",
			Place:            "测试场馆",
			ShowTime:         "2026-12-31 19:30:00",
			PermitChooseSeat: 0,
		},
		TicketCategorySnapshot: orderevent.TicketCategorySnapshot{
			ID:    40001,
			Name:  "普通票",
			Price: 299,
		},
		TicketUserSnapshot: []orderevent.TicketUserSnapshot{
			{TicketUserID: 701, Name: "张三", IDNumber: "110101199001011234"},
			{TicketUserID: 702, Name: "李四", IDNumber: "110101199002021234"},
		},
		SeatSnapshot: []orderevent.SeatSnapshot{
			{SeatID: 501, RowCode: 1, ColCode: 1, Price: 299},
			{SeatID: 502, RowCode: 1, ColCode: 2, Price: 299},
		},
	}
}

func buildTestOrderCreateEvent(orderNumber, userID int64, ticketUserIDs, seatIDs []int64) *orderevent.OrderCreateEvent {
	ticketSnapshots := make([]orderevent.TicketUserSnapshot, 0, len(ticketUserIDs))
	seatSnapshots := make([]orderevent.SeatSnapshot, 0, len(seatIDs))
	for idx, ticketUserID := range ticketUserIDs {
		ticketSnapshots = append(ticketSnapshots, orderevent.TicketUserSnapshot{
			TicketUserID: ticketUserID,
			Name:         fmt.Sprintf("观演人-%d", ticketUserID),
			IDNumber:     fmt.Sprintf("11010119900101%04d", ticketUserID),
		})
		seatSnapshots = append(seatSnapshots, orderevent.SeatSnapshot{
			SeatID:  seatIDs[idx],
			RowCode: 1,
			ColCode: int64(idx + 1),
			Price:   299,
		})
	}

	return &orderevent.OrderCreateEvent{
		EventID:          fmt.Sprintf("evt-%d", orderNumber),
		Version:          orderevent.OrderCreateEventVersion,
		OrderNumber:      orderNumber,
		RequestNo:        fmt.Sprintf("req-%d", orderNumber),
		OccurredAt:       "2026-04-05 12:00:00",
		UserID:           userID,
		ProgramID:        10001,
		ShowTimeID:       10001,
		TicketCategoryID: 40001,
		TicketUserIDs:    ticketUserIDs,
		DistributionMode: "express",
		TakeTicketMode:   "paper",
		FreezeToken:      fmt.Sprintf("freeze-%d", orderNumber),
		FreezeExpireTime: "2026-04-05 12:15:00",
		ProgramSnapshot: orderevent.ProgramSnapshot{
			Title:            "测试演出",
			ItemPicture:      "https://example.com/program.jpg",
			Place:            "测试场馆",
			ShowTime:         "2026-12-31 19:30:00",
			PermitChooseSeat: 0,
		},
		TicketCategorySnapshot: orderevent.TicketCategorySnapshot{
			ID:    40001,
			Name:  "普通票",
			Price: 299,
		},
		TicketUserSnapshot: ticketSnapshots,
		SeatSnapshot:       seatSnapshots,
	}
}

func mustMarshalOrderCreateEvent(t *testing.T, event *orderevent.OrderCreateEvent) []byte {
	t.Helper()

	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	return body
}

func admitTestOrderCreateEvent(t *testing.T, svcCtx *svc.ServiceContext, event *orderevent.OrderCreateEvent) {
	t.Helper()
	if svcCtx == nil || svcCtx.AttemptStore == nil || event == nil {
		t.Fatalf("expected service context, attempt store and event")
	}

	showTimeID := event.ShowTimeID
	if showTimeID <= 0 {
		showTimeID = event.ProgramID
	}
	viewerIDs := append([]int64(nil), event.TicketUserIDs...)
	if len(viewerIDs) == 0 {
		for _, snapshot := range event.TicketUserSnapshot {
			viewerIDs = append(viewerIDs, snapshot.TicketUserID)
		}
	}
	ticketCount := event.TicketCount
	if ticketCount <= 0 {
		ticketCount = int64(len(viewerIDs))
	}

	ctx := context.Background()
	if err := svcCtx.AttemptStore.SetQuotaAvailable(ctx, showTimeID, event.TicketCategoryID, ticketCount); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := svcCtx.AttemptStore.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      event.OrderNumber,
		UserID:           event.UserID,
		ProgramID:        event.ProgramID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: event.TicketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      ticketCount,
		Now:              time.Now(),
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
}

func requireOrderCoreFields(t *testing.T, dataSource, table string, orderNumber, programID, showTimeID, userID int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var (
		gotOrderNumber int64
		gotProgramID   int64
		gotShowTimeID  int64
		gotUserID      int64
	)
	err := db.QueryRow(
		"SELECT order_number, program_id, show_time_id, user_id FROM "+table+" WHERE order_number = ? LIMIT 1",
		orderNumber,
	).Scan(&gotOrderNumber, &gotProgramID, &gotShowTimeID, &gotUserID)
	if err != nil {
		t.Fatalf("query order core fields error: %v", err)
	}
	if gotOrderNumber != orderNumber || gotProgramID != programID || gotShowTimeID != showTimeID || gotUserID != userID {
		t.Fatalf("order core fields mismatch, got=(%d,%d,%d,%d)", gotOrderNumber, gotProgramID, gotShowTimeID, gotUserID)
	}
}

func requireTicketUsersAndSeats(t *testing.T, dataSource, table string, orderNumber, showTimeID int64, expectedViewerIDs, expectedSeatIDs []int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	rows, err := db.Query(
		"SELECT ticket_user_id, seat_id, show_time_id FROM "+table+" WHERE order_number = ? ORDER BY ticket_user_id ASC",
		orderNumber,
	)
	if err != nil {
		t.Fatalf("query ticket rows error: %v", err)
	}
	defer rows.Close()

	var gotViewerIDs []int64
	var gotSeatIDs []int64
	for rows.Next() {
		var viewerID int64
		var seatID int64
		var gotShowTimeID int64
		if scanErr := rows.Scan(&viewerID, &seatID, &gotShowTimeID); scanErr != nil {
			t.Fatalf("scan ticket row error: %v", scanErr)
		}
		if gotShowTimeID != showTimeID {
			t.Fatalf("ticket row show_time_id = %d, want %d", gotShowTimeID, showTimeID)
		}
		gotViewerIDs = append(gotViewerIDs, viewerID)
		gotSeatIDs = append(gotSeatIDs, seatID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate ticket rows error: %v", err)
	}

	requireInt64SlicesEqual(t, gotViewerIDs, expectedViewerIDs)
	requireInt64SlicesEqual(t, gotSeatIDs, expectedSeatIDs)
}

func requireUserGuard(t *testing.T, dataSource, table string, orderNumber, programID, showTimeID, userID int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var (
		gotOrderNumber int64
		gotProgramID   int64
		gotShowTimeID  int64
		gotUserID      int64
	)
	err := db.QueryRow(
		"SELECT order_number, program_id, show_time_id, user_id FROM "+table+" WHERE order_number = ? LIMIT 1",
		orderNumber,
	).Scan(&gotOrderNumber, &gotProgramID, &gotShowTimeID, &gotUserID)
	if err != nil {
		t.Fatalf("query user guard error: %v", err)
	}
	if gotOrderNumber != orderNumber || gotProgramID != programID || gotShowTimeID != showTimeID || gotUserID != userID {
		t.Fatalf("user guard mismatch, got=(%d,%d,%d,%d)", gotOrderNumber, gotProgramID, gotShowTimeID, gotUserID)
	}
}

func requireViewerGuards(t *testing.T, dataSource, table string, orderNumber, programID, showTimeID int64, expectedViewerIDs []int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	rows, err := db.Query(
		"SELECT viewer_id, program_id, show_time_id FROM "+table+" WHERE order_number = ? ORDER BY viewer_id ASC",
		orderNumber,
	)
	if err != nil {
		t.Fatalf("query viewer guards error: %v", err)
	}
	defer rows.Close()

	var gotViewerIDs []int64
	for rows.Next() {
		var viewerID int64
		var gotProgramID int64
		var gotShowTimeID int64
		if scanErr := rows.Scan(&viewerID, &gotProgramID, &gotShowTimeID); scanErr != nil {
			t.Fatalf("scan viewer guard error: %v", scanErr)
		}
		if gotProgramID != programID {
			t.Fatalf("viewer guard program_id = %d, want %d", gotProgramID, programID)
		}
		if gotShowTimeID != showTimeID {
			t.Fatalf("viewer guard show_time_id = %d, want %d", gotShowTimeID, showTimeID)
		}
		gotViewerIDs = append(gotViewerIDs, viewerID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate viewer guards error: %v", err)
	}

	requireInt64SlicesEqual(t, gotViewerIDs, expectedViewerIDs)
}

func requireSeatGuards(t *testing.T, dataSource, table string, orderNumber, programID, showTimeID int64, expectedSeatIDs []int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	rows, err := db.Query(
		"SELECT seat_id, program_id, show_time_id FROM "+table+" WHERE order_number = ? ORDER BY seat_id ASC",
		orderNumber,
	)
	if err != nil {
		t.Fatalf("query seat guards error: %v", err)
	}
	defer rows.Close()

	var gotSeatIDs []int64
	for rows.Next() {
		var seatID int64
		var gotProgramID int64
		var gotShowTimeID int64
		if scanErr := rows.Scan(&seatID, &gotProgramID, &gotShowTimeID); scanErr != nil {
			t.Fatalf("scan seat guard error: %v", scanErr)
		}
		if gotProgramID != programID {
			t.Fatalf("seat guard program_id = %d, want %d", gotProgramID, programID)
		}
		if gotShowTimeID != showTimeID {
			t.Fatalf("seat guard show_time_id = %d, want %d", gotShowTimeID, showTimeID)
		}
		gotSeatIDs = append(gotSeatIDs, seatID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate seat guards error: %v", err)
	}

	requireInt64SlicesEqual(t, gotSeatIDs, expectedSeatIDs)
}

func requireDelayTaskOutbox(t *testing.T, dataSource, table string, orderNumber int64, executeAt string) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var (
		taskType        string
		taskKey         string
		payload         string
		gotExecuteAt    time.Time
		publishedStatus int64
	)
	err := db.QueryRow(
		"SELECT task_type, task_key, payload, execute_at, published_status FROM "+table+" WHERE task_key = ? LIMIT 1",
		fmt.Sprintf("order.close_timeout:%d", orderNumber),
	).Scan(&taskType, &taskKey, &payload, &gotExecuteAt, &publishedStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("delay task outbox row not found for orderNumber=%d", orderNumber)
		}
		t.Fatalf("query delay task outbox error: %v", err)
	}

	if taskType != "order.close_timeout" {
		t.Fatalf("task_type = %s, want order.close_timeout", taskType)
	}
	if taskKey != fmt.Sprintf("order.close_timeout:%d", orderNumber) {
		t.Fatalf("task_key = %s, want order.close_timeout:%d", taskKey, orderNumber)
	}
	if publishedStatus != 0 {
		t.Fatalf("published_status = %d, want 0", publishedStatus)
	}

	var decoded map[string]int64
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode delay task payload error: %v", err)
	}
	if decoded["orderNumber"] != orderNumber {
		t.Fatalf("unexpected delay task payload orderNumber=%d", decoded["orderNumber"])
	}

	expectedExecuteAt, err := time.ParseInLocation("2006-01-02 15:04:05", executeAt, time.Local)
	if err != nil {
		t.Fatalf("parse executeAt error: %v", err)
	}
	if !gotExecuteAt.Equal(expectedExecuteAt) {
		t.Fatalf("execute_at = %s, want %s", gotExecuteAt.Format("2006-01-02 15:04:05"), expectedExecuteAt.Format("2006-01-02 15:04:05"))
	}
}

func requireOrderGuardDeleted(t *testing.T, dataSource, table string, orderNumber int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var count int64
	err := db.QueryRow("SELECT COUNT(1) FROM "+table+" WHERE order_number = ?", orderNumber).Scan(&count)
	if err != nil {
		t.Fatalf("query guard count error: %v", err)
	}
	if count != 0 {
		t.Fatalf("guard rows for order %d in table %s = %d, want 0", orderNumber, table, count)
	}
}

func requireInt64SlicesEqual(t *testing.T, got, want []int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("slice length mismatch, got=%v want=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice mismatch at %d, got=%v want=%v", i, got, want)
		}
	}
}
