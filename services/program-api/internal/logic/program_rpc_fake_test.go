package logic

import (
	"context"

	"damai-go/services/program-rpc/programrpc"

	"google.golang.org/grpc"
)

type fakeProgramRPC struct {
	listProgramCategoriesResp    *programrpc.ProgramCategoryListResp
	listProgramCategoriesErr     error
	lastListProgramCategoriesReq *programrpc.Empty

	listHomeProgramsResp    *programrpc.ProgramHomeListResp
	listHomeProgramsErr     error
	lastListHomeProgramsReq *programrpc.ListHomeProgramsReq

	pageProgramsResp    *programrpc.ProgramPageResp
	pageProgramsErr     error
	lastPageProgramsReq *programrpc.PageProgramsReq

	getProgramDetailResp    *programrpc.ProgramDetailInfo
	getProgramDetailErr     error
	lastGetProgramDetailReq *programrpc.GetProgramDetailReq

	listTicketCategoriesByProgramResp    *programrpc.TicketCategoryDetailListResp
	listTicketCategoriesByProgramErr     error
	lastListTicketCategoriesByProgramReq *programrpc.ListTicketCategoriesByProgramReq

	autoAssignAndFreezeSeatsResp    *programrpc.AutoAssignAndFreezeSeatsResp
	autoAssignAndFreezeSeatsErr     error
	lastAutoAssignAndFreezeSeatsReq *programrpc.AutoAssignAndFreezeSeatsReq

	releaseSeatFreezeResp    *programrpc.ReleaseSeatFreezeResp
	releaseSeatFreezeErr     error
	lastReleaseSeatFreezeReq *programrpc.ReleaseSeatFreezeReq
}

func (f *fakeProgramRPC) ListProgramCategories(ctx context.Context, in *programrpc.Empty, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	f.lastListProgramCategoriesReq = in
	return f.listProgramCategoriesResp, f.listProgramCategoriesErr
}

func (f *fakeProgramRPC) ListHomePrograms(ctx context.Context, in *programrpc.ListHomeProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramHomeListResp, error) {
	f.lastListHomeProgramsReq = in
	return f.listHomeProgramsResp, f.listHomeProgramsErr
}

func (f *fakeProgramRPC) PagePrograms(ctx context.Context, in *programrpc.PageProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramPageResp, error) {
	f.lastPageProgramsReq = in
	return f.pageProgramsResp, f.pageProgramsErr
}

func (f *fakeProgramRPC) GetProgramDetail(ctx context.Context, in *programrpc.GetProgramDetailReq, opts ...grpc.CallOption) (*programrpc.ProgramDetailInfo, error) {
	f.lastGetProgramDetailReq = in
	return f.getProgramDetailResp, f.getProgramDetailErr
}

func (f *fakeProgramRPC) ListTicketCategoriesByProgram(ctx context.Context, in *programrpc.ListTicketCategoriesByProgramReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailListResp, error) {
	f.lastListTicketCategoriesByProgramReq = in
	return f.listTicketCategoriesByProgramResp, f.listTicketCategoriesByProgramErr
}

func (f *fakeProgramRPC) AutoAssignAndFreezeSeats(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq, opts ...grpc.CallOption) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
	f.lastAutoAssignAndFreezeSeatsReq = in
	return f.autoAssignAndFreezeSeatsResp, f.autoAssignAndFreezeSeatsErr
}

func (f *fakeProgramRPC) ReleaseSeatFreeze(ctx context.Context, in *programrpc.ReleaseSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ReleaseSeatFreezeResp, error) {
	f.lastReleaseSeatFreezeReq = in
	return f.releaseSeatFreezeResp, f.releaseSeatFreezeErr
}
