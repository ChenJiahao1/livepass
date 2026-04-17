package integration_test

import (
	"context"
	"testing"

	logicpkg "livepass/services/user-api/internal/logic"
	"livepass/services/user-api/internal/svc"
	"livepass/services/user-api/internal/types"
	"livepass/services/user-rpc/userrpc"
)

func TestLogoutCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		logoutResp: &userrpc.BoolResp{Success: true},
	}
	logic := logicpkg.NewLogoutLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.Logout(&types.UserLogoutReq{
		Token: "jwt-token",
	})
	if err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastLogoutReq == nil || fake.lastLogoutReq.Token != "jwt-token" {
		t.Fatalf("unexpected logout request: %+v", fake.lastLogoutReq)
	}
}
