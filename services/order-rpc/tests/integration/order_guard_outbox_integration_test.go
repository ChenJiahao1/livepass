package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	orderevent "damai-go/services/order-rpc/internal/event"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
)

func TestCreateOrderTransactionPersistsOrderAndGuards(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	orderNumber := sharding.BuildOrderNumber(3001, time.Date(2026, time.April, 5, 11, 0, 0, 0, time.UTC), 1, 1)
	event := testOrderCreateEvent(orderNumber, "2026-04-05 11:00:00", "2026-04-05 11:15:00")
	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}

	if err := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	route, err := svcCtx.OrderRepository.RouteByOrderNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("RouteByOrderNumber returned error: %v", err)
	}

	orderTable := "d_order_" + route.TableSuffix
	ticketTable := "d_order_ticket_user_" + route.TableSuffix
	userGuardTable := "d_order_user_guard_" + route.TableSuffix
	viewerGuardTable := "d_order_viewer_guard_" + route.TableSuffix
	seatGuardTable := "d_order_seat_guard_" + route.TableSuffix
	outboxTable := "d_order_outbox_" + route.TableSuffix

	requireOrderCoreFields(t, svcCtx.Config.MySQL.DataSource, orderTable, orderNumber, 10001, 3001)
	requireTicketUsersAndSeats(t, svcCtx.Config.MySQL.DataSource, ticketTable, orderNumber, []int64{701, 702}, []int64{501, 502})
	requireUserGuard(t, svcCtx.Config.MySQL.DataSource, userGuardTable, orderNumber, 10001, 3001)
	requireViewerGuards(t, svcCtx.Config.MySQL.DataSource, viewerGuardTable, orderNumber, 10001, []int64{701, 702})
	requireSeatGuards(t, svcCtx.Config.MySQL.DataSource, seatGuardTable, orderNumber, 10001, []int64{501, 502})
	requireOutboxEvent(t, svcCtx.Config.MySQL.DataSource, outboxTable, orderNumber, "order.created")
}

func TestCloseExpiredOrderDeletesGuardsAndWritesOutbox(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	orderNumber := sharding.BuildOrderNumber(3001, time.Date(2026, time.April, 5, 11, 5, 0, 0, time.UTC), 1, 2)
	event := testOrderCreateEvent(orderNumber, "2026-04-05 11:05:00", "2026-01-01 00:00:00")
	body, err := event.Marshal()
	if err != nil {
		t.Fatalf("event.Marshal returned error: %v", err)
	}
	if err := logicpkg.NewCreateOrderConsumerLogic(context.Background(), svcCtx).Consume(body); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	route, err := svcCtx.OrderRepository.RouteByOrderNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("RouteByOrderNumber returned error: %v", err)
	}
	userGuardTable := "d_order_user_guard_" + route.TableSuffix
	viewerGuardTable := "d_order_viewer_guard_" + route.TableSuffix
	seatGuardTable := "d_order_seat_guard_" + route.TableSuffix
	outboxTable := "d_order_outbox_" + route.TableSuffix
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
	requireOutboxEvent(t, svcCtx.Config.MySQL.DataSource, outboxTable, orderNumber, "order.closed")
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

func requireOrderCoreFields(t *testing.T, dataSource, table string, orderNumber, programID, userID int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var (
		gotOrderNumber int64
		gotProgramID   int64
		gotUserID      int64
	)
	err := db.QueryRow(
		"SELECT order_number, program_id, user_id FROM "+table+" WHERE order_number = ? LIMIT 1",
		orderNumber,
	).Scan(&gotOrderNumber, &gotProgramID, &gotUserID)
	if err != nil {
		t.Fatalf("query order core fields error: %v", err)
	}
	if gotOrderNumber != orderNumber || gotProgramID != programID || gotUserID != userID {
		t.Fatalf("order core fields mismatch, got=(%d,%d,%d)", gotOrderNumber, gotProgramID, gotUserID)
	}
}

