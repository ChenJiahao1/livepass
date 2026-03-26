package integration_test

import (
	"context"
	"testing"

	"damai-go/pkg/xredis"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
)

func newOrderCacheServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	redisClient, err := xredis.New(xredis.Config{
		Host: "127.0.0.1:6379",
		Type: "node",
	})
	if err != nil {
		t.Skipf("skip integration test, redis unavailable: %v", err)
	}

	return &svc.ServiceContext{
		Redis: redisClient,
	}
}

func TestSetOrderCreateMarkerWritesPendingMarker(t *testing.T) {
	svcCtx := newOrderCacheServiceContext(t)

	if err := logicpkg.SetOrderCreateMarker(context.Background(), svcCtx.Redis, 91001); err != nil {
		t.Fatalf("SetOrderCreateMarker returned error: %v", err)
	}

	value, err := svcCtx.Redis.GetCtx(context.Background(), "order:create:marker:91001")
	if err != nil {
		t.Fatalf("expected cache marker, got err=%v", err)
	}
	if value != "91001" {
		t.Fatalf("unexpected cache marker value: %s", value)
	}
}

func TestGetOrderCacheReturnsPendingMarker(t *testing.T) {
	svcCtx := newOrderCacheServiceContext(t)

	if err := logicpkg.SetOrderCreateMarker(context.Background(), svcCtx.Redis, 91001); err != nil {
		t.Fatalf("SetOrderCreateMarker returned error: %v", err)
	}

	l := logicpkg.NewGetOrderCacheLogic(context.Background(), svcCtx)
	resp, err := l.GetOrderCache(&pb.GetOrderCacheReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("GetOrderCache returned error: %v", err)
	}
	if resp.Cache != "91001" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetOrderCacheReturnsEmptyWhenMarkerMissing(t *testing.T) {
	svcCtx := newOrderCacheServiceContext(t)

	l := logicpkg.NewGetOrderCacheLogic(context.Background(), svcCtx)
	resp, err := l.GetOrderCache(&pb.GetOrderCacheReq{OrderNumber: 99999})
	if err != nil {
		t.Fatalf("GetOrderCache returned error: %v", err)
	}
	if resp.Cache != "" {
		t.Fatalf("expected empty cache marker, got %+v", resp)
	}
}
