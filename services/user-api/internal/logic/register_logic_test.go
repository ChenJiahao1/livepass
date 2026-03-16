package logic

import (
	"context"
	"testing"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc"
)

func TestRegisterCallsRpc(t *testing.T) {
	fake := &fakeUserRPC{
		registerResp: &userrpc.BoolResp{Success: true},
	}
	logic := NewRegisterLogic(context.Background(), &svc.ServiceContext{
		UserRpc: fake,
	})

	resp, err := logic.Register(&types.UserRegisterReq{
		Mobile:   "13800000002",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if fake.lastReq == nil {
		t.Fatalf("expected rpc request")
	}
	if fake.lastReq.Mobile != "13800000002" {
		t.Fatalf("unexpected mobile: %s", fake.lastReq.Mobile)
	}
	if fake.lastReq.Password != "123456" {
		t.Fatalf("unexpected password")
	}
}

type fakeUserRPC struct {
	registerResp *userrpc.BoolResp
	registerErr  error
	lastReq      *userrpc.RegisterReq
}

func (f *fakeUserRPC) Register(ctx context.Context, in *userrpc.RegisterReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastReq = in
	return f.registerResp, f.registerErr
}

func (f *fakeUserRPC) Login(ctx context.Context, in *userrpc.LoginReq, opts ...grpc.CallOption) (*userrpc.LoginResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) GetUserById(ctx context.Context, in *userrpc.GetUserByIdReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) GetUserByMobile(ctx context.Context, in *userrpc.GetUserByMobileReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) Logout(ctx context.Context, in *userrpc.LogoutReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) UpdateUser(ctx context.Context, in *userrpc.UpdateUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) UpdatePassword(ctx context.Context, in *userrpc.UpdatePasswordReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) UpdateEmail(ctx context.Context, in *userrpc.UpdateEmailReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) UpdateMobile(ctx context.Context, in *userrpc.UpdateMobileReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) Authentication(ctx context.Context, in *userrpc.AuthenticationReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) ListTicketUsers(ctx context.Context, in *userrpc.ListTicketUsersReq, opts ...grpc.CallOption) (*userrpc.ListTicketUsersResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) AddTicketUser(ctx context.Context, in *userrpc.AddTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) DeleteTicketUser(ctx context.Context, in *userrpc.DeleteTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) GetUserAndTicketUserList(ctx context.Context, in *userrpc.GetUserAndTicketUserListReq, opts ...grpc.CallOption) (*userrpc.GetUserAndTicketUserListResp, error) {
	panic("unexpected call")
}
