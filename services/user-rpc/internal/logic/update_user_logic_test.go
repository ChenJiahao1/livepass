package logic

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"damai-go/services/user-rpc/pb"
)

func TestUpdateUserUpdatesProfileAndMobileMapping(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Name:     "before",
		Mobile:   "13800000040",
		Password: "123456",
	})

	l := NewUpdateUserLogic(context.Background(), svcCtx)
	resp, err := l.UpdateUser(&pb.UpdateUserReq{
		Id:      user.Id,
		Name:    "after",
		Mobile:  "13800000041",
		Address: "Hangzhou",
	})
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	updated, err := svcCtx.DUserModel.FindOne(context.Background(), user.Id)
	if err != nil {
		t.Fatalf("FindOne returned error: %v", err)
	}
	if !updated.Name.Valid || updated.Name.String != "after" {
		t.Fatalf("unexpected name: %+v", updated.Name)
	}
	if updated.Mobile != "13800000041" {
		t.Fatalf("unexpected mobile: %s", updated.Mobile)
	}
	mobileMapping, err := svcCtx.DUserMobileModel.FindOneByMobile(context.Background(), "13800000041")
	if err != nil {
		t.Fatalf("FindOneByMobile returned error: %v", err)
	}
	if mobileMapping.UserId != user.Id {
		t.Fatalf("unexpected mapping user id: %d", mobileMapping.UserId)
	}
}

func TestUpdateEmailRejectsDuplicate(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	target := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000042",
		Password: "123456",
	})
	mustSeedUser(t, svcCtx, userSeed{
		Mobile:      "13800000043",
		Email:       "dup@example.com",
		EmailStatus: 1,
		Password:    "123456",
	})

	l := NewUpdateEmailLogic(context.Background(), svcCtx)
	_, err := l.UpdateEmail(&pb.UpdateEmailReq{
		Id:          target.Id,
		Email:       "dup@example.com",
		EmailStatus: 1,
	})
	if err == nil {
		t.Fatalf("expected already exists error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}

func TestUpdateMobileRejectsDuplicate(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	target := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000044",
		Password: "123456",
	})
	mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000045",
		Password: "123456",
	})

	l := NewUpdateMobileLogic(context.Background(), svcCtx)
	_, err := l.UpdateMobile(&pb.UpdateMobileReq{
		Id:     target.Id,
		Mobile: "13800000045",
	})
	if err == nil {
		t.Fatalf("expected already exists error")
	}
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected already exists code, got %s", status.Code(err))
	}
}

func TestUpdatePasswordUpdatesHash(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000046",
		Password: "123456",
	})

	l := NewUpdatePasswordLogic(context.Background(), svcCtx)
	resp, err := l.UpdatePassword(&pb.UpdatePasswordReq{
		Id:       user.Id,
		Password: "654321",
	})
	if err != nil {
		t.Fatalf("UpdatePassword returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	updated, err := svcCtx.DUserModel.FindOne(context.Background(), user.Id)
	if err != nil {
		t.Fatalf("FindOne returned error: %v", err)
	}
	if updated.Password.String != md5Hex("654321") {
		t.Fatalf("unexpected password hash: %s", updated.Password.String)
	}
}

func TestAuthenticationUpdatesIdentity(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	resetUserDomainState(t)
	user := mustSeedUser(t, svcCtx, userSeed{
		Mobile:   "13800000047",
		Password: "123456",
	})

	l := NewAuthenticationLogic(context.Background(), svcCtx)
	resp, err := l.Authentication(&pb.AuthenticationReq{
		Id:       user.Id,
		RelName:  "李四",
		IdNumber: "320101199202021234",
	})
	if err != nil {
		t.Fatalf("Authentication returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}

	updated, err := svcCtx.DUserModel.FindOne(context.Background(), user.Id)
	if err != nil {
		t.Fatalf("FindOne returned error: %v", err)
	}
	if !updated.RelName.Valid || updated.RelName.String != "李四" {
		t.Fatalf("unexpected rel name: %+v", updated.RelName)
	}
	if updated.RelAuthenticationStatus != 1 {
		t.Fatalf("expected authenticated status, got %d", updated.RelAuthenticationStatus)
	}
}
