package integration_test

import (
	"context"

	"damai-go/services/program-rpc/programrpc"

	"google.golang.org/grpc"
)

type fakeProgramRPC struct {
	idResp   *programrpc.IdResp
	idErr    error
	boolResp *programrpc.BoolResp
	boolErr  error

	createProgramResp    *programrpc.CreateProgramResp
	createProgramErr     error
	lastCreateProgramReq *programrpc.CreateProgramReq

	lastInvalidProgramReq *programrpc.ProgramInvalidReq

	updateProgramResp    *programrpc.BoolResp
	updateProgramErr     error
	lastUpdateProgramReq *programrpc.UpdateProgramReq

	listProgramCategoriesResp    *programrpc.ProgramCategoryListResp
	listProgramCategoriesErr     error
	lastListProgramCategoriesReq *programrpc.Empty

	listProgramCategoriesByTypeResp    *programrpc.ProgramCategoryListResp
	listProgramCategoriesByTypeErr     error
	lastListProgramCategoriesByTypeReq *programrpc.ProgramCategoryTypeReq

	listProgramCategoriesByParentResp    *programrpc.ProgramCategoryListResp
	listProgramCategoriesByParentErr     error
	lastListProgramCategoriesByParentReq *programrpc.ParentProgramCategoryReq

	lastBatchCreateProgramCategoriesReq *programrpc.ProgramCategoryBatchSaveReq

	listHomeProgramsResp    *programrpc.ProgramHomeListResp
	listHomeProgramsErr     error
	lastListHomeProgramsReq *programrpc.ListHomeProgramsReq

	pageProgramsResp    *programrpc.ProgramPageResp
	pageProgramsErr     error
	lastPageProgramsReq *programrpc.PageProgramsReq

	getProgramDetailViewResp    *programrpc.ProgramDetailViewInfo
	getProgramDetailViewErr     error
	lastGetProgramDetailViewReq *programrpc.GetProgramDetailViewReq

	getProgramPreorderResp    *programrpc.ProgramPreorderInfo
	getProgramPreorderErr     error
	lastGetProgramPreorderReq *programrpc.GetProgramPreorderReq

	listProgramShowTimesForRushResp    *programrpc.ListProgramShowTimesForRushResp
	listProgramShowTimesForRushErr     error
	lastListProgramShowTimesForRushReq *programrpc.ListProgramShowTimesForRushReq

	listTicketCategoriesByProgramResp    *programrpc.TicketCategoryDetailListResp
	listTicketCategoriesByProgramErr     error
	lastListTicketCategoriesByProgramReq *programrpc.ListTicketCategoriesByProgramReq

	createProgramShowTimeResp    *programrpc.IdResp
	createProgramShowTimeErr     error
	lastCreateProgramShowTimeReq *programrpc.ProgramShowTimeAddReq

	createTicketCategoryResp    *programrpc.IdResp
	createTicketCategoryErr     error
	lastCreateTicketCategoryReq *programrpc.TicketCategoryAddReq

	getTicketCategoryDetailResp    *programrpc.TicketCategoryDetailInfo
	getTicketCategoryDetailErr     error
	lastGetTicketCategoryDetailReq *programrpc.TicketCategoryReq

	createSeatResp    *programrpc.IdResp
	createSeatErr     error
	lastCreateSeatReq *programrpc.SeatAddReq

	batchCreateSeatsResp    *programrpc.BoolResp
	batchCreateSeatsErr     error
	lastBatchCreateSeatsReq *programrpc.SeatBatchAddReq

	getSeatRelateInfoResp    *programrpc.SeatRelateInfo
	getSeatRelateInfoErr     error
	lastGetSeatRelateInfoReq *programrpc.SeatListReq

	resetProgramResp    *programrpc.BoolResp
	resetProgramErr     error
	lastResetProgramReq *programrpc.ProgramResetReq

	autoAssignAndFreezeSeatsResp    *programrpc.AutoAssignAndFreezeSeatsResp
	autoAssignAndFreezeSeatsErr     error
	lastAutoAssignAndFreezeSeatsReq *programrpc.AutoAssignAndFreezeSeatsReq

	releaseSeatFreezeResp    *programrpc.ReleaseSeatFreezeResp
	releaseSeatFreezeErr     error
	lastReleaseSeatFreezeReq *programrpc.ReleaseSeatFreezeReq
}

