package integration_test

import (
	"context"
	"testing"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmiddleware"
	logicpkg "damai-go/services/user-api/internal/logic"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewListTicketUsersLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.ListTicketUsers(&types.ListTicketUsersReq{})
	if err != nil {
		t.Fatalf("ListTicketUsers returned error: %v", err)
	}
	if resp == nil || len(resp.List) != 1 || resp.List[0].RelName != "购票人A" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastListTicketUsersReq == nil || fake.lastListTicketUsersReq.UserId != 3001 {
		t.Fatalf("unexpected request: %+v", fake.lastListTicketUsersReq)
	}
}

func TestAddTicketUserCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		addTicketUserResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewAddTicketUserLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.AddTicketUser(&types.AddTicketUserReq{
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
	if fake.lastAddTicketUserReq == nil || fake.lastAddTicketUserReq.UserId != 3001 || fake.lastAddTicketUserReq.RelName != "购票人B" {
		t.Fatalf("unexpected request: %+v", fake.lastAddTicketUserReq)
	}
}

func TestDeleteTicketUserCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		listTicketUsersResp: &userrpc.ListTicketUsersResp{
			List: []*userrpc.TicketUserInfo{
				{Id: 302, UserId: 3001, RelName: "购票人B"},
			},
		},
		deleteTicketUserResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewDeleteTicketUserLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.DeleteTicketUser(&types.DeleteTicketUserReq{ID: 302})
	if err != nil {
		t.Fatalf("DeleteTicketUser returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastListTicketUsersReq == nil || fake.lastListTicketUsersReq.UserId != 3001 {
		t.Fatalf("unexpected list ticket users request: %+v", fake.lastListTicketUsersReq)
	}
	if fake.lastDeleteTicketUserReq == nil || fake.lastDeleteTicketUserReq.Id != 302 {
		t.Fatalf("unexpected request: %+v", fake.lastDeleteTicketUserReq)
	}
}

func TestDeleteTicketUserRejectsNonOwnedTicketUser(t *testing.T) {
	fake := &fakeUserRPC{
		listTicketUsersResp: &userrpc.ListTicketUsersResp{
			List: []*userrpc.TicketUserInfo{
				{Id: 401, UserId: 3001, RelName: "购票人C"},
			},
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewDeleteTicketUserLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	_, err := logic.DeleteTicketUser(&types.DeleteTicketUserReq{ID: 999})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated, got %s", status.Code(err))
	}
	if status.Convert(err).Message() != xerr.ErrUnauthorized.Error() {
		t.Fatalf("expected unauthorized message, got %q", status.Convert(err).Message())
	}
	if fake.lastDeleteTicketUserReq != nil {
		t.Fatalf("expected delete rpc not called, got %+v", fake.lastDeleteTicketUserReq)
	}
}
