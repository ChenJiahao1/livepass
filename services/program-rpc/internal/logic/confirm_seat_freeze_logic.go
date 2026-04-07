package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/seatcache"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConfirmSeatFreezeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmSeatFreezeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmSeatFreezeLogic {
	return &ConfirmSeatFreezeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfirmSeatFreezeLogic) ConfirmSeatFreeze(in *pb.ConfirmSeatFreezeReq) (*pb.ConfirmSeatFreezeResp, error) {
	if in.GetFreezeToken() == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	now := time.Now()
	seatStore := ensureSeatStockStore(l.svcCtx)
	if seatStore == nil {
		return nil, mapConfirmSeatFreezeError(xerr.ErrProgramSeatLedgerNotReady)
	}

	unlock := ensureSeatFreezeLocker(l.svcCtx).Lock(seatFreezeTokenLockKey(in.GetFreezeToken()))
	defer unlock()

	freeze, err := seatStore.GetFreezeMetadataByToken(l.ctx, in.GetFreezeToken())
	if err != nil {
		return nil, mapConfirmSeatFreezeError(err)
	}
	if freeze == nil {
		return nil, mapConfirmSeatFreezeError(xerr.ErrSeatFreezeNotFound)
	}
	if freeze.FreezeStatus == seatcache.SeatFreezeStatusConfirmed {
		return &pb.ConfirmSeatFreezeResp{Success: true}, nil
	}
	if freeze.FreezeStatus != seatcache.SeatFreezeStatusFrozen || !freeze.ExpireTime().After(now) {
		return nil, mapConfirmSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}
	if hasSeatFreezeOwner(in.GetOwnerOrderNumber(), in.GetOwnerEpoch()) && !freeze.MatchesOwner(in.GetOwnerOrderNumber(), in.GetOwnerEpoch()) {
		return nil, mapConfirmSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}

	frozenSeats, err := seatStore.FrozenSeats(l.ctx, freeze.ShowTimeID, freeze.TicketCategoryID, freeze.FreezeToken)
	if err != nil {
		return nil, mapConfirmSeatFreezeError(err)
	}
	if len(frozenSeats) == 0 || int64(len(frozenSeats)) != freeze.SeatCount {
		return nil, mapConfirmSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		seatModel := model.NewDSeatModel(sqlx.NewSqlConnFromSession(session))
		seatIDs := make([]int64, 0, len(frozenSeats))
		for _, seat := range frozenSeats {
			seatIDs = append(seatIDs, seat.SeatID)
		}

		seats, err := seatModel.FindByShowTimeAndIDsForUpdate(ctx, session, freeze.ShowTimeID, seatIDs)
		if err != nil {
			return err
		}
		if len(seats) != len(seatIDs) {
			return xerr.ErrSeatFreezeStatusInvalid
		}
		for _, seat := range seats {
			if seat.SeatStatus != 1 && seat.SeatStatus != 3 {
				return xerr.ErrSeatFreezeStatusInvalid
			}
		}
		if err := seatModel.BatchConfirmByShowTimeAndIDs(ctx, session, freeze.ShowTimeID, seatIDs, freeze.FreezeToken, freeze.ExpireTime()); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, mapConfirmSeatFreezeError(err)
	}

	if err := seatStore.ConfirmFrozenSeats(l.ctx, freeze.ShowTimeID, freeze.TicketCategoryID, freeze.FreezeToken, in.GetOwnerOrderNumber(), in.GetOwnerEpoch()); err != nil {
		return nil, mapConfirmSeatFreezeError(err)
	}
	if _, err := seatStore.MarkFreezeConfirmed(l.ctx, freeze.FreezeToken, now); err != nil {
		return nil, mapConfirmSeatFreezeError(err)
	}

	return &pb.ConfirmSeatFreezeResp{Success: true}, nil
}

func mapConfirmSeatFreezeError(err error) error {
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