func (f *fakeProgramRPC) CreateProgram(ctx context.Context, in *programrpc.CreateProgramReq, opts ...grpc.CallOption) (*programrpc.CreateProgramResp, error) {
	f.lastCreateProgramReq = in
	return f.createProgramResp, f.createProgramErr
}

func (f *fakeProgramRPC) InvalidProgram(ctx context.Context, in *programrpc.ProgramInvalidReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	f.lastInvalidProgramReq = in
	return f.boolResp, f.boolErr
}

func (f *fakeProgramRPC) UpdateProgram(ctx context.Context, in *programrpc.UpdateProgramReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	f.lastUpdateProgramReq = in
	return f.updateProgramResp, f.updateProgramErr
}

func (f *fakeProgramRPC) ListProgramCategories(ctx context.Context, in *programrpc.Empty, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	f.lastListProgramCategoriesReq = in
	return f.listProgramCategoriesResp, f.listProgramCategoriesErr
}

func (f *fakeProgramRPC) ListProgramCategoriesByType(ctx context.Context, in *programrpc.ProgramCategoryTypeReq, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	f.lastListProgramCategoriesByTypeReq = in
	return f.listProgramCategoriesByTypeResp, f.listProgramCategoriesByTypeErr
}

func (f *fakeProgramRPC) ListProgramCategoriesByParent(ctx context.Context, in *programrpc.ParentProgramCategoryReq, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	f.lastListProgramCategoriesByParentReq = in
	return f.listProgramCategoriesByParentResp, f.listProgramCategoriesByParentErr
}

func (f *fakeProgramRPC) BatchCreateProgramCategories(ctx context.Context, in *programrpc.ProgramCategoryBatchSaveReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	f.lastBatchCreateProgramCategoriesReq = in
	return f.boolResp, f.boolErr
}

func (f *fakeProgramRPC) ListHomePrograms(ctx context.Context, in *programrpc.ListHomeProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramHomeListResp, error) {
	f.lastListHomeProgramsReq = in
	return f.listHomeProgramsResp, f.listHomeProgramsErr
}

func (f *fakeProgramRPC) PagePrograms(ctx context.Context, in *programrpc.PageProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramPageResp, error) {
	f.lastPageProgramsReq = in
	return f.pageProgramsResp, f.pageProgramsErr
}

func (f *fakeProgramRPC) GetProgramDetailView(ctx context.Context, in *programrpc.GetProgramDetailViewReq, opts ...grpc.CallOption) (*programrpc.ProgramDetailViewInfo, error) {
	f.lastGetProgramDetailViewReq = in
	return f.getProgramDetailViewResp, f.getProgramDetailViewErr
}

func (f *fakeProgramRPC) GetProgramPreorder(ctx context.Context, in *programrpc.GetProgramPreorderReq, opts ...grpc.CallOption) (*programrpc.ProgramPreorderInfo, error) {
	f.lastGetProgramPreorderReq = in
	return f.getProgramPreorderResp, f.getProgramPreorderErr
}

func (f *fakeProgramRPC) ListProgramShowTimesForRush(ctx context.Context, in *programrpc.ListProgramShowTimesForRushReq, opts ...grpc.CallOption) (*programrpc.ListProgramShowTimesForRushResp, error) {
	f.lastListProgramShowTimesForRushReq = in
	return f.listProgramShowTimesForRushResp, f.listProgramShowTimesForRushErr
}

