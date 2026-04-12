package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"damai-go/jobs/order-close/internal/config"
	logicpkg "damai-go/jobs/order-close/internal/logic"
	"damai-go/jobs/order-close/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"google.golang.org/grpc"
)

const (
	testOrderStatusUnpaid    int64 = 1
	testOrderStatusCancelled int64 = 2
	testOrderStatusPaid      int64 = 3
)

type fakeOrderCloseStore struct {
	listItems       []svc.PendingOrderCreatedOutbox
	listErr         error
	listCalls       int
	markPublished   []svc.OutboxRef
	markPublishedAt []time.Time
	markErr         error
}

func (f *fakeOrderCloseStore) ListPendingOrderCreatedOutboxes(_ context.Context, limit int64) ([]svc.PendingOrderCreatedOutbox, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	if int64(len(f.listItems)) <= limit {
		items := append([]svc.PendingOrderCreatedOutbox(nil), f.listItems...)
		return items, nil
	}
	items := append([]svc.PendingOrderCreatedOutbox(nil), f.listItems[:limit]...)
	return items, nil
}

func (f *fakeOrderCloseStore) MarkOutboxPublished(_ context.Context, ref svc.OutboxRef, publishedAt time.Time) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.markPublished = append(f.markPublished, ref)
	f.markPublishedAt = append(f.markPublishedAt, publishedAt)
	return nil
}

type fakeOrderCloseAsyncClient struct {
	enqueueCalls []enqueueCloseTimeoutCall
	enqueueErr   error
}

type enqueueCloseTimeoutCall struct {
	orderNumber int64
	expireAt    time.Time
}

func (f *fakeOrderCloseAsyncClient) EnqueueCloseTimeout(_ context.Context, orderNumber int64, expireAt time.Time) error {
	if f.enqueueErr != nil {
		return f.enqueueErr
	}
	f.enqueueCalls = append(f.enqueueCalls, enqueueCloseTimeoutCall{
		orderNumber: orderNumber,
		expireAt:    expireAt,
	})
	return nil
}

type fakeOrderCloseRPC struct {
	getOrderResp          map[int64]*orderrpc.OrderDetailInfo
	getOrderErr           error
	getOrderReqs          []*orderrpc.GetOrderReq
	closeExpiredOrderErr  error
	closeExpiredOrderReqs []*orderrpc.CloseExpiredOrderReq
}

func (f *fakeOrderCloseRPC) GetOrder(_ context.Context, in *orderrpc.GetOrderReq, _ ...grpc.CallOption) (*orderrpc.OrderDetailInfo, error) {
	f.getOrderReqs = append(f.getOrderReqs, in)
	if f.getOrderErr != nil {
		return nil, f.getOrderErr
	}
	if resp, ok := f.getOrderResp[in.GetOrderNumber()]; ok {
		return resp, nil
	}
	return nil, errors.New("order not found")
}

func (f *fakeOrderCloseRPC) CloseExpiredOrder(_ context.Context, in *orderrpc.CloseExpiredOrderReq, _ ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	f.closeExpiredOrderReqs = append(f.closeExpiredOrderReqs, in)
	if f.closeExpiredOrderErr != nil {
		return nil, f.closeExpiredOrderErr
	}
	return &orderrpc.BoolResp{Success: true}, nil
}

func TestRunOnceEnqueuesCloseTimeoutForPendingUnpaidOrder(t *testing.T) {
	now := time.Now()
	store := &fakeOrderCloseStore{
		listItems: []svc.PendingOrderCreatedOutbox{
			{Ref: svc.OutboxRef{DBKey: "order-db-0", ID: 101}, OrderNumber: 91001, UserID: 3001},
		},
	}
	asyncClient := &fakeOrderCloseAsyncClient{}
	orderRPC := &fakeOrderCloseRPC{
		getOrderResp: map[int64]*orderrpc.OrderDetailInfo{
			91001: {
				OrderNumber:     91001,
				UserId:          3001,
				OrderStatus:     testOrderStatusUnpaid,
				OrderExpireTime: now.Add(10 * time.Minute).Format("2006-01-02 15:04:05"),
			},
		},
	}
	logic := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config:           config.Config{BatchSize: 10},
		OutboxStore:      store,
		AsyncCloseClient: asyncClient,
		OrderRpc:         orderRPC,
	})

	if err := logic.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(asyncClient.enqueueCalls) != 1 {
		t.Fatalf("enqueue calls = %d, want 1", len(asyncClient.enqueueCalls))
	}
	if got := asyncClient.enqueueCalls[0]; got.orderNumber != 91001 {
		t.Fatalf("unexpected enqueue call: %+v", got)
	}
	if len(orderRPC.closeExpiredOrderReqs) != 0 {
		t.Fatalf("close expired order calls = %d, want 0", len(orderRPC.closeExpiredOrderReqs))
	}
	if len(store.markPublished) != 1 || store.markPublished[0].ID != 101 {
		t.Fatalf("unexpected mark published refs: %+v", store.markPublished)
	}
}

