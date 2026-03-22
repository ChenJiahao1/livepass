package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
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
	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		seatFreezeModel := model.NewDSeatFreezeModel(sqlx.NewSqlConnFromSession(session))
		seatModel := model.NewDSeatModel(sqlx.NewSqlConnFromSession(session))

		freeze, err := seatFreezeModel.FindOneByFreezeTokenForUpdate(ctx, session, in.GetFreezeToken())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrSeatFreezeNotFound
			}
			return err
		}
		if freeze.FreezeStatus != seatFreezeStatusFrozen || !freeze.ExpireTime.After(now) {
			return xerr.ErrSeatFreezeStatusInvalid
		}

		seatStore := ensureSeatStockStore(l.svcCtx)
		if seatStore == nil {
			return xerr.ErrProgramSeatLedgerNotReady
		}
		if err := seatStore.ConfirmFrozenSeats(ctx, freeze.ProgramId, freeze.TicketCategoryId, freeze.FreezeToken); err != nil {
			return err
		}

		if err := seatModel.ConfirmByFreezeToken(ctx, session, freeze.FreezeToken); err != nil {
			return err
		}
		if err := seatFreezeModel.MarkConfirmedByFreezeToken(ctx, session, freeze.FreezeToken, now); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
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
