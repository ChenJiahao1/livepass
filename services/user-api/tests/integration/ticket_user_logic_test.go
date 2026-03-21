package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/user-api/internal/logic"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"
)

func TestListTicketUsersMapsResponse(t *testing.T) {
	fake := &fakeUserRPC{
		listTicketUsersResp: &userrpc.ListTicketUsersResp{
			List: []*userrpc.TicketUserInfo{
				{
					Id:       301,
					UserId:   120,
					RelName:  "购票人A",
					IdType:   1,
					IdNumber: "320101199505051234",
				},
			},
		},
	}
	logic := logicpkg.NewListTicketUsersLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.ListTicketUsers(&types.ListTicketUsersReq{UserID: 120})
	if err != nil {
		t.Fatalf("ListTicketUsers returned error: %v", err)
	}
	if resp == nil || len(resp.List) != 1 || resp.List[0].RelName != "购票人A" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastListTicketUsersReq == nil || fake.lastListTicketUsersReq.UserId != 120 {
		t.Fatalf("unexpected request: %+v", fake.lastListTicketUsersReq)
	}
}

func TestAddTicketUserCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		addTicketUserResp: &userrpc.BoolResp{Success: true},
	}
	logic := logicpkg.NewAddTicketUserLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.AddTicketUser(&types.AddTicketUserReq{
		UserID:   121,
		RelName:  "购票人B",
		IdType:   1,
		IdNumber: "320101199606061234",
	})
	if err != nil {
		t.Fatalf("AddTicketUser returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastAddTicketUserReq == nil || fake.lastAddTicketUserReq.UserId != 121 || fake.lastAddTicketUserReq.RelName != "购票人B" {
		t.Fatalf("unexpected request: %+v", fake.lastAddTicketUserReq)
	}
}

func TestDeleteTicketUserCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		deleteTicketUserResp: &userrpc.BoolResp{Success: true},
	}
	logic := logicpkg.NewDeleteTicketUserLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.DeleteTicketUser(&types.DeleteTicketUserReq{ID: 302})
	if err != nil {
		t.Fatalf("DeleteTicketUser returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastDeleteTicketUserReq == nil || fake.lastDeleteTicketUserReq.Id != 302 {
		t.Fatalf("unexpected request: %+v", fake.lastDeleteTicketUserReq)
	}
}
