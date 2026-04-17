// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/services/program-api/internal/svc"
	"livepass/services/program-api/internal/types"
	"livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProgramCategoriesByTypeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListProgramCategoriesByTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProgramCategoriesByTypeLogic {
	return &ListProgramCategoriesByTypeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProgramCategoriesByTypeLogic) ListProgramCategoriesByType(req *types.ProgramCategoryTypeReq) (resp *types.ProgramCategoryListResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.ListProgramCategoriesByType(l.ctx, &programrpc.ProgramCategoryTypeReq{Type: req.Type})
	if err != nil {
		return nil, err
	}

	return mapProgramCategoryListResp(rpcResp), nil
}
