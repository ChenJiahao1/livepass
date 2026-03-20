package logic

import (
	"context"
	"testing"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"
)

func TestExistCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		existResp: &userrpc.BoolResp{Success: true},
	}
	logic := NewExistLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.Exist(&types.UserExistReq{
		Mobile: "13800000010",
	})
	if err != nil {
		t.Fatalf("Exist returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastExistReq == nil || fake.lastExistReq.Mobile != "13800000010" {
		t.Fatalf("unexpected request: %+v", fake.lastExistReq)
	}
}
