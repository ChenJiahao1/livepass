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

type ReleaseSoldSeatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseSoldSeatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseSoldSeatsLogic {
	return &ReleaseSoldSeatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReleaseSoldSeatsLogic) ReleaseSoldSeats(in *pb.ReleaseSoldSeatsReq) (*pb.ReleaseSoldSeatsResp, error) {
	if in.GetShowTimeId() <= 0 || len(in.GetSeatIds()) == 0 || in.GetRequestNo() == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	showTime, err := l.svcCtx.DProgramShowTimeModel.FindOne(l.ctx, in.GetShowTimeId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "show time not found")
		}
		return nil, err
	}
	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, showTime.ProgramId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}
	if isRefundBlockedDuringRushSale(program, time.Time{}) {
		return nil, status.Error(codes.FailedPrecondition, rushSaleRefundBlockedReason)
	}

	seatIDs := uniqueSeatIDs(in.GetSeatIds())
	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		seatModel := model.NewDSeatModel(sqlx.NewSqlConnFromSession(session))
		seats, err := seatModel.FindByShowTimeAndIDsForUpdate(ctx, session, in.GetShowTimeId(), seatIDs)
		if err != nil {
			return err
		}
		if len(seats) != len(seatIDs) {
			return status.Error(codes.NotFound, "seat not found")
		}

		for _, seat := range seats {
			if seat.SeatStatus != 1 && seat.SeatStatus != 3 {
				return status.Error(codes.FailedPrecondition, "seat status invalid")
			}
		}

		return seatModel.ReleaseSoldByShowTimeAndIDs(ctx, session, in.GetShowTimeId(), seatIDs)
	})
	if err != nil {
		if status.Code(err) != codes.Unknown || errors.Is(err, xerr.ErrInvalidParam) {
			return nil, err
		}
		return nil, err
	}

	return &pb.ReleaseSoldSeatsResp{Success: true}, nil
}

func uniqueSeatIDs(seatIDs []int64) []int64 {
	if len(seatIDs) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(seatIDs))
	resp := make([]int64, 0, len(seatIDs))
	for _, seatID := range seatIDs {
		if seatID <= 0 {
			continue
		}
		if _, ok := seen[seatID]; ok {
			continue
		}
		seen[seatID] = struct{}{}
		resp = append(resp, seatID)
	}

	return resp
}
