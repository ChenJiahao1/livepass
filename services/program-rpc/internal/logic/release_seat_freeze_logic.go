package logic

import (
	"context"
	"database/sql"
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
	var resp *pb.ReleaseSeatFreezeResp

	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		sessionConn := sqlx.NewSqlConnFromSession(session)
		seatModel := model.NewDSeatModel(sessionConn)
		seatFreezeModel := model.NewDSeatFreezeModel(sessionConn)

		freeze, err := seatFreezeModel.FindOneByFreezeToken(ctx, in.GetFreezeToken())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrSeatFreezeNotFound
			}
			return err
		}

		if freeze.FreezeStatus == seatFreezeStatusReleased || freeze.FreezeStatus == seatFreezeStatusExpired {
			resp = &pb.ReleaseSeatFreezeResp{Success: true}
			return nil
		}

		if !freeze.ExpireTime.After(now) {
			if err := seatModel.ReleaseByFreezeToken(ctx, session, freeze.FreezeToken); err != nil {
				return err
			}
			if err := seatFreezeModel.MarkExpiredByFreezeTokens(ctx, session, []string{freeze.FreezeToken}, now); err != nil {
				return err
			}
			resp = &pb.ReleaseSeatFreezeResp{Success: true}
			return nil
		}

		if err := seatModel.ReleaseByFreezeToken(ctx, session, freeze.FreezeToken); err != nil {
			return err
		}

		freeze.FreezeStatus = seatFreezeStatusReleased
		freeze.ReleaseReason = nullableString(in.GetReleaseReason())
		freeze.ReleaseTime = sql.NullTime{Time: now, Valid: true}
		freeze.EditTime = now
		if err := seatFreezeModel.Update(ctx, freeze); err != nil {
			return err
		}

		resp = &pb.ReleaseSeatFreezeResp{Success: true}
		return nil
	})
	if err != nil {
		return nil, mapReleaseSeatFreezeError(err)
	}

	return resp, nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: s, Valid: true}
}

func mapReleaseSeatFreezeError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrSeatFreezeNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrProgramSeatLedgerNotReady):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return err
	}
}
