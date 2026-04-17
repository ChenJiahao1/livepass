package integration_test

import (
	"context"
	"testing"

	serverpkg "livepass/services/order-rpc/internal/server"
	"livepass/services/order-rpc/pb"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetOrderServiceViewReturnsPayAndRefundHintsForOwner(t *testing.T) {
	svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	const showTimeID int64 = 21001
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:              99601,
		OrderNumber:     93601,
		ProgramID:       11001,
		ShowTimeID:      showTimeID,
		ProgramTitle:    "测试演出",
		ProgramShowTime: "2026-12-31 20:00:00",
		UserID:          3001,
		TicketCount:     2,
		OrderPrice:      598,
		OrderStatus:     testOrderStatusPaid,
		PayOrderTime:    "2026-12-31 19:05:00",
	})
	seedOrderTicketUserFixtures(t, svcCtx,
		orderTicketUserFixture{ID: 99611, OrderNumber: 93601, UserID: 3001, TicketUserID: 701, OrderStatus: testOrderStatusPaid},
		orderTicketUserFixture{ID: 99612, OrderNumber: 93601, UserID: 3001, TicketUserID: 702, OrderStatus: testOrderStatusPaid},
	)
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		PayBillNo:   94601,
		OrderNumber: 93601,
		UserId:      3001,
		Amount:      598,
		PayStatus:   2,
		PayTime:     "2026-12-31 19:05:00",
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:   true,
		RefundPercent: 80,
		RefundAmount:  478,
	}

	server := serverpkg.NewOrderRpcServer(svcCtx)
	resp, err := server.GetOrderServiceView(context.Background(), &pb.GetOrderServiceViewReq{
		UserId:      3001,
		OrderNumber: 93601,
	})
	if err != nil {
		t.Fatalf("GetOrderServiceView returned error: %v", err)
	}
	if resp.OrderNumber != 93601 || resp.OrderStatus != testOrderStatusPaid {
		t.Fatalf("unexpected order view: %+v", resp)
	}
	if resp.PayStatus != 2 || resp.TicketStatus != testOrderStatusPaid {
		t.Fatalf("unexpected pay/ticket status: %+v", resp)
	}
	if !resp.CanRefund || resp.RefundBlockedReason != "" {
		t.Fatalf("expected refundable service view, got %+v", resp)
	}
	if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
		t.Fatalf("unexpected refund rule request: %+v", programRPC.lastEvaluateRefundRuleReq)
	}
}

func TestGetOrderServiceViewReturnsNotFoundForAnotherUser(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          99602,
		OrderNumber: 93602,
		ProgramID:   11001,
		UserID:      3002,
		OrderStatus: testOrderStatusPaid,
	})

	server := serverpkg.NewOrderRpcServer(svcCtx)
	_, err := server.GetOrderServiceView(context.Background(), &pb.GetOrderServiceViewReq{
		UserId:      3001,
		OrderNumber: 93602,
	})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found, got %s", status.Code(err))
	}
}

func TestPreviewRefundOrderReturnsPreviewForRefundablePaidOrder(t *testing.T) {
	svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	const showTimeID int64 = 21003
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:              99603,
		OrderNumber:     93603,
		ProgramID:       11002,
		ShowTimeID:      showTimeID,
		UserID:          3001,
		OrderPrice:      598,
		OrderStatus:     testOrderStatusPaid,
		ProgramShowTime: "2026-12-31 20:00:00",
		PayOrderTime:    "2026-12-31 19:05:00",
	})
	seedOrderTicketUserFixtures(t, svcCtx,
		orderTicketUserFixture{ID: 99631, OrderNumber: 93603, UserID: 3001, TicketUserID: 701, SeatID: 50301, OrderStatus: testOrderStatusPaid},
		orderTicketUserFixture{ID: 99632, OrderNumber: 93603, UserID: 3001, TicketUserID: 702, SeatID: 50302, OrderStatus: testOrderStatusPaid},
	)
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		PayBillNo:   94603,
		OrderNumber: 93603,
		UserId:      3001,
		Amount:      598,
		PayStatus:   2,
		PayTime:     "2026-12-31 19:05:00",
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:   true,
		RefundPercent: 80,
		RefundAmount:  478,
	}

	server := serverpkg.NewOrderRpcServer(svcCtx)
	resp, err := server.PreviewRefundOrder(context.Background(), &pb.PreviewRefundOrderReq{
		UserId:      3001,
		OrderNumber: 93603,
	})
	if err != nil {
		t.Fatalf("PreviewRefundOrder returned error: %v", err)
	}
	if !resp.AllowRefund || resp.RefundAmount != 478 || resp.RefundPercent != 80 || resp.RejectReason != "" {
		t.Fatalf("unexpected preview response: %+v", resp)
	}
	if findOrderStatus(t, testOrderMySQLDataSource, 93603) != testOrderStatusPaid {
		t.Fatalf("expected preview to keep order status unchanged")
	}
	if findOrderTicketStatus(t, testOrderMySQLDataSource, 93603) != testOrderStatusPaid {
		t.Fatalf("expected preview to keep ticket status unchanged")
	}
	if payRPC.refundCalls != 0 {
		t.Fatalf("expected preview not to call refund rpc, got %d", payRPC.refundCalls)
	}
	if programRPC.releaseSoldSeatsCalls != 0 {
		t.Fatalf("expected preview not to release sold seats, got %d", programRPC.releaseSoldSeatsCalls)
	}
	if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
		t.Fatalf("unexpected refund rule request: %+v", programRPC.lastEvaluateRefundRuleReq)
	}
}

