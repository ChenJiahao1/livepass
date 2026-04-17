package integration_test

import (
	"context"
	"testing"
	"time"

	"livepass/services/order-rpc/internal/model"
	ordermcp "livepass/services/order-rpc/mcp"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

func TestPreviewRefundTool_ReturnsRefundProjection(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders: map[int64]*model.DOrder{
			10001: {
				Id:              1,
				OrderNumber:     10001,
				UserId:          3001,
				ProgramId:       20001,
				ShowTimeId:      30001,
				ProgramTitle:    "演出 A",
				ProgramShowTime: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				OrderPrice:      299,
				OrderStatus:     testOrderStatusPaid,
				CreateOrderTime: time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			},
		},
		tickets: map[int64][]*model.DOrderTicketUser{},
	}
	svcCtx, programRPC, payRPC := newMCPServiceContext(t, repo)
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		OrderNumber: 10001,
		UserId:      3001,
		PayStatus:   2,
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:   true,
		RefundAmount:  199,
		RefundPercent: 80,
	}
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	result, err := server.PreviewRefundOrder(context.Background(), ordermcp.PreviewRefundOrderArgs{
		UserID:  3001,
		OrderID: "ORD-10001",
	})
	if err != nil {
		t.Fatalf("PreviewRefundOrder returned error: %v", err)
	}
	if !result.AllowRefund || result.RefundAmount != "199" || result.RefundPercent != 80 {
		t.Fatalf("unexpected refund preview: %+v", result)
	}
	if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != 30001 {
		t.Fatalf("unexpected refund rule request: %+v", programRPC.lastEvaluateRefundRuleReq)
	}
}

func TestRefundTool_SubmitsRefundResult(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders: map[int64]*model.DOrder{
			10001: {
				Id:              1,
				OrderNumber:     10001,
				UserId:          3001,
				ProgramId:       20001,
				ShowTimeId:      30002,
				ProgramTitle:    "演出 A",
				ProgramShowTime: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				OrderPrice:      299,
				OrderStatus:     testOrderStatusPaid,
				CreateOrderTime: time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			},
		},
		tickets: map[int64][]*model.DOrderTicketUser{
			10001: {
				{OrderNumber: 10001, UserId: 3001, SeatId: 9001, OrderStatus: testOrderStatusPaid},
			},
		},
	}
	svcCtx, programRPC, payRPC := newMCPServiceContext(t, repo)
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		OrderNumber: 10001,
		UserId:      3001,
		PayStatus:   2,
	}
	payRPC.refundResp = &payrpc.RefundResp{
		OrderNumber:  10001,
		RefundAmount: 199,
		RefundBillNo: 7001,
		PayStatus:    3,
		RefundTime:   "2026-04-05 10:05:00",
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:   true,
		RefundAmount:  199,
		RefundPercent: 80,
	}
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	result, err := server.RefundOrder(context.Background(), ordermcp.RefundOrderArgs{
		UserID:  3001,
		OrderID: "10001",
		Reason:  "行程变更",
	})
	if err != nil {
		t.Fatalf("RefundOrder returned error: %v", err)
	}
	if result.OrderID != "ORD-10001" || result.Status != "refunded" {
		t.Fatalf("unexpected refund result: %+v", result)
	}
	if result.RefundBillNo != "7001" || result.RefundAmount != "199" {
		t.Fatalf("unexpected refund payload: %+v", result)
	}
	if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ShowTimeId != 30002 {
		t.Fatalf("unexpected refund rule request: %+v", programRPC.lastEvaluateRefundRuleReq)
	}
	if programRPC.lastReleaseSoldSeatsReq == nil || programRPC.lastReleaseSoldSeatsReq.ShowTimeId != 30002 {
		t.Fatalf("unexpected release sold seats request: %+v", programRPC.lastReleaseSoldSeatsReq)
	}
}

func TestOrderMCPTool_RejectsInvalidOrderID(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders:  map[int64]*model.DOrder{},
		tickets: map[int64][]*model.DOrderTicketUser{},
	}
	svcCtx, _, _ := newMCPServiceContext(t, repo)
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	_, err := server.PreviewRefundOrder(context.Background(), ordermcp.PreviewRefundOrderArgs{
		UserID:  3001,
		OrderID: "ORD-ABC",
	})
	if err == nil {
		t.Fatalf("expected invalid order id error")
	}
}

func TestRefundTool_RejectsDuringRushSaleWindow(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders: map[int64]*model.DOrder{
			10002: {
				Id:              2,
				OrderNumber:     10002,
				UserId:          3001,
				ProgramId:       20002,
				ShowTimeId:      30003,
				ProgramTitle:    "演出 B",
				ProgramShowTime: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				OrderPrice:      399,
				OrderStatus:     testOrderStatusPaid,
				CreateOrderTime: time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			},
		},
		tickets: map[int64][]*model.DOrderTicketUser{
			10002: {
				{OrderNumber: 10002, UserId: 3001, SeatId: 9002, OrderStatus: testOrderStatusPaid},
			},
		},
	}
	svcCtx, programRPC, payRPC := newMCPServiceContext(t, repo)
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		OrderNumber: 10002,
		UserId:      3001,
		PayStatus:   2,
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:  false,
		RejectReason: "秒杀活动进行中，暂不支持退票",
	}
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	_, err := server.RefundOrder(context.Background(), ordermcp.RefundOrderArgs{
		UserID:  3001,
		OrderID: "ORD-10002",
		Reason:  "活动进行中",
	})
	if err == nil {
		t.Fatalf("expected refund reject error")
	}
	if err.Error() != "rpc error: code = FailedPrecondition desc = 秒杀活动进行中，暂不支持退票" {
		t.Fatalf("unexpected error: %v", err)
	}
	if payRPC.lastRefundReq != nil {
		t.Fatalf("expected no refund rpc on reject, got %+v", payRPC.lastRefundReq)
	}
	if programRPC.lastReleaseSoldSeatsReq != nil {
		t.Fatalf("expected no release sold seats on reject, got %+v", programRPC.lastReleaseSoldSeatsReq)
	}
}
