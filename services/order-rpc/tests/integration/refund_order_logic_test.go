package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRefundOrder(t *testing.T) {
	t.Run("paid shard order refunds and updates shard snapshots", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)

		userID := int64(3001)
		showTimeID := int64(13001)
		orderNumber := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 10, 30, 0, 0, time.UTC), 1, 3)
		route := orderRouteForUser(t, svcCtx, userID)
		seedShardOrderFixtures(t, svcCtx, route, orderFixture{
			ID:              99000,
			OrderNumber:     orderNumber,
			ProgramID:       10001,
			ShowTimeID:      showTimeID,
			UserID:          userID,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-shard",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedShardOrderTicketUserFixtures(t, svcCtx, route,
			orderTicketUserFixture{ID: 99100, OrderNumber: orderNumber, UserID: userID, TicketUserID: 701, SeatID: 50101, OrderStatus: testOrderStatusPaid},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94000,
			OrderNumber: orderNumber,
			UserId:      userID,
			Amount:      299,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}
		payRPC.refundResp = &payrpc.RefundResp{
			RefundBillNo: 95000,
			OrderNumber:  orderNumber,
			RefundAmount: 239,
			PayStatus:    3,
			RefundTime:   "2026-12-31 19:10:00",
		}
		programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
			AllowRefund:   true,
			RefundPercent: 80,
			RefundAmount:  239,
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		resp, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      userID,
			OrderNumber: orderNumber,
			Reason:      "行程变更",
		})
		if err != nil {
			t.Fatalf("RefundOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded {
			t.Fatalf("unexpected refund response: %+v", resp)
		}
		if findOrderStatusFromTable(t, testOrderMySQLDataSource, "d_order_"+route.TableSuffix, orderNumber) != testOrderStatusRefunded {
			t.Fatalf("expected shard order status refunded")
		}

		listResp, err := logicpkg.NewListOrdersLogic(context.Background(), svcCtx).ListOrders(&pb.ListOrdersReq{
			UserId:      userID,
			PageNumber:  1,
			PageSize:    10,
			OrderStatus: testOrderStatusRefunded,
		})
		if err != nil {
			t.Fatalf("ListOrders returned error: %v", err)
		}
		if listResp.TotalSize != 1 || len(listResp.List) != 1 || listResp.List[0].OrderNumber != orderNumber {
			t.Fatalf("expected refunded shard list to return current order, got %+v", listResp)
		}
	})

	t.Run("paid order refunds and updates local snapshots", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		const showTimeID int64 = 13002
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              99001,
			OrderNumber:     93001,
			ProgramID:       10001,
			ShowTimeID:      showTimeID,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-success",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 99101, OrderNumber: 93001, UserID: 3001, TicketUserID: 701, SeatID: 50101, OrderStatus: testOrderStatusPaid},
			orderTicketUserFixture{ID: 99102, OrderNumber: 93001, UserID: 3001, TicketUserID: 702, SeatID: 50102, OrderStatus: testOrderStatusPaid},
		)
		seedRefundGuardFixtures(t, svcCtx.Config.MySQL.DataSource, 93001, 10001, showTimeID, 3001, []int64{701, 702}, []int64{50101, 50102})
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94001,
			OrderNumber: 93001,
			UserId:      3001,
			Amount:      598,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}
		payRPC.refundResp = &payrpc.RefundResp{
			RefundBillNo: 95001,
			OrderNumber:  93001,
			RefundAmount: 478,
			PayStatus:    3,
			RefundTime:   "2026-12-31 19:10:00",
		}
		programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
			AllowRefund:   true,
			RefundPercent: 80,
			RefundAmount:  478,
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		resp, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      3001,
			OrderNumber: 93001,
			Reason:      "行程变更",
		})
		if err != nil {
			t.Fatalf("RefundOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded || resp.RefundBillNo != 95001 || resp.RefundAmount != 478 || resp.RefundPercent != 80 {
			t.Fatalf("unexpected refund response: %+v", resp)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 93001) != testOrderStatusRefunded {
			t.Fatalf("expected order status refunded")
		}
		if findOrderTicketStatus(t, testOrderMySQLDataSource, 93001) != testOrderStatusRefunded {
			t.Fatalf("expected order ticket status refunded")
		}
		if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
			t.Fatalf("unexpected evaluate refund request: %+v", programRPC.lastEvaluateRefundRuleReq)
		}
		if programRPC.lastReleaseSoldSeatsReq == nil || programRPC.lastReleaseSoldSeatsReq.ShowTimeId != showTimeID || len(programRPC.lastReleaseSoldSeatsReq.SeatIds) != 2 || programRPC.lastReleaseSoldSeatsReq.RequestNo != "refund-93001" {
			t.Fatalf("unexpected release sold seats request: %+v", programRPC.lastReleaseSoldSeatsReq)
		}
		if payRPC.lastRefundReq == nil || payRPC.lastRefundReq.OrderNumber != 93001 || payRPC.lastRefundReq.Amount != 478 {
			t.Fatalf("unexpected refund rpc request: %+v", payRPC.lastRefundReq)
		}
		requireOrderGuardDeleted(t, svcCtx.Config.MySQL.DataSource, "d_order_user_guard", 93001)
		requireOrderGuardDeleted(t, svcCtx.Config.MySQL.DataSource, "d_order_viewer_guard", 93001)
		requireOrderGuardDeleted(t, svcCtx.Config.MySQL.DataSource, "d_order_seat_guard", 93001)
	})

	t.Run("paid order converges when pay bill already refunded", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		const showTimeID int64 = 13003
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              99002,
			OrderNumber:     93002,
			ProgramID:       10001,
			ShowTimeID:      showTimeID,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-converge",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 99201, OrderNumber: 93002, UserID: 3001, TicketUserID: 701, SeatID: 50201, OrderStatus: testOrderStatusPaid},
			orderTicketUserFixture{ID: 99202, OrderNumber: 93002, UserID: 3001, TicketUserID: 702, SeatID: 50202, OrderStatus: testOrderStatusPaid},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94002,
			OrderNumber: 93002,
			UserId:      3001,
			Amount:      598,
			PayStatus:   3,
			PayTime:     "2026-12-31 19:05:00",
		}
		payRPC.refundResp = &payrpc.RefundResp{
			RefundBillNo: 95002,
			OrderNumber:  93002,
			RefundAmount: 478,
			PayStatus:    3,
			RefundTime:   "2026-12-31 19:12:00",
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		resp, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      3001,
			OrderNumber: 93002,
			Reason:      "重复发起退款",
		})
		if err != nil {
			t.Fatalf("RefundOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded || resp.RefundBillNo != 95002 || resp.RefundAmount != 478 {
			t.Fatalf("unexpected convergence response: %+v", resp)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 93002) != testOrderStatusRefunded {
			t.Fatalf("expected order status refunded after convergence")
		}
		if programRPC.lastEvaluateRefundRuleReq != nil {
			t.Fatalf("expected no rule evaluation when pay bill already refunded, got %+v", programRPC.lastEvaluateRefundRuleReq)
		}
		if programRPC.lastReleaseSoldSeatsReq == nil || programRPC.lastReleaseSoldSeatsReq.ShowTimeId != showTimeID || programRPC.lastReleaseSoldSeatsReq.RequestNo != "refund-93002" {
			t.Fatalf("expected release sold seats request, got %+v", programRPC.lastReleaseSoldSeatsReq)
		}
	})

	t.Run("refund order returns java-facing reject copy", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		const showTimeID int64 = 13004
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              99003,
			OrderNumber:     93003,
			ProgramID:       10001,
			ShowTimeID:      showTimeID,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-reject",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 99301, OrderNumber: 93003, UserID: 3001, TicketUserID: 701, SeatID: 50301, OrderStatus: testOrderStatusPaid},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94003,
			OrderNumber: 93003,
			UserId:      3001,
			Amount:      299,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}
		programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: "演出开始前 120 分钟外可退；请按退票规则办理",
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		_, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      3001,
			OrderNumber: 93003,
			Reason:      "行程变更",
		})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %s", status.Code(err))
		}
		if status.Convert(err).Message() != "演出开始前 120 分钟外可退；请按退票规则办理" {
			t.Fatalf("expected business reject copy, got %q", status.Convert(err).Message())
		}
		if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
			t.Fatalf("unexpected evaluate refund request: %+v", programRPC.lastEvaluateRefundRuleReq)
		}
		if payRPC.lastRefundReq != nil {
			t.Fatalf("expected no refund rpc on reject, got %+v", payRPC.lastRefundReq)
		}
		if programRPC.lastReleaseSoldSeatsReq != nil {
			t.Fatalf("expected no release sold seats on reject, got %+v", programRPC.lastReleaseSoldSeatsReq)
		}
	})

	t.Run("rush sale window reject keeps order paid and skips all side effects", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		const showTimeID int64 = 13005
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              99004,
			OrderNumber:     93004,
			ProgramID:       10001,
			ShowTimeID:      showTimeID,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-rush-window",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 99401, OrderNumber: 93004, UserID: 3001, TicketUserID: 701, SeatID: 50401, OrderStatus: testOrderStatusPaid},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94004,
			OrderNumber: 93004,
			UserId:      3001,
			Amount:      299,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}
		programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
			AllowRefund:  false,
			RejectReason: "秒杀活动进行中，暂不支持退票",
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		_, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      3001,
			OrderNumber: 93004,
			Reason:      "活动进行中",
		})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %s", status.Code(err))
		}
		if status.Convert(err).Message() != "秒杀活动进行中，暂不支持退票" {
			t.Fatalf("unexpected reject message: %q", status.Convert(err).Message())
		}
		if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
			t.Fatalf("unexpected evaluate refund request: %+v", programRPC.lastEvaluateRefundRuleReq)
		}
		if payRPC.lastRefundReq != nil {
			t.Fatalf("expected no refund rpc on rush sale reject, got %+v", payRPC.lastRefundReq)
		}
		if programRPC.lastReleaseSoldSeatsReq != nil {
			t.Fatalf("expected no release sold seats on rush sale reject, got %+v", programRPC.lastReleaseSoldSeatsReq)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 93004) != testOrderStatusPaid {
			t.Fatalf("expected order status to remain paid")
		}
		if findOrderTicketStatus(t, testOrderMySQLDataSource, 93004) != testOrderStatusPaid {
			t.Fatalf("expected order ticket status to remain paid")
		}
	})
}

