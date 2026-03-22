// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/program-api/internal/svc"
	"damai-go/services/program-api/internal/types"
	"damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProgramCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListProgramCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramCategoriesLogic {
	return &ListProgramCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProgramCategoriesLogic) ListProgramCategories(req *types.EmptyReq) (resp *types.ProgramCategoryListResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.ListProgramCategories(l.ctx, &programrpc.Empty{})
	if err != nil {
		return nil, err
	}

	return mapProgramCategoryListResp(rpcResp), nil
}
