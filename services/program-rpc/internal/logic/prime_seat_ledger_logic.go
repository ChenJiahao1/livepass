package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PrimeSeatLedgerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPrimeSeatLedgerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrimeSeatLedgerLogic {
	return &PrimeSeatLedgerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PrimeSeatLedgerLogic) PrimeSeatLedger(in *pb.PrimeSeatLedgerReq) (*pb.BoolResp, error) {
	if in == nil || in.GetShowTimeId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if err := primeSeatLedger(l.ctx, l.svcCtx, in.GetShowTimeId()); err != nil {
		return nil, mapPrimeSeatLedgerError(err)
	}

	return &pb.BoolResp{Success: true}, nil
}

func primeSeatLedger(ctx context.Context, svcCtx *svc.ServiceContext, showTimeID int64) error {
	if svcCtx == nil || svcCtx.SeatStockStore == nil || svcCtx.DProgramShowTimeModel == nil || svcCtx.DTicketCategoryModel == nil {
		return xerr.ErrProgramSeatLedgerNotReady
	}
	if showTimeID <= 0 {
		return xerr.ErrInvalidParam
	}

	if _, err := svcCtx.DProgramShowTimeModel.FindOne(ctx, showTimeID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return xerr.ErrProgramShowTimeNotFound
		}
		return err
	}

	ticketCategories, err := svcCtx.DTicketCategoryModel.FindByShowTimeId(ctx, showTimeID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return xerr.ErrProgramTicketCategoryNotFound
		}
		return err
	}

	for _, ticketCategory := range ticketCategories {
		if ticketCategory == nil || ticketCategory.Id <= 0 {
			continue
		}
		if err := svcCtx.SeatStockStore.PrimeFromDB(ctx, showTimeID, ticketCategory.Id); err != nil {
			return err
		}
	}

	return nil
}

func mapPrimeSeatLedgerError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrInvalidParam):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, xerr.ErrProgramShowTimeNotFound), errors.Is(err, xerr.ErrProgramTicketCategoryNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrProgramSeatLedgerNotReady):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, xerr.ErrInternal):
		return status.Error(codes.Internal, err.Error())
	default:
		return err
	}
}
