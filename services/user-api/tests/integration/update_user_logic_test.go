package integration_test

import (
	"context"
	"testing"

	"damai-go/pkg/xmiddleware"
	logicpkg "damai-go/services/user-api/internal/logic"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"
)

func TestUpdateUserCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		updateUserResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewUpdateUserLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.UpdateUser(&types.UpdateUserReq{
		Name:    "new-name",
		Gender:  2,
		Mobile:  "13800000020",
		Address: "Hangzhou",
	})
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastUpdateUserReq == nil || fake.lastUpdateUserReq.Id != 3001 || fake.lastUpdateUserReq.Address != "Hangzhou" {
		t.Fatalf("unexpected update user request: %+v", fake.lastUpdateUserReq)
	}
}

func TestUpdatePasswordCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		updatePasswordResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewUpdatePasswordLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.UpdatePassword(&types.UpdatePasswordReq{
		Password: "654321",
	})
	if err != nil {
		t.Fatalf("UpdatePassword returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastUpdatePasswordReq == nil || fake.lastUpdatePasswordReq.Id != 3001 || fake.lastUpdatePasswordReq.Password != "654321" {
		t.Fatalf("unexpected update password request: %+v", fake.lastUpdatePasswordReq)
	}
}

func TestUpdateEmailCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		updateEmailResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewUpdateEmailLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.UpdateEmail(&types.UpdateEmailReq{
		Email: "new@example.com",
	})
	if err != nil {
		t.Fatalf("UpdateEmail returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastUpdateEmailReq == nil || fake.lastUpdateEmailReq.Id != 3001 || fake.lastUpdateEmailReq.Email != "new@example.com" {
		t.Fatalf("unexpected update email request: %+v", fake.lastUpdateEmailReq)
	}
}

func TestUpdateMobileCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		updateMobileResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewUpdateMobileLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.UpdateMobile(&types.UpdateMobileReq{
		Mobile: "13800000021",
	})
	if err != nil {
		t.Fatalf("UpdateMobile returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastUpdateMobileReq == nil || fake.lastUpdateMobileReq.Id != 3001 || fake.lastUpdateMobileReq.Mobile != "13800000021" {
		t.Fatalf("unexpected update mobile request: %+v", fake.lastUpdateMobileReq)
	}
}

func TestAuthenticationCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		authenticationResp: &userrpc.BoolResp{Success: true},
	}
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	logic := logicpkg.NewAuthenticationLogic(ctx, &svc.ServiceContext{UserRpc: fake})

	resp, err := logic.Authentication(&types.AuthenticationReq{
		RelName:  "王五",
		IdNumber: "110101199404041234",
	})
	if err != nil {
		t.Fatalf("Authentication returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fake.lastAuthenticationReq == nil || fake.lastAuthenticationReq.Id != 3001 || fake.lastAuthenticationReq.RelName != "王五" {
		t.Fatalf("unexpected authentication request: %+v", fake.lastAuthenticationReq)
	}
}
