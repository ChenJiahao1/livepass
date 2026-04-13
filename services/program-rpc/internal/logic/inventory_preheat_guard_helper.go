package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ensureProgramInventoryMutable(ctx context.Context, svcCtx *svc.ServiceContext, programID int64) error {
	if programID <= 0 {
		return xerr.ErrInvalidParam
	}
	if svcCtx == nil || svcCtx.DProgramShowTimeModel == nil {
		return xerr.ErrInternal
	}

	showTime, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(ctx, programID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}

	return ensureShowTimeInventoryMutableRecord(showTime)
}

func ensureShowTimeInventoryMutable(ctx context.Context, svcCtx *svc.ServiceContext, showTimeID int64) error {
	if showTimeID <= 0 {
		return xerr.ErrInvalidParam
	}
	if svcCtx == nil || svcCtx.DProgramShowTimeModel == nil {
		return xerr.ErrInternal
	}

	showTime, err := svcCtx.DProgramShowTimeModel.FindOne(ctx, showTimeID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return xerr.ErrProgramShowTimeNotFound
		}
		return err
	}

	return ensureShowTimeInventoryMutableRecord(showTime)
}

func ensureShowTimeInventoryMutableRecord(showTime *model.DProgramShowTime) error {
	if showTime == nil {
		return nil
	}
	if showTime.InventoryPreheatStatus == 2 {
		return xerr.ErrProgramInventoryPreheated
	}

	return nil
}

func mapInventoryMutationError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrInvalidParam):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, xerr.ErrProgramShowTimeNotFound), errors.Is(err, xerr.ErrProgramTicketCategoryNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrProgramInventoryPreheated):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, xerr.ErrInternal):
		return status.Error(codes.Internal, err.Error())
	default:
		return err
	}
}
