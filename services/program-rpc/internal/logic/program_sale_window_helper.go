package logic

import (
	"context"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func validateShowTimeAgainstProgramRushSaleOpenTime(program *model.DProgram, showTime time.Time) error {
	if program == nil || !program.RushSaleOpenTime.Valid || showTime.IsZero() {
		return nil
	}
	if showTime.Before(program.RushSaleOpenTime.Time) {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func validateProgramRushSaleWindowAgainstExistingShowTimes(ctx context.Context, showTimeModel model.DProgramShowTimeModel, programID int64, rushSaleOpenTime string) error {
	if showTimeModel == nil || programID <= 0 || strings.TrimSpace(rushSaleOpenTime) == "" {
		return nil
	}

	firstShowTime, err := showTimeModel.FindFirstByProgramId(ctx, programID)
	if err != nil {
		if err == model.ErrNotFound {
			return nil
		}
		return err
	}

	openAt, err := time.ParseInLocation(programDateTimeLayout, rushSaleOpenTime, time.Local)
	if err != nil {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if firstShowTime != nil && firstShowTime.ShowTime.Before(openAt) {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}
