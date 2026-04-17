package logic

import (
	"context"
	"errors"

	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PageProgramsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPageProgramsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PageProgramsLogic {
	return &PageProgramsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PageProgramsLogic) PagePrograms(in *pb.PageProgramsReq) (*pb.ProgramPageResp, error) {
	if err := validatePageProgramsReq(in); err != nil {
		return nil, err
	}

	pageNum := in.GetPageNumber()
	if pageNum <= 0 {
		pageNum = 1
	}
	pageSize := in.GetPageSize()
	if pageSize <= 0 {
		pageSize = 10
	}

	query := &model.ProgramPageListQuery{
		PageNumber:              pageNum,
		PageSize:                pageSize,
		AreaId:                  in.GetAreaId(),
		ParentProgramCategoryId: in.GetParentProgramCategoryId(),
		ProgramCategoryId:       in.GetProgramCategoryId(),
		TimeType:                in.GetTimeType(),
		StartDateTime:           in.GetStartDateTime(),
		EndDateTime:             in.GetEndDateTime(),
		Type:                    in.GetType(),
	}

	total, err := l.svcCtx.DProgramModel.CountPageList(l.ctx, query)
	if err != nil {
		return nil, err
	}

	resp := &pb.ProgramPageResp{
		PageNum:   pageNum,
		PageSize:  pageSize,
		TotalSize: total,
		List:      []*pb.ProgramListInfo{},
	}
	if total == 0 {
		return resp, nil
	}

	programs, err := l.svcCtx.DProgramModel.FindPageList(l.ctx, query)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return resp, nil
		}
		return nil, err
	}

	categories, err := l.svcCtx.DProgramCategoryModel.FindAll(l.ctx)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	categoryMap := buildCategoryMap(categories)

	programIDs := make([]int64, 0, len(programs))
	for _, program := range programs {
		programIDs = append(programIDs, program.Id)
	}

	showTimes, err := l.svcCtx.DProgramShowTimeModel.FindByProgramIds(l.ctx, programIDs)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	showTimeMap := mapFirstShowTime(showTimes)

	priceAggregates, err := l.svcCtx.DTicketCategoryModel.FindPriceAggregateByProgramIds(l.ctx, programIDs)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	priceRangeMap := mapTicketPriceRange(priceAggregates)

	list := make([]*pb.ProgramListInfo, 0, len(programs))
	for _, program := range programs {
		list = append(list, toProgramListInfo(program, showTimeMap[program.Id], categoryMap, priceRangeMap[program.Id]))
	}
	resp.List = list

	return resp, nil
}