func seedRefundGuardFixtures(
	t *testing.T,
	dataSource string,
	orderNumber, programID, showTimeID, userID int64,
	viewerIDs []int64,
	seatIDs []int64,
) {
	t.Helper()

	now := "2026-12-31 19:05:00"
	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	if _, err := db.Exec(
		"INSERT INTO d_order_user_guard (id, order_number, program_id, show_time_id, user_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
		orderNumber+10000,
		orderNumber,
		programID,
		showTimeID,
		userID,
		now,
		now,
	); err != nil {
		t.Fatalf("insert user guard error: %v", err)
	}

	for idx, viewerID := range viewerIDs {
		if _, err := db.Exec(
			"INSERT INTO d_order_viewer_guard (id, order_number, program_id, show_time_id, viewer_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
			orderNumber+11000+int64(idx),
			orderNumber,
			programID,
			showTimeID,
			viewerID,
			now,
			now,
		); err != nil {
			t.Fatalf("insert viewer guard error: %v", err)
		}
	}

	for idx, seatID := range seatIDs {
		if _, err := db.Exec(
			"INSERT INTO d_order_seat_guard (id, order_number, program_id, show_time_id, seat_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)",
			orderNumber+12000+int64(idx),
			orderNumber,
			programID,
			showTimeID,
			seatID,
			now,
			now,
		); err != nil {
			t.Fatalf("insert seat guard error: %v", err)
		}
	}
}
