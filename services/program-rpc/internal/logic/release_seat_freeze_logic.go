package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/seatcache"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReleaseSeatFreezeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseSeatFreezeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseSeatFreezeLogic {
	return &ReleaseSeatFreezeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReleaseSeatFreezeLogic) ReleaseSeatFreeze(in *pb.ReleaseSeatFreezeReq) (*pb.ReleaseSeatFreezeResp, error) {
	if in.GetFreezeToken() == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	now := time.Now()
	seatStore := ensureSeatStockStore(l.svcCtx)
	if seatStore == nil {
		return nil, mapReleaseSeatFreezeError(xerr.ErrProgramSeatLedgerNotReady)
	}

	unlock := ensureSeatFreezeLocker(l.svcCtx).Lock(seatFreezeTokenLockKey(in.GetFreezeToken()))
	defer unlock()

	freeze, err := seatStore.GetFreezeMetadataByToken(l.ctx, in.GetFreezeToken())
	if err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}
	if freeze == nil {
		return nil, mapReleaseSeatFreezeError(xerr.ErrSeatFreezeNotFound)
	}
	if freeze.FreezeStatus == seatcache.SeatFreezeStatusConfirmed {
		return nil, mapReleaseSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}
	if hasSeatFreezeOwner(in.GetOwnerOrderNumber(), in.GetOwnerEpoch()) && !freeze.MatchesOwner(in.GetOwnerOrderNumber(), in.GetOwnerEpoch()) {
		return nil, mapReleaseSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}

	if freeze.FreezeStatus == seatcache.SeatFreezeStatusReleased ||
		freeze.FreezeStatus == seatcache.SeatFreezeStatusExpired {
		return &pb.ReleaseSeatFreezeResp{Success: true}, nil
	}

	if err := seatStore.ReleaseFrozenSeats(l.ctx, freeze.ProgramID, freeze.TicketCategoryID, freeze.FreezeToken, in.GetOwnerOrderNumber(), in.GetOwnerEpoch()); err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}

	if !freeze.ExpireTime().After(now) {
		if _, err := seatStore.MarkFreezeExpired(l.ctx, freeze.FreezeToken, now); err != nil {
			return nil, mapReleaseSeatFreezeError(err)
		}
		return &pb.ReleaseSeatFreezeResp{Success: true}, nil
	}

	if _, err := seatStore.MarkFreezeReleased(l.ctx, freeze.FreezeToken, in.GetReleaseReason(), now); err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}

	return &pb.ReleaseSeatFreezeResp{Success: true}, nil
}

func mapReleaseSeatFreezeError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrSeatFreezeNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrSeatFreezeStatusInvalid), errors.Is(err, xerr.ErrProgramSeatLedgerNotReady):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return err
	}
}
