package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
)

func TestCloseExpiredOrdersClosesOnlyExpiredUnpaidRows(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8001,
			OrderNumber:     91001,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-001",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
		orderFixture{
			ID:              8002,
			OrderNumber:     91002,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-002",
			OrderExpireTime: "2099-01-01 00:00:00",
		},
		orderFixture{
			ID:              8003,
			OrderNumber:     91003,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusCancelled,
			FreezeToken:     "freeze-close-003",
			OrderExpireTime: "2026-01-01 00:00:00",
			CancelOrderTime: "2026-01-01 00:10:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8801, OrderNumber: 91001, UserID: 3001, TicketUserID: 701, SeatID: 501, SeatRow: 1, SeatCol: 1},
		orderTicketUserFixture{ID: 8802, OrderNumber: 91002, UserID: 3001, TicketUserID: 702, SeatID: 502, SeatRow: 1, SeatCol: 2},
		orderTicketUserFixture{ID: 8803, OrderNumber: 91003, UserID: 3001, TicketUserID: 703, SeatID: 503, SeatRow: 1, SeatCol: 3, OrderStatus: testOrderStatusCancelled},
	)

	l := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), svcCtx)
	resp, err := l.CloseExpiredOrders(&pb.CloseExpiredOrdersReq{Limit: 10})
	if err != nil {
		t.Fatalf("CloseExpiredOrders returned error: %v", err)
	}
	if resp.ClosedCount != 1 {
		t.Fatalf("expected one closed order, got %d", resp.ClosedCount)
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 91001) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 91002) != testOrderStatusUnpaid {
		t.Fatalf("expected non-expired order unchanged")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCloseExpiredOrdersScansRequestedLogicSlots(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	setOrderTestRepositoryMode(t, svcCtx, sharding.MigrationModeShardOnly)

	slot10UserID := mustFindOrderTestUserIDByLogicSlot(t, 10)
	slot11UserID := mustFindOrderTestUserIDByLogicSlot(t, 11)
	slot12UserID := mustFindOrderTestUserIDByLogicSlot(t, 12)
	slot10Route := orderRouteForUser(t, svcCtx, slot10UserID)
	slot11Route := orderRouteForUser(t, svcCtx, slot11UserID)
	slot12Route := orderRouteForUser(t, svcCtx, slot12UserID)

	slot10OrderNumber := sharding.BuildOrderNumber(slot10UserID, time.Date(2026, time.March, 24, 11, 10, 0, 0, time.UTC), 1, 1)
	slot11OrderNumber := sharding.BuildOrderNumber(slot11UserID, time.Date(2026, time.March, 24, 11, 11, 0, 0, time.UTC), 1, 2)
	slot12OrderNumber := sharding.BuildOrderNumber(slot12UserID, time.Date(2026, time.March, 24, 11, 12, 0, 0, time.UTC), 1, 3)

	seedShardOrderFixtures(
		t,
		svcCtx,
		slot10Route,
		orderFixture{
			ID:              8101,
			OrderNumber:     slot10OrderNumber,
			ProgramID:       10001,
			UserID:          slot10UserID,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-slot-10",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedShardOrderFixtures(
		t,
		svcCtx,
		slot11Route,
		orderFixture{
			ID:              8102,
			OrderNumber:     slot11OrderNumber,
			ProgramID:       10001,
			UserID:          slot11UserID,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-slot-11",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedShardOrderFixtures(
		t,
		svcCtx,
		slot12Route,
		orderFixture{
			ID:              8103,
			OrderNumber:     slot12OrderNumber,
			ProgramID:       10001,
			UserID:          slot12UserID,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-slot-12",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedShardOrderTicketUserFixtures(
		t,
		svcCtx,
		slot10Route,
		orderTicketUserFixture{ID: 8901, OrderNumber: slot10OrderNumber, UserID: slot10UserID, TicketUserID: 801, SeatID: 601, SeatRow: 1, SeatCol: 1},
	)
	seedShardOrderTicketUserFixtures(
		t,
		svcCtx,
		slot11Route,
		orderTicketUserFixture{ID: 8902, OrderNumber: slot11OrderNumber, UserID: slot11UserID, TicketUserID: 802, SeatID: 602, SeatRow: 1, SeatCol: 2},
	)
	seedShardOrderTicketUserFixtures(
		t,
		svcCtx,
		slot12Route,
		orderTicketUserFixture{ID: 8903, OrderNumber: slot12OrderNumber, UserID: slot12UserID, TicketUserID: 803, SeatID: 603, SeatRow: 1, SeatCol: 3},
	)

	l := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), svcCtx)
	resp, err := l.CloseExpiredOrders(&pb.CloseExpiredOrdersReq{
		Limit:          10,
		LogicSlotStart: 10,
		LogicSlotCount: 2,
	})
	if err != nil {
		t.Fatalf("CloseExpiredOrders returned error: %v", err)
	}
	if resp.ClosedCount != 2 {
		t.Fatalf("expected two closed orders, got %d", resp.ClosedCount)
	}
	if findOrderStatusFromTable(t, svcCtx.Config.MySQL.DataSource, "d_order_"+slot10Route.TableSuffix, slot10OrderNumber) != testOrderStatusCancelled {
		t.Fatalf("expected slot 10 order closed")
	}
	if findOrderStatusFromTable(t, svcCtx.Config.MySQL.DataSource, "d_order_"+slot11Route.TableSuffix, slot11OrderNumber) != testOrderStatusCancelled {
		t.Fatalf("expected slot 11 order closed")
	}
	if findOrderStatusFromTable(t, svcCtx.Config.MySQL.DataSource, "d_order_"+slot12Route.TableSuffix, slot12OrderNumber) != testOrderStatusUnpaid {
		t.Fatalf("expected slot 12 order unchanged")
	}
	if programRPC.releaseSeatFreezeCalls != 2 {
		t.Fatalf("expected two release calls, got %d", programRPC.releaseSeatFreezeCalls)
	}
}
