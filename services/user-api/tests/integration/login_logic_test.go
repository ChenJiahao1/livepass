package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/user-api/internal/logic"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"
)

func TestLoginCallsRpcAndMapsResponse(t *testing.T) {
	fake := &fakeUserRPC{
		loginResp: &userrpc.LoginResp{
			UserId: 88,
			Token:  "token-88",
		},
	}
	logic := logicpkg.NewLoginLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.Login(&types.UserLoginReq{
		Mobile:   "13800000002",
		Email:    "login@example.com",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if resp == nil || resp.UserID != 88 || resp.Token != "token-88" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastLoginReq == nil {
		t.Fatalf("expected rpc request")
	}
	if fake.lastLoginReq.Mobile != "13800000002" || fake.lastLoginReq.Email != "login@example.com" {
		t.Fatalf("unexpected login request: %+v", fake.lastLoginReq)
	}
}