func TestPreviewRefundOrderRejectsNonRefundableOrder(t *testing.T) {
	svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	const showTimeID int64 = 21004
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:              99604,
		OrderNumber:     93604,
		ProgramID:       11003,
		ShowTimeID:      showTimeID,
		UserID:          3001,
		OrderPrice:      299,
		OrderStatus:     testOrderStatusPaid,
		ProgramShowTime: "2026-12-31 20:00:00",
		PayOrderTime:    "2026-12-31 19:05:00",
	})
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		PayBillNo:   94604,
		OrderNumber: 93604,
		UserId:      3001,
		Amount:      299,
		PayStatus:   2,
		PayTime:     "2026-12-31 19:05:00",
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:  false,
		RejectReason: "演出开始前 120 分钟外可退；请按退票规则办理",
	}

	server := serverpkg.NewOrderRpcServer(svcCtx)
	resp, err := server.PreviewRefundOrder(context.Background(), &pb.PreviewRefundOrderReq{
		UserId:      3001,
		OrderNumber: 93604,
	})
	if err != nil {
		t.Fatalf("PreviewRefundOrder returned error: %v", err)
	}
	if resp.AllowRefund || resp.RefundAmount != 0 || resp.RefundPercent != 0 {
		t.Fatalf("expected reject preview, got %+v", resp)
	}
	if resp.RejectReason != "演出开始前 120 分钟外可退；请按退票规则办理" {
		t.Fatalf("unexpected reject reason: %+v", resp)
	}
	if findOrderStatus(t, testOrderMySQLDataSource, 93604) != testOrderStatusPaid {
		t.Fatalf("expected preview reject to keep order status unchanged")
	}
	if payRPC.refundCalls != 0 {
		t.Fatalf("expected preview reject not to call refund rpc, got %d", payRPC.refundCalls)
	}
	if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
		t.Fatalf("unexpected refund rule request: %+v", programRPC.lastEvaluateRefundRuleReq)
	}
	if programRPC.releaseSoldSeatsCalls != 0 {
		t.Fatalf("expected preview reject not to release sold seats, got %d", programRPC.releaseSoldSeatsCalls)
	}
}

func TestGetOrderServiceViewBlocksRefundDuringRushSaleWindow(t *testing.T) {
	svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	const showTimeID int64 = 21005
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:              99605,
		OrderNumber:     93605,
		ProgramID:       11004,
		ShowTimeID:      showTimeID,
		ProgramTitle:    "抢票场次",
		ProgramShowTime: "2026-12-31 20:00:00",
		UserID:          3001,
		TicketCount:     1,
		OrderPrice:      299,
		OrderStatus:     testOrderStatusPaid,
		PayOrderTime:    "2026-12-31 19:05:00",
	})
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		PayBillNo:   94605,
		OrderNumber: 93605,
		UserId:      3001,
		Amount:      299,
		PayStatus:   2,
		PayTime:     "2026-12-31 19:05:00",
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:  false,
		RejectReason: "秒杀活动进行中，暂不支持退票",
	}

	server := serverpkg.NewOrderRpcServer(svcCtx)
	resp, err := server.GetOrderServiceView(context.Background(), &pb.GetOrderServiceViewReq{
		UserId:      3001,
		OrderNumber: 93605,
	})
	if err != nil {
		t.Fatalf("GetOrderServiceView returned error: %v", err)
	}
	if resp.CanRefund {
		t.Fatalf("expected refund blocked during rush sale window, got %+v", resp)
	}
	if resp.RefundBlockedReason != "秒杀活动进行中，暂不支持退票" {
		t.Fatalf("unexpected blocked reason: %+v", resp)
	}
	if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != showTimeID {
		t.Fatalf("unexpected refund rule request: %+v", programRPC.lastEvaluateRefundRuleReq)
	}
}
