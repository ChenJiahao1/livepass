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

type ListProgramCategoriesByTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProgramCategoriesByTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramCategoriesByTypeLogic {
	return &ListProgramCategoriesByTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProgramCategoriesByTypeLogic) ListProgramCategoriesByType(in *pb.ProgramCategoryTypeReq) (*pb.ProgramCategoryListResp, error) {
	if in.GetType() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	categories, err := l.svcCtx.DProgramCategoryModel.FindByType(l.ctx, in.GetType())
	if err != nil {
		return nil, err
	}

	return buildProgramCategoryListResp(categories), nil
}
