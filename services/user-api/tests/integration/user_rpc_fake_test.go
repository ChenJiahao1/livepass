package integration_test

import (
	"context"

	"damai-go/services/user-rpc/userrpc"

	"google.golang.org/grpc"
)

type fakeUserRPC struct {
	registerResp    *userrpc.BoolResp
	registerErr     error
	lastRegisterReq *userrpc.RegisterReq

	existResp    *userrpc.BoolResp
	existErr     error
	lastExistReq *userrpc.ExistReq

	loginResp    *userrpc.LoginResp
	loginErr     error
	lastLoginReq *userrpc.LoginReq

	getUserByIDResp    *userrpc.UserInfo
	getUserByIDErr     error
	lastGetUserByIDReq *userrpc.GetUserByIdReq

	getUserByMobileResp    *userrpc.UserInfo
	getUserByMobileErr     error
	lastGetUserByMobileReq *userrpc.GetUserByMobileReq

	logoutResp    *userrpc.BoolResp
	logoutErr     error
	lastLogoutReq *userrpc.LogoutReq

	updateUserResp    *userrpc.BoolResp
	updateUserErr     error
	lastUpdateUserReq *userrpc.UpdateUserReq

	updatePasswordResp    *userrpc.BoolResp
	updatePasswordErr     error
	lastUpdatePasswordReq *userrpc.UpdatePasswordReq

	updateEmailResp    *userrpc.BoolResp
	updateEmailErr     error
	lastUpdateEmailReq *userrpc.UpdateEmailReq

	updateMobileResp    *userrpc.BoolResp
	updateMobileErr     error
	lastUpdateMobileReq *userrpc.UpdateMobileReq

	authenticationResp    *userrpc.BoolResp
	authenticationErr     error
	lastAuthenticationReq *userrpc.AuthenticationReq

	listTicketUsersResp    *userrpc.ListTicketUsersResp
	listTicketUsersErr     error
	lastListTicketUsersReq *userrpc.ListTicketUsersReq

	addTicketUserResp    *userrpc.BoolResp
	addTicketUserErr     error
	lastAddTicketUserReq *userrpc.AddTicketUserReq

	deleteTicketUserResp    *userrpc.BoolResp
	deleteTicketUserErr     error
	lastDeleteTicketUserReq *userrpc.DeleteTicketUserReq

	getUserAndTicketUserListResp    *userrpc.GetUserAndTicketUserListResp
	getUserAndTicketUserListErr     error
	lastGetUserAndTicketUserListReq *userrpc.GetUserAndTicketUserListReq
}

func (f *fakeUserRPC) Register(ctx context.Context, in *userrpc.RegisterReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastRegisterReq = in
	return f.registerResp, f.registerErr
}

func (f *fakeUserRPC) Exist(ctx context.Context, in *userrpc.ExistReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastExistReq = in
	return f.existResp, f.existErr
}

func (f *fakeUserRPC) Login(ctx context.Context, in *userrpc.LoginReq, opts ...grpc.CallOption) (*userrpc.LoginResp, error) {
	f.lastLoginReq = in
	return f.loginResp, f.loginErr
}

func (f *fakeUserRPC) GetUserById(ctx context.Context, in *userrpc.GetUserByIdReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	f.lastGetUserByIDReq = in
	return f.getUserByIDResp, f.getUserByIDErr
}

func (f *fakeUserRPC) GetUserByMobile(ctx context.Context, in *userrpc.GetUserByMobileReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	f.lastGetUserByMobileReq = in
	return f.getUserByMobileResp, f.getUserByMobileErr
}

func (f *fakeUserRPC) Logout(ctx context.Context, in *userrpc.LogoutReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastLogoutReq = in
	return f.logoutResp, f.logoutErr
}

func (f *fakeUserRPC) UpdateUser(ctx context.Context, in *userrpc.UpdateUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastUpdateUserReq = in
	return f.updateUserResp, f.updateUserErr
}

func (f *fakeUserRPC) UpdatePassword(ctx context.Context, in *userrpc.UpdatePasswordReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastUpdatePasswordReq = in
	return f.updatePasswordResp, f.updatePasswordErr
}

func (f *fakeUserRPC) UpdateEmail(ctx context.Context, in *userrpc.UpdateEmailReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastUpdateEmailReq = in
	return f.updateEmailResp, f.updateEmailErr
}

func (f *fakeUserRPC) UpdateMobile(ctx context.Context, in *userrpc.UpdateMobileReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastUpdateMobileReq = in
	return f.updateMobileResp, f.updateMobileErr
}

func (f *fakeUserRPC) Authentication(ctx context.Context, in *userrpc.AuthenticationReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastAuthenticationReq = in
	return f.authenticationResp, f.authenticationErr
}

func (f *fakeUserRPC) ListTicketUsers(ctx context.Context, in *userrpc.ListTicketUsersReq, opts ...grpc.CallOption) (*userrpc.ListTicketUsersResp, error) {
	f.lastListTicketUsersReq = in
	return f.listTicketUsersResp, f.listTicketUsersErr
}

func (f *fakeUserRPC) AddTicketUser(ctx context.Context, in *userrpc.AddTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastAddTicketUserReq = in
	return f.addTicketUserResp, f.addTicketUserErr
}

func (f *fakeUserRPC) DeleteTicketUser(ctx context.Context, in *userrpc.DeleteTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	f.lastDeleteTicketUserReq = in
	return f.deleteTicketUserResp, f.deleteTicketUserErr
}

func (f *fakeUserRPC) GetUserAndTicketUserList(ctx context.Context, in *userrpc.GetUserAndTicketUserListReq, opts ...grpc.CallOption) (*userrpc.GetUserAndTicketUserListResp, error) {
	f.lastGetUserAndTicketUserListReq = in
	return f.getUserAndTicketUserListResp, f.getUserAndTicketUserListErr
}