func TestRunOnceClosesExpiredUnpaidOrderImmediately(t *testing.T) {
	now := time.Now()
	store := &fakeOrderCloseStore{
		listItems: []svc.PendingOrderCreatedOutbox{
			{Ref: svc.OutboxRef{DBKey: "order-db-1", ID: 201}, OrderNumber: 92001, UserID: 3002},
		},
	}
	asyncClient := &fakeOrderCloseAsyncClient{}
	orderRPC := &fakeOrderCloseRPC{
		getOrderResp: map[int64]*orderrpc.OrderDetailInfo{
			92001: {
				OrderNumber:     92001,
				UserId:          3002,
				OrderStatus:     testOrderStatusUnpaid,
				OrderExpireTime: now.Add(-2 * time.Minute).Format("2006-01-02 15:04:05"),
			},
		},
	}
	logic := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config:           config.Config{BatchSize: 10},
		OutboxStore:      store,
		AsyncCloseClient: asyncClient,
		OrderRpc:         orderRPC,
	})

	if err := logic.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(orderRPC.closeExpiredOrderReqs) != 1 || orderRPC.closeExpiredOrderReqs[0].GetOrderNumber() != 92001 {
		t.Fatalf("unexpected close expired order reqs: %+v", orderRPC.closeExpiredOrderReqs)
	}
	if len(asyncClient.enqueueCalls) != 0 {
		t.Fatalf("enqueue calls = %d, want 0", len(asyncClient.enqueueCalls))
	}
	if len(store.markPublished) != 1 || store.markPublished[0].ID != 201 {
		t.Fatalf("unexpected mark published refs: %+v", store.markPublished)
	}
}

func TestRunOnceMarksTerminalOrderOutboxWithoutDispatch(t *testing.T) {
	store := &fakeOrderCloseStore{
		listItems: []svc.PendingOrderCreatedOutbox{
			{Ref: svc.OutboxRef{DBKey: "order-db-0", ID: 301}, OrderNumber: 93001, UserID: 3003},
			{Ref: svc.OutboxRef{DBKey: "order-db-0", ID: 302}, OrderNumber: 93002, UserID: 3003},
		},
	}
	asyncClient := &fakeOrderCloseAsyncClient{}
	orderRPC := &fakeOrderCloseRPC{
		getOrderResp: map[int64]*orderrpc.OrderDetailInfo{
			93001: {
				OrderNumber: 93001,
				UserId:      3003,
				OrderStatus: testOrderStatusPaid,
			},
			93002: {
				OrderNumber: 93002,
				UserId:      3003,
				OrderStatus: testOrderStatusCancelled,
			},
		},
	}
	logic := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config:           config.Config{BatchSize: 10},
		OutboxStore:      store,
		AsyncCloseClient: asyncClient,
		OrderRpc:         orderRPC,
	})

	if err := logic.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(asyncClient.enqueueCalls) != 0 {
		t.Fatalf("enqueue calls = %d, want 0", len(asyncClient.enqueueCalls))
	}
	if len(orderRPC.closeExpiredOrderReqs) != 0 {
		t.Fatalf("close expired order calls = %d, want 0", len(orderRPC.closeExpiredOrderReqs))
	}
	if len(store.markPublished) != 2 {
		t.Fatalf("mark published calls = %d, want 2", len(store.markPublished))
	}
}

func TestRunOncePropagatesOutboxLoadFailure(t *testing.T) {
	logic := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config:      config.Config{BatchSize: 10},
		OutboxStore: &fakeOrderCloseStore{listErr: errors.New("load failed")},
	})

	if err := logic.RunOnce(); err == nil {
		t.Fatalf("expected outbox load error")
	}
}

func TestRunOnceUsesBatchSizeWhenLoadingOutbox(t *testing.T) {
	store := &fakeOrderCloseStore{
		listItems: []svc.PendingOrderCreatedOutbox{
			{Ref: svc.OutboxRef{DBKey: "order-db-0", ID: 401}, OrderNumber: 94001, UserID: 3004},
			{Ref: svc.OutboxRef{DBKey: "order-db-0", ID: 402}, OrderNumber: 94002, UserID: 3004},
		},
	}
	orderRPC := &fakeOrderCloseRPC{
		getOrderResp: map[int64]*orderrpc.OrderDetailInfo{
			94001: {
				OrderNumber:     94001,
				UserId:          3004,
				OrderStatus:     testOrderStatusUnpaid,
				OrderExpireTime: time.Now().Add(time.Minute).Format("2006-01-02 15:04:05"),
			},
		},
	}
	logic := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config:           config.Config{BatchSize: 1},
		OutboxStore:      store,
		AsyncCloseClient: &fakeOrderCloseAsyncClient{},
		OrderRpc:         orderRPC,
	})

	if err := logic.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if store.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", store.listCalls)
	}
	if len(orderRPC.getOrderReqs) != 1 || orderRPC.getOrderReqs[0].GetOrderNumber() != 94001 {
		t.Fatalf("unexpected get order reqs: %+v", orderRPC.getOrderReqs)
	}
}
