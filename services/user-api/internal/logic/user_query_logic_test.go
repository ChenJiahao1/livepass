package logic

import (
	"context"
	"testing"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"
)

func TestGetUserByIDMapsResponse(t *testing.T) {
	fake := &fakeUserRPC{
		getUserByIDResp: &userrpc.UserInfo{
			Id:                      101,
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
	logic := NewGetUserByIDLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.GetUserByID(&types.GetUserByIDReq{ID: 101})
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if resp == nil || resp.ID != 101 || resp.RelName != "张三实名" || resp.Email != "query@example.com" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastGetUserByIDReq == nil || fake.lastGetUserByIDReq.Id != 101 {
		t.Fatalf("unexpected request: %+v", fake.lastGetUserByIDReq)
	}
}

func TestGetUserByMobileMapsResponse(t *testing.T) {
	fake := &fakeUserRPC{
		getUserByMobileResp: &userrpc.UserInfo{
			Id:     102,
			Name:   "李四",
			Mobile: "13800000011",
		},
	}
	logic := NewGetUserByMobileLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.GetUserByMobile(&types.GetUserByMobileReq{Mobile: "13800000011"})
	if err != nil {
		t.Fatalf("GetUserByMobile returned error: %v", err)
	}
	if resp == nil || resp.ID != 102 || resp.Mobile != "13800000011" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastGetUserByMobileReq == nil || fake.lastGetUserByMobileReq.Mobile != "13800000011" {
		t.Fatalf("unexpected request: %+v", fake.lastGetUserByMobileReq)
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
	logic := NewGetUserAndTicketUserListLogic(context.Background(), &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.GetUserAndTicketUserList(&types.GetUserAndTicketUserListReq{UserID: 103})
	if err != nil {
		t.Fatalf("GetUserAndTicketUserList returned error: %v", err)
	}
	if resp == nil || resp.UserVo.ID != 103 || len(resp.TicketUserVoList) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.TicketUserVoList[0].RelName != "购票人A" {
		t.Fatalf("unexpected ticket user list: %+v", resp.TicketUserVoList)
	}
	if fake.lastGetUserAndTicketUserListReq == nil || fake.lastGetUserAndTicketUserListReq.UserId != 103 {
		t.Fatalf("unexpected request: %+v", fake.lastGetUserAndTicketUserListReq)
	}
}
