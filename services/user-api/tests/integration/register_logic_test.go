package integration_test

import (
	"context"
	"testing"

	logicpkg "livepass/services/user-api/internal/logic"
	"livepass/services/user-api/internal/svc"
	"livepass/services/user-api/internal/types"
	"livepass/services/user-rpc/userrpc"
)

func TestRegisterCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		registerResp: &userrpc.BoolResp{Success: true},
	}
	logic := logicpkg.NewRegisterLogic(context.Background(), &svc.ServiceContext{
		UserRpc: fake,
	})

	resp, err := logic.Register(&types.UserRegisterReq{
		Mobile:          "13800000002",
		Password:        "123456",
		ConfirmPassword: "123456",
		Mail:            "api@example.com",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if fake.lastRegisterReq == nil {
		t.Fatalf("expected rpc request")
	}
	if fake.lastRegisterReq.Mobile != "13800000002" {
		t.Fatalf("unexpected mobile: %s", fake.lastRegisterReq.Mobile)
	}
	if fake.lastRegisterReq.Password != "123456" {
		t.Fatalf("unexpected password")
	}
	if fake.lastRegisterReq.ConfirmPassword != "123456" {
		t.Fatalf("unexpected confirm password")
	}
	if fake.lastRegisterReq.Mail != "api@example.com" {
		t.Fatalf("unexpected mail payload: %+v", fake.lastRegisterReq)
	}
}
