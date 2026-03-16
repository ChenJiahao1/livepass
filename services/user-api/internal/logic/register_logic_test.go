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

func (f *fakeUserRPC) UpdateUser(ctx context.Context, in *userrpc.UpdateUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) ListTicketUsers(ctx context.Context, in *userrpc.ListTicketUsersReq, opts ...grpc.CallOption) (*userrpc.ListTicketUsersResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) AddTicketUser(ctx context.Context, in *userrpc.AddTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) GetCaptcha(ctx context.Context, in *userrpc.GetCaptchaReq, opts ...grpc.CallOption) (*userrpc.CaptchaResp, error) {
	panic("unexpected call")
}

func (f *fakeUserRPC) VerifyCaptcha(ctx context.Context, in *userrpc.VerifyCaptchaReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	panic("unexpected call")
}
