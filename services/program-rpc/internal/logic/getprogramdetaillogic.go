package logic

import (
	"context"
	"errors"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProgramDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProgramDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramDetailLogic {
	return &GetProgramDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProgramDetailLogic) GetProgramDetail(in *pb.GetProgramDetailReq) (*pb.ProgramDetailInfo, error) {
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

	categories, err := l.svcCtx.DProgramCategoryModel.FindAll(l.ctx)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	categoryMap := buildCategoryMap(categories)

	group, err := l.svcCtx.DProgramGroupModel.FindOne(l.ctx, program.ProgramGroupId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}
	groupInfo, err := parseProgramGroupJSON(group)
	if err != nil {
		return nil, err
	}

	ticketCategories, err := l.svcCtx.DTicketCategoryModel.FindByProgramId(l.ctx, program.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}

	return toProgramDetailInfo(program, firstShowTime, groupInfo, categoryMap, ticketCategories), nil
}
