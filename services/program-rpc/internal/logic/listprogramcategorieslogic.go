package logic

import (
	"context"
	"errors"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProgramCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProgramCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramCategoriesLogic {
	return &ListProgramCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProgramCategoriesLogic) ListProgramCategories(in *pb.Empty) (*pb.ProgramCategoryListResp, error) {
	categories, err := l.svcCtx.DProgramCategoryModel.FindAll(l.ctx)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.ProgramCategoryListResp{List: []*pb.ProgramCategoryInfo{}}, nil
		}
		return nil, err
	}

	list := make([]*pb.ProgramCategoryInfo, 0, len(categories))
	for _, category := range categories {
		list = append(list, &pb.ProgramCategoryInfo{
			Id:       category.Id,
			ParentId: category.ParentId,
			Name:     category.Name,
			Type:     category.Type,
		})
	}

	return &pb.ProgramCategoryListResp{List: list}, nil
}
