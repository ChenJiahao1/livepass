package logic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &pb.ProgramCategoryListResp{}, nil
}
