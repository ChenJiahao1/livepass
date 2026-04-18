package logic

import (
	"context"
	"errors"

	"livepass/pkg/xerr"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

	seatStore := ensureSeatStockStore(l.svcCtx)
	if seatStore == nil {
		return nil, mapReleaseSeatFreezeError(xerr.ErrProgramSeatLedgerNotReady)
	}

	unlock := ensureSeatFreezeLocker(l.svcCtx).Lock(seatFreezeTokenLockKey(in.GetFreezeToken()))
	defer unlock()

	freezeState, err := loadSeatFreezeDBState(l.ctx, l.svcCtx.DSeatModel, in.GetFreezeToken())
	if err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}
	if len(freezeState.dbSeats) == 0 {
		if err := seatStore.ReleaseFrozenSeats(l.ctx, freezeState.token.ShowTimeID, freezeState.token.TicketCategoryID, in.GetFreezeToken()); err != nil && !errors.Is(err, xerr.ErrProgramSeatLedgerNotReady) {
			return nil, mapReleaseSeatFreezeError(err)
		}
		return &pb.ReleaseSeatFreezeResp{Success: true}, nil
	}
	if freezeState.allSold {
		return nil, mapReleaseSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}
	if !freezeState.allFrozen {
		return nil, mapReleaseSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}

	frozenSeats, err := seatStore.FrozenSeats(l.ctx, freezeState.token.ShowTimeID, freezeState.token.TicketCategoryID, in.GetFreezeToken())
	if err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}
	if len(frozenSeats) == 0 || len(frozenSeats) != len(freezeState.dbSeats) {
		return nil, mapReleaseSeatFreezeError(xerr.ErrSeatFreezeStatusInvalid)
	}
	seatIDs := make([]int64, 0, len(frozenSeats))
	for _, seat := range frozenSeats {
		seatIDs = append(seatIDs, seat.SeatID)
	}

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		seatModel := model.NewDSeatModel(sqlx.NewSqlConnFromSession(session))
		seats, err := seatModel.FindByShowTimeAndIDsForUpdate(ctx, session, freezeState.token.ShowTimeID, seatIDs)
		if err != nil {
			return err
		}
		if len(seats) != len(seatIDs) {
			return xerr.ErrSeatFreezeStatusInvalid
		}
		for _, seat := range seats {
			if seat.SeatStatus != 2 || !seat.FreezeToken.Valid || seat.FreezeToken.String != in.GetFreezeToken() {
				return xerr.ErrSeatFreezeStatusInvalid
			}
		}

		return seatModel.ReleaseFrozenByShowTimeAndIDs(ctx, session, freezeState.token.ShowTimeID, seatIDs, in.GetFreezeToken())
	})
	if err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}

	if err := seatStore.ReleaseFrozenSeats(l.ctx, freezeState.token.ShowTimeID, freezeState.token.TicketCategoryID, in.GetFreezeToken()); err != nil {
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
