package logic

import (
	"context"
	"testing"

	"damai-go/services/user-rpc/pb"
)

func TestGetUserByIdReturnsUser(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Name:     "query-by-id",
		Mobile:   "13800000030",
		Email:    "query-id@example.com",
		Password: "123456",
		RelName:  "张三",
		IdNumber: "310101199001011234",
		Address:  "Shanghai",
	})

	l := NewGetUserByIdLogic(context.Background(), svcCtx)
	resp, err := l.GetUserById(&pb.GetUserByIdReq{Id: user.Id})
	if err != nil {
		t.Fatalf("GetUserById returned error: %v", err)
	}
	if resp.Id != user.Id {
		t.Fatalf("unexpected user id: %d", resp.Id)
	}
	if resp.Mobile != "13800000030" || resp.RelName != "张三" || resp.Address != "Shanghai" {
		t.Fatalf("unexpected user info: %+v", resp)
	}
}

func TestGetUserByMobileReturnsUser(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Name:     "query-by-mobile",
		Mobile:   "13800000031",
		Password: "123456",
	})

	l := NewGetUserByMobileLogic(context.Background(), svcCtx)
	resp, err := l.GetUserByMobile(&pb.GetUserByMobileReq{Mobile: "13800000031"})
	if err != nil {
		t.Fatalf("GetUserByMobile returned error: %v", err)
	}
	if resp.Id != user.Id {
		t.Fatalf("unexpected user id: %d", resp.Id)
	}
	if resp.Mobile != "13800000031" {
		t.Fatalf("unexpected mobile: %s", resp.Mobile)
	}
}
