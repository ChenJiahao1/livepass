package logic

import (
	"context"
	"errors"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListHomeProgramsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHomeProgramsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHomeProgramsLogic {
	return &ListHomeProgramsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListHomeProgramsLogic) ListHomePrograms(in *pb.ListHomeProgramsReq) (*pb.ProgramHomeListResp, error) {
	categories, err := l.svcCtx.DProgramCategoryModel.FindAll(l.ctx)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	categoryMap := buildCategoryMap(categories)

	programs, err := l.svcCtx.DProgramModel.FindHomeList(l.ctx, &model.ProgramHomeListQuery{
		AreaId:                   in.GetAreaId(),
		ParentProgramCategoryIds: in.GetParentProgramCategoryIds(),
		Limit:                    homeListLimit(in.GetParentProgramCategoryIds()),
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.ProgramHomeListResp{Sections: []*pb.ProgramHomeSection{}}, nil
		}
		return nil, err
	}
	if len(programs) == 0 {
		return &pb.ProgramHomeListResp{Sections: []*pb.ProgramHomeSection{}}, nil
	}

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

	grouped := make(map[int64][]*pb.ProgramListInfo)
	for _, program := range programs {
		info := toProgramListInfo(program, showTimeMap[program.Id], categoryMap, priceRangeMap[program.Id])
		grouped[program.ParentProgramCategoryId] = append(grouped[program.ParentProgramCategoryId], info)
	}

	orderedIDs := orderedHomeParentCategoryIDs(in.GetParentProgramCategoryIds(), grouped)
	sections := make([]*pb.ProgramHomeSection, 0, len(orderedIDs))
	for _, parentCategoryID := range orderedIDs {
		programList := grouped[parentCategoryID]
		if len(programList) == 0 {
			continue
		}
		if len(programList) > 7 {
			programList = programList[:7]
		}
		sections = append(sections, &pb.ProgramHomeSection{
			CategoryName:      categoryName(categoryMap[parentCategoryID]),
			CategoryId:        parentCategoryID,
			ProgramListVoList: programList,
		})
	}

	return &pb.ProgramHomeListResp{Sections: sections}, nil
}
