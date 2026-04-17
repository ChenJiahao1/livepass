package logic

import (
	"context"
	"errors"

	"livepass/pkg/xerr"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListProgramShowTimesForRushLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProgramShowTimesForRushLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramShowTimesForRushLogic {
	return &ListProgramShowTimesForRushLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProgramShowTimesForRushLogic) ListProgramShowTimesForRush(in *pb.ListProgramShowTimesForRushReq) (*pb.ListProgramShowTimesForRushResp, error) {
	if in == nil || in.GetProgramId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if l.svcCtx == nil || l.svcCtx.DProgramShowTimeModel == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}
	if l.svcCtx.DProgramModel == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	showTimes, err := l.svcCtx.DProgramShowTimeModel.FindByProgramIds(l.ctx, []int64{in.GetProgramId()})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.ListProgramShowTimesForRushResp{}, nil
		}
		return nil, err
	}
	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.ListProgramShowTimesForRushResp{}, nil
		}
		return nil, err
	}

	resp := &pb.ListProgramShowTimesForRushResp{
		List: make([]*pb.ProgramShowTimeForRushInfo, 0, len(showTimes)),
	}
	for _, showTime := range showTimes {
		if showTime == nil {
			continue
		}
		resp.List = append(resp.List, &pb.ProgramShowTimeForRushInfo{
			ShowTimeId:             showTime.Id,
			RushSaleOpenTime:       formatNullTime(program.RushSaleOpenTime),
			InventoryPreheatStatus: program.InventoryPreheatStatus,
		})
	}

	return resp, nil
}
