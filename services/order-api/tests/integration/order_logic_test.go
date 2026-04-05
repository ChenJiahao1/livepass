package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmiddleware"
	"damai-go/services/order-api/internal/handler"
	logicpkg "damai-go/services/order-api/internal/logic"
	"damai-go/services/order-api/internal/svc"
	"damai-go/services/order-api/internal/types"
	"damai-go/services/order-rpc/orderrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newOrderAPIServiceContext(fakeRPC *fakeOrderRPC) *svc.ServiceContext {
	return &svc.ServiceContext{
		OrderRpc: fakeRPC,
	}
}

func TestCreateOrderReturnsPreAllocatedOrderNumber(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		createOrderResp: &orderrpc.CreateOrderResp{OrderNumber: 91001},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewCreateOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.CreateOrder(&types.CreateOrderReq{
		PurchaseToken: "pt_91001",
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.OrderNumber != 91001 {
		t.Fatalf("unexpected order number: %+v", resp)
	}
	if fakeRPC.lastCreateOrderReq == nil || fakeRPC.lastCreateOrderReq.UserId != 3001 || fakeRPC.lastCreateOrderReq.PurchaseToken != "pt_91001" {
		t.Fatalf("expected user id from context, got %+v", fakeRPC.lastCreateOrderReq)
	}
}

func TestCreateOrderMayNotBeImmediatelyVisible(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		createOrderResp: &orderrpc.CreateOrderResp{OrderNumber: 91001},
		getOrderErr:     status.Error(codes.NotFound, xerr.ErrOrderNotFound.Error()),
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	serviceCtx := newOrderAPIServiceContext(fakeRPC)

	createLogic := logicpkg.NewCreateOrderLogic(ctx, serviceCtx)
	createResp, err := createLogic.CreateOrder(&types.CreateOrderReq{
		PurchaseToken: "pt_91001",
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if createResp.OrderNumber != 91001 {
		t.Fatalf("unexpected create response: %+v", createResp)
	}

	getLogic := logicpkg.NewGetOrderLogic(ctx, serviceCtx)
	_, err = getLogic.GetOrder(&types.GetOrderReq{OrderNumber: createResp.OrderNumber})
	if err == nil {
		t.Fatalf("expected not found during async visibility window")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found, got %s", status.Code(err))
	}
}

func TestPollOrderProgressPassesThroughRpcResult(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		pollOrderProgressResp: &orderrpc.PollOrderProgressResp{
			OrderNumber: 91001,
			OrderStatus: 2,
			Done:        true,
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewPollOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.PollOrder(&types.PollOrderReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("PollOrder returned error: %v", err)
	}
	if resp.OrderNumber != 91001 || resp.OrderStatus != 2 || !resp.Done {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastPollOrderProgressReq == nil || fakeRPC.lastPollOrderProgressReq.UserId != 3001 || fakeRPC.lastPollOrderProgressReq.OrderNumber != 91001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastPollOrderProgressReq)
	}
}

func TestCreateOrderPropagatesLedgerNotReady(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		createOrderErr: status.Error(codes.FailedPrecondition, xerr.ErrOrderLimitLedgerNotReady.Error()),
	}

	req := httptest.NewRequest(http.MethodPost, "/order/create", strings.NewReader(`{"purchaseToken":"pt_91001"}`))
	req = req.WithContext(xmiddleware.WithUserID(req.Context(), 3001))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.CreateOrderHandler(newOrderAPIServiceContext(fakeRPC))(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, xerr.ErrOrderLimitLedgerNotReady.Error()) {
		t.Fatalf("expected response body to contain ledger not ready message, got %s", body)
	}
	if strings.Contains(body, xerr.ErrInternal.Error()) {
		t.Fatalf("expected response body not to contain internal error, got %s", body)
	}
}

func TestListOrdersUsesDefaultPageValuesWhenOmitted(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		listOrdersResp: &orderrpc.ListOrdersResp{
			PageNum:   1,
			PageSize:  10,
			TotalSize: 1,
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewListOrdersLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.ListOrders(&types.ListOrdersReq{})
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if resp.PageNum != 1 || resp.PageSize != 10 {
		t.Fatalf("unexpected page defaults: %+v", resp)
	}
	if fakeRPC.lastListOrdersReq == nil || fakeRPC.lastListOrdersReq.UserId != 3001 || fakeRPC.lastListOrdersReq.PageNumber != 1 || fakeRPC.lastListOrdersReq.PageSize != 10 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastListOrdersReq)
	}
}

func TestGetOrderForwardsOrderNumber(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		getOrderResp: &orderrpc.OrderDetailInfo{OrderNumber: 91001},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewGetOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.GetOrder(&types.GetOrderReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	if resp.OrderNumber != 91001 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastGetOrderReq == nil || fakeRPC.lastGetOrderReq.UserId != 3001 || fakeRPC.lastGetOrderReq.OrderNumber != 91001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastGetOrderReq)
	}
}

func TestCancelOrderForwardsUserIDAndOrderNumber(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		cancelOrderResp: &orderrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewCancelOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.CancelOrder(&types.CancelOrderReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if fakeRPC.lastCancelOrderReq == nil || fakeRPC.lastCancelOrderReq.UserId != 3001 || fakeRPC.lastCancelOrderReq.OrderNumber != 91001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastCancelOrderReq)
	}
}

