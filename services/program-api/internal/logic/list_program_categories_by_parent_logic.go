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

type ListProgramCategoriesByParentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListProgramCategoriesByParentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramCategoriesByParentLogic {
	return &ListProgramCategoriesByParentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProgramCategoriesByParentLogic) ListProgramCategoriesByParent(req *types.ParentProgramCategoryReq) (resp *types.ProgramCategoryListResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.ListProgramCategoriesByParent(l.ctx, &programrpc.ParentProgramCategoryReq{
		ParentProgramCategoryId: req.ParentProgramCategoryID,
	})
	if err != nil {
		return nil, err
	}

	return mapProgramCategoryListResp(rpcResp), nil
}