func requireTicketUsersAndSeats(t *testing.T, dataSource, table string, orderNumber int64, expectedViewerIDs, expectedSeatIDs []int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	rows, err := db.Query(
		"SELECT ticket_user_id, seat_id FROM "+table+" WHERE order_number = ? ORDER BY ticket_user_id ASC",
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
		if scanErr := rows.Scan(&viewerID, &seatID); scanErr != nil {
			t.Fatalf("scan ticket row error: %v", scanErr)
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

func requireUserGuard(t *testing.T, dataSource, table string, orderNumber, programID, userID int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var (
		gotOrderNumber int64
		gotProgramID   int64
		gotUserID      int64
	)
	err := db.QueryRow(
		"SELECT order_number, program_id, user_id FROM "+table+" WHERE order_number = ? LIMIT 1",
		orderNumber,
	).Scan(&gotOrderNumber, &gotProgramID, &gotUserID)
	if err != nil {
		t.Fatalf("query user guard error: %v", err)
	}
	if gotOrderNumber != orderNumber || gotProgramID != programID || gotUserID != userID {
		t.Fatalf("user guard mismatch, got=(%d,%d,%d)", gotOrderNumber, gotProgramID, gotUserID)
	}
}

func requireViewerGuards(t *testing.T, dataSource, table string, orderNumber, programID int64, expectedViewerIDs []int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	rows, err := db.Query(
		"SELECT viewer_id, program_id FROM "+table+" WHERE order_number = ? ORDER BY viewer_id ASC",
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
		if scanErr := rows.Scan(&viewerID, &gotProgramID); scanErr != nil {
			t.Fatalf("scan viewer guard error: %v", scanErr)
		}
		if gotProgramID != programID {
			t.Fatalf("viewer guard program_id = %d, want %d", gotProgramID, programID)
		}
		gotViewerIDs = append(gotViewerIDs, viewerID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate viewer guards error: %v", err)
	}

	requireInt64SlicesEqual(t, gotViewerIDs, expectedViewerIDs)
}

func requireSeatGuards(t *testing.T, dataSource, table string, orderNumber, programID int64, expectedSeatIDs []int64) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	rows, err := db.Query(
		"SELECT seat_id, program_id FROM "+table+" WHERE order_number = ? ORDER BY seat_id ASC",
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
		if scanErr := rows.Scan(&seatID, &gotProgramID); scanErr != nil {
			t.Fatalf("scan seat guard error: %v", scanErr)
		}
		if gotProgramID != programID {
			t.Fatalf("seat guard program_id = %d, want %d", gotProgramID, programID)
		}
		gotSeatIDs = append(gotSeatIDs, seatID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate seat guards error: %v", err)
	}

	requireInt64SlicesEqual(t, gotSeatIDs, expectedSeatIDs)
}

func requireOutboxEvent(t *testing.T, dataSource, table string, orderNumber int64, expectedEventType string) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var (
		gotEventType    string
		payload         string
		publishedStatus int64
	)
	err := db.QueryRow(
		"SELECT event_type, payload, published_status FROM "+table+" WHERE order_number = ? AND event_type = ? LIMIT 1",
		orderNumber,
		expectedEventType,
	).Scan(&gotEventType, &payload, &publishedStatus)
	if err != nil {
		t.Fatalf("query outbox event error: %v", err)
	}
	if gotEventType != expectedEventType {
		t.Fatalf("outbox event_type = %s, want %s", gotEventType, expectedEventType)
	}
	if publishedStatus != 0 {
		t.Fatalf("published_status = %d, want 0", publishedStatus)
	}

	var decoded map[string]int64
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode outbox payload error: %v", err)
	}
	if decoded["orderNumber"] != orderNumber || decoded["programId"] != 10001 || decoded["userId"] != 3001 {
		t.Fatalf(
			"unexpected outbox payload, got orderNumber=%d programId=%d userId=%d",
			decoded["orderNumber"],
			decoded["programId"],
			decoded["userId"],
		)
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
