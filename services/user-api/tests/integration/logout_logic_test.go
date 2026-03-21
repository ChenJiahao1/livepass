package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/user-api/internal/logic"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"
)

func TestLogoutCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		logoutResp: &userrpc.BoolResp{Success: true},
	}
	logic := logicpkg.NewLogoutLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.Logout(&types.UserLogoutReq{
		Code:  "0001",
		Token: "jwt-token",
	})
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastLogoutReq == nil || fake.lastLogoutReq.Code != "0001" || fake.lastLogoutReq.Token != "jwt-token" {
		t.Fatalf("unexpected logout request: %+v", fake.lastLogoutReq)
	}
}
