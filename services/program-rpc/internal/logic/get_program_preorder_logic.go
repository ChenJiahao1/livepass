package logic

import (
	"context"
	"errors"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProgramPreorderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProgramPreorderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramPreorderLogic {
	return &GetProgramPreorderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProgramPreorderLogic) GetProgramPreorder(in *pb.GetProgramPreorderReq) (*pb.ProgramPreorderInfo, error) {
	showTime, err := l.svcCtx.DProgramShowTimeModel.FindOne(l.ctx, in.GetShowTimeId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
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

	ticketCategories, err := l.svcCtx.DTicketCategoryModel.FindByShowTimeId(l.ctx, showTime.Id)
	switch {
	case err == nil:
	case errors.Is(err, model.ErrNotFound):
		ticketCategories = []*model.DTicketCategory{}
	default:
		return nil, err
	}

	remainAggregates, err := l.svcCtx.DSeatModel.FindAvailableCountByShowTimeId(l.ctx, showTime.Id)
	if err != nil {
		return nil, err
	}

	return toProgramPreorderInfo(program, showTime, ticketCategories, mapSeatRemainAggregates(remainAggregates)), nil
}
