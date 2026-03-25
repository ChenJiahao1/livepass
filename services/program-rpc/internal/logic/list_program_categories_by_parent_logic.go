package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListProgramCategoriesByParentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProgramCategoriesByParentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramCategoriesByParentLogic {
	return &ListProgramCategoriesByParentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProgramCategoriesByParentLogic) ListProgramCategoriesByParent(in *pb.ParentProgramCategoryReq) (*pb.ProgramCategoryListResp, error) {
	if in.GetParentProgramCategoryId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	categories, err := l.svcCtx.DProgramCategoryModel.FindByParentID(l.ctx, in.GetParentProgramCategoryId())
	if err != nil {
		return nil, err
	}

	return buildProgramCategoryListResp(categories), nil
}