func (f *fakeProgramRPC) ListTicketCategoriesByProgram(ctx context.Context, in *programrpc.ListTicketCategoriesByProgramReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailListResp, error) {
	f.lastListTicketCategoriesByProgramReq = in
	return f.listTicketCategoriesByProgramResp, f.listTicketCategoriesByProgramErr
}

func (f *fakeProgramRPC) CreateProgramShowTime(ctx context.Context, in *programrpc.ProgramShowTimeAddReq, opts ...grpc.CallOption) (*programrpc.IdResp, error) {
	f.lastCreateProgramShowTimeReq = in
	return f.createProgramShowTimeResp, f.createProgramShowTimeErr
}

func (f *fakeProgramRPC) UpdateProgramShowTime(ctx context.Context, in *programrpc.UpdateProgramShowTimeReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return f.boolResp, f.boolErr
}

func (f *fakeProgramRPC) PrimeSeatLedger(ctx context.Context, in *programrpc.PrimeSeatLedgerReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return f.boolResp, f.boolErr
}

func (f *fakeProgramRPC) CreateTicketCategory(ctx context.Context, in *programrpc.TicketCategoryAddReq, opts ...grpc.CallOption) (*programrpc.IdResp, error) {
	f.lastCreateTicketCategoryReq = in
	return f.createTicketCategoryResp, f.createTicketCategoryErr
}

func (f *fakeProgramRPC) GetTicketCategoryDetail(ctx context.Context, in *programrpc.TicketCategoryReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailInfo, error) {
	f.lastGetTicketCategoryDetailReq = in
	return f.getTicketCategoryDetailResp, f.getTicketCategoryDetailErr
}

func (f *fakeProgramRPC) CreateSeat(ctx context.Context, in *programrpc.SeatAddReq, opts ...grpc.CallOption) (*programrpc.IdResp, error) {
	f.lastCreateSeatReq = in
	return f.createSeatResp, f.createSeatErr
}

func (f *fakeProgramRPC) BatchCreateSeats(ctx context.Context, in *programrpc.SeatBatchAddReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	f.lastBatchCreateSeatsReq = in
	return f.batchCreateSeatsResp, f.batchCreateSeatsErr
}

func (f *fakeProgramRPC) GetSeatRelateInfo(ctx context.Context, in *programrpc.SeatListReq, opts ...grpc.CallOption) (*programrpc.SeatRelateInfo, error) {
	f.lastGetSeatRelateInfoReq = in
	return f.getSeatRelateInfoResp, f.getSeatRelateInfoErr
}

func (f *fakeProgramRPC) ResetProgram(ctx context.Context, in *programrpc.ProgramResetReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	f.lastResetProgramReq = in
	return f.resetProgramResp, f.resetProgramErr
}

func (f *fakeProgramRPC) AutoAssignAndFreezeSeats(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq, opts ...grpc.CallOption) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
	f.lastAutoAssignAndFreezeSeatsReq = in
	return f.autoAssignAndFreezeSeatsResp, f.autoAssignAndFreezeSeatsErr
}

func (f *fakeProgramRPC) ReleaseSeatFreeze(ctx context.Context, in *programrpc.ReleaseSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ReleaseSeatFreezeResp, error) {
	f.lastReleaseSeatFreezeReq = in
	return f.releaseSeatFreezeResp, f.releaseSeatFreezeErr
}

func (f *fakeProgramRPC) EvaluateRefundRule(ctx context.Context, in *programrpc.EvaluateRefundRuleReq, opts ...grpc.CallOption) (*programrpc.EvaluateRefundRuleResp, error) {
	return nil, nil
}

func (f *fakeProgramRPC) ConfirmSeatFreeze(ctx context.Context, in *programrpc.ConfirmSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ConfirmSeatFreezeResp, error) {
	return nil, nil
}

func (f *fakeProgramRPC) ReleaseSoldSeats(ctx context.Context, in *programrpc.ReleaseSoldSeatsReq, opts ...grpc.CallOption) (*programrpc.ReleaseSoldSeatsResp, error) {
	return nil, nil
}