func TestPayOrderForwardsUserIDAndPayload(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		payOrderResp: &orderrpc.PayOrderResp{
			OrderNumber: 91001,
			OrderStatus: 3,
			PayBillNo:   92001,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewPayOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.PayOrder(&types.PayOrderReq{
		OrderNumber: 91001,
		Subject:     "演出票支付",
		Channel:     "mock",
	})
	if err != nil {
		t.Fatalf("PayOrder returned error: %v", err)
	}
	if resp.OrderStatus != 3 || resp.PayBillNo != 92001 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastPayOrderReq == nil || fakeRPC.lastPayOrderReq.UserId != 3001 || fakeRPC.lastPayOrderReq.OrderNumber != 91001 || fakeRPC.lastPayOrderReq.Subject != "演出票支付" || fakeRPC.lastPayOrderReq.Channel != "mock" {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastPayOrderReq)
	}
}

func TestPayCheckForwardsUserIDAndOrderNumber(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		payCheckResp: &orderrpc.PayCheckResp{
			OrderNumber: 91001,
			OrderStatus: 3,
			PayBillNo:   92001,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewPayCheckLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.PayCheck(&types.PayCheckReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("PayCheck returned error: %v", err)
	}
	if resp.OrderStatus != 3 || resp.PayBillNo != 92001 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastPayCheckReq == nil || fakeRPC.lastPayCheckReq.UserId != 3001 || fakeRPC.lastPayCheckReq.OrderNumber != 91001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastPayCheckReq)
	}
}

func TestCreateOrderReturnsUnauthorizedWhenUserIDMissing(t *testing.T) {
	l := logicpkg.NewCreateOrderLogic(context.Background(), newOrderAPIServiceContext(&fakeOrderRPC{}))
	_, err := l.CreateOrder(&types.CreateOrderReq{
		PurchaseToken: "pt_91001",
	})
	if err == nil {
		t.Fatalf("expected unauthorized error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated, got %s", status.Code(err))
	}
	if status.Convert(err).Message() != xerr.ErrUnauthorized.Error() {
		t.Fatalf("unexpected error message: %s", status.Convert(err).Message())
	}
}

func TestRefundOrderForwardsUserIDAndPayload(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		refundOrderResp: &orderrpc.RefundOrderResp{
			OrderNumber:   91001,
			OrderStatus:   4,
			RefundAmount:  478,
			RefundPercent: 80,
			RefundBillNo:  92001,
			RefundTime:    "2026-12-31 19:10:00",
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := logicpkg.NewRefundOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.RefundOrder(&types.RefundOrderReq{
		OrderNumber: 91001,
		Reason:      "行程变更",
	})
	if err != nil {
		t.Fatalf("RefundOrder returned error: %v", err)
	}
	if resp.OrderStatus != 4 || resp.RefundAmount != 478 || resp.RefundPercent != 80 || resp.RefundBillNo != 92001 || resp.RefundTime != "2026-12-31 19:10:00" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastRefundOrderReq == nil || fakeRPC.lastRefundOrderReq.UserId != 3001 || fakeRPC.lastRefundOrderReq.OrderNumber != 91001 || fakeRPC.lastRefundOrderReq.Reason != "行程变更" {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastRefundOrderReq)
	}
}

func TestAccountOrderCountForwardsExplicitUserAndProgram(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		countActiveTicketsByUserProgramResp: &orderrpc.CountActiveTicketsByUserProgramResp{ActiveTicketCount: 5},
	}

	l := logicpkg.NewAccountOrderCountLogic(context.Background(), newOrderAPIServiceContext(fakeRPC))
	resp, err := l.AccountOrderCount(&types.AccountOrderCountReq{
		UserID:    3001,
		ProgramID: 10001,
	})
	if err != nil {
		t.Fatalf("AccountOrderCount returned error: %v", err)
	}
	if resp.Count != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastCountActiveTicketsByUserProgramReq == nil ||
		fakeRPC.lastCountActiveTicketsByUserProgramReq.UserId != 3001 ||
		fakeRPC.lastCountActiveTicketsByUserProgramReq.ProgramId != 10001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastCountActiveTicketsByUserProgramReq)
	}
}

func TestGetOrderCacheForwardsOrderNumber(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		getOrderCacheResp: &orderrpc.GetOrderCacheResp{Cache: "91001"},
	}

	l := logicpkg.NewGetOrderCacheLogic(context.Background(), newOrderAPIServiceContext(fakeRPC))
	resp, err := l.GetOrderCache(&types.OrderCacheReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("GetOrderCache returned error: %v", err)
	}
	if resp.Cache != "91001" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastGetOrderCacheReq == nil || fakeRPC.lastGetOrderCacheReq.OrderNumber != 91001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastGetOrderCacheReq)
	}
}
