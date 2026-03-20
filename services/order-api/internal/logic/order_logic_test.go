package logic

import (
	"context"
	"testing"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmiddleware"
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

func TestCreateOrderUsesUserIDFromContext(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		createOrderResp: &orderrpc.CreateOrderResp{OrderNumber: 91001},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)

	l := NewCreateOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
	resp, err := l.CreateOrder(&types.CreateOrderReq{
		ProgramID:        10001,
		TicketCategoryID: 40001,
		TicketUserIds:    []int64{701, 702},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp.OrderNumber != 91001 {
		t.Fatalf("unexpected order number: %+v", resp)
	}
	if fakeRPC.lastCreateOrderReq == nil || fakeRPC.lastCreateOrderReq.UserId != 3001 {
		t.Fatalf("expected user id from context, got %+v", fakeRPC.lastCreateOrderReq)
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

	l := NewListOrdersLogic(ctx, newOrderAPIServiceContext(fakeRPC))
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

	l := NewGetOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
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

	l := NewCancelOrderLogic(ctx, newOrderAPIServiceContext(fakeRPC))
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

func TestCreateOrderReturnsUnauthorizedWhenUserIDMissing(t *testing.T) {
	l := NewCreateOrderLogic(context.Background(), newOrderAPIServiceContext(&fakeOrderRPC{}))
	_, err := l.CreateOrder(&types.CreateOrderReq{
		ProgramID:        10001,
		TicketCategoryID: 40001,
		TicketUserIds:    []int64{701},
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
