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

type ListHomeProgramsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListHomeProgramsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHomeProgramsLogic {
	return &ListHomeProgramsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListHomeProgramsLogic) ListHomePrograms(req *types.ListHomeProgramsReq) (resp *types.ProgramHomeListResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.ListHomePrograms(l.ctx, &programrpc.ListHomeProgramsReq{
		AreaId:                   req.AreaId,
		ParentProgramCategoryIds: req.ParentProgramCategoryIds,
	})
	if err != nil {
		return nil, err
	}

	return mapProgramHomeListResp(rpcResp), nil
}
