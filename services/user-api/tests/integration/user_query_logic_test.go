package integration_test

import (
	"context"
	"testing"

	"livepass/pkg/xmiddleware"
	logicpkg "livepass/services/user-api/internal/logic"
	"livepass/services/user-api/internal/svc"
	"livepass/services/user-api/internal/types"
	"livepass/services/user-rpc/userrpc"
)

func TestGetUserByIDMapsResponse(t *testing.T) {
	fake := &fakeUserRPC{
		getUserByIDResp: &userrpc.UserInfo{
			Id:                      3001,
			Name:                    "张三",
			RelName:                 "张三实名",
			Gender:                  1,
			Mobile:                  "13800000010",
			EmailStatus:             1,
			Email:                   "query@example.com",
			RelAuthenticationStatus: 1,
			IdNumber:                "310101199001011234",
			Address:                 "Shanghai",
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewGetUserByIDLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.GetUserByID(&types.GetUserByIDReq{})
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if resp == nil || resp.ID != 3001 || resp.RelName != "张三实名" || resp.Email != "query@example.com" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastGetUserByIDReq == nil || fake.lastGetUserByIDReq.Id != 3001 {
		t.Fatalf("unexpected request: %+v", fake.lastGetUserByIDReq)
	}
}

func TestGetUserByMobileMapsResponse(t *testing.T) {
	fake := &fakeUserRPC{
		getUserByIDResp: &userrpc.UserInfo{
			Id:     3001,
			Name:   "李四",
			Mobile: "13800000011",
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewGetUserByMobileLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.GetUserByMobile(&types.GetUserByMobileReq{})
	if err != nil {
		t.Fatalf("GetUserByMobile returned error: %v", err)
	}
	if resp == nil || resp.ID != 3001 || resp.Mobile != "13800000011" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastGetUserByIDReq == nil || fake.lastGetUserByIDReq.Id != 3001 {
		t.Fatalf("unexpected request: %+v", fake.lastGetUserByIDReq)
	}
	if fake.lastGetUserByMobileReq != nil {
		t.Fatalf("expected mobile lookup skipped, got %+v", fake.lastGetUserByMobileReq)
	}
}

func TestGetUserAndTicketUserListMapsAggregateResponse(t *testing.T) {
	fake := &fakeUserRPC{
		getUserAndTicketUserListResp: &userrpc.GetUserAndTicketUserListResp{
			UserVo: &userrpc.UserInfo{
				Id:     103,
				Name:   "聚合用户",
				Mobile: "13800000012",
			},
			TicketUserVoList: []*userrpc.TicketUserInfo{
				{
					Id:       201,
					UserId:   103,
					RelName:  "购票人A",
					IdType:   1,
					IdNumber: "110101199303031234",
				},
			},
		},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewGetUserAndTicketUserListLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.GetUserAndTicketUserList(&types.GetUserAndTicketUserListReq{})
	if err != nil {
		t.Fatalf("GetUserAndTicketUserList returned error: %v", err)
	}
	if resp == nil || resp.UserVo.ID != 103 || len(resp.TicketUserVoList) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.TicketUserVoList[0].RelName != "购票人A" {
		t.Fatalf("unexpected ticket user list: %+v", resp.TicketUserVoList)
	}
	if fake.lastGetUserAndTicketUserListReq == nil || fake.lastGetUserAndTicketUserListReq.UserId != 3001 {
		t.Fatalf("unexpected request: %+v", fake.lastGetUserAndTicketUserListReq)
	}
}
