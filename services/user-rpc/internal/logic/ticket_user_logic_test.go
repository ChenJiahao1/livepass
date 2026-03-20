package logic

import (
	"context"
	"testing"

	"damai-go/services/user-rpc/pb"
)

func TestAddTicketUserCreatesRecord(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000050",
		Password: "123456",
	})

	l := NewAddTicketUserLogic(context.Background(), svcCtx)
	resp, err := l.AddTicketUser(&pb.AddTicketUserReq{
		UserId:   user.Id,
		RelName:  "王五",
		IdType:   1,
		IdNumber: "440101199303031234",
	})
	if err != nil {
		t.Fatalf("AddTicketUser returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	list, err := svcCtx.DTicketUserModel.FindByUserId(context.Background(), user.Id)
	if err != nil {
		t.Fatalf("FindByUserId returned error: %v", err)
	}
	if len(list) != 1 || list[0].RelName != "王五" {
		t.Fatalf("unexpected ticket users: %+v", list)
	}
}

func TestDeleteTicketUserRemovesRecord(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000051",
		Password: "123456",
	})
	ticketUser := mustSeedTicketUser(t, svcCtx, user.Id, "赵六", 1, "110101199404041234")

	l := NewDeleteTicketUserLogic(context.Background(), svcCtx)
	resp, err := l.DeleteTicketUser(&pb.DeleteTicketUserReq{Id: ticketUser.Id})
	if err != nil {
		t.Fatalf("DeleteTicketUser returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	list, err := svcCtx.DTicketUserModel.FindByUserId(context.Background(), user.Id)
	if err == nil && len(list) != 0 {
		t.Fatalf("expected no active ticket users, got %+v", list)
	}
}

func TestListTicketUsersReturnsRecords(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000052",
		Password: "123456",
	})
	mustSeedTicketUser(t, svcCtx, user.Id, "甲", 1, "110101199505051111")
	mustSeedTicketUser(t, svcCtx, user.Id, "乙", 1, "110101199505052222")

	l := NewListTicketUsersLogic(context.Background(), svcCtx)
	resp, err := l.ListTicketUsers(&pb.ListTicketUsersReq{UserId: user.Id})
	if err != nil {
		t.Fatalf("ListTicketUsers returned error: %v", err)
	}
	if len(resp.List) != 2 {
		t.Fatalf("expected 2 ticket users, got %d", len(resp.List))
	}
}

func TestGetUserAndTicketUserListReturnsAggregate(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Name:     "aggregate-user",
		Mobile:   "13800000053",
		Password: "123456",
	})
	mustSeedTicketUser(t, svcCtx, user.Id, "聚合购票人", 1, "110101199606061234")

	l := NewGetUserAndTicketUserListLogic(context.Background(), svcCtx)
	resp, err := l.GetUserAndTicketUserList(&pb.GetUserAndTicketUserListReq{UserId: user.Id})
	if err != nil {
		t.Fatalf("GetUserAndTicketUserList returned error: %v", err)
	}
	if resp.UserVo == nil || resp.UserVo.Id != user.Id {
		t.Fatalf("unexpected user vo: %+v", resp.UserVo)
	}
	if len(resp.TicketUserVoList) != 1 {
		t.Fatalf("expected 1 ticket user, got %d", len(resp.TicketUserVoList))
	}
}
