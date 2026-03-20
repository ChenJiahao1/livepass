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

func (l *GetProgramPreorderLogic) GetProgramPreorder(in *pb.GetProgramDetailReq) (*pb.ProgramPreorderInfo, error) {
	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	firstShowTime, err := l.svcCtx.DProgramShowTimeModel.FindFirstByProgramId(l.ctx, program.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	ticketCategories, err := l.svcCtx.DTicketCategoryModel.FindByProgramId(l.ctx, program.Id)
	switch {
	case err == nil:
	case errors.Is(err, model.ErrNotFound):
		ticketCategories = []*model.DTicketCategory{}
	default:
		return nil, err
	}

	remainAggregates, err := l.svcCtx.DSeatModel.FindAvailableCountByProgramId(l.ctx, program.Id)
	if err != nil {
		return nil, err
	}

	return toProgramPreorderInfo(program, firstShowTime, ticketCategories, mapSeatRemainAggregates(remainAggregates)), nil
}
