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

type GetProgramDetailViewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProgramDetailViewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramDetailViewLogic {
	return &GetProgramDetailViewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProgramDetailViewLogic) GetProgramDetailView(req *types.GetProgramDetailViewReq) (resp *types.ProgramDetailViewInfo, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.GetProgramDetailView(l.ctx, &programrpc.GetProgramDetailViewReq{Id: req.ID})
	if err != nil {
		return nil, err
	}

	return mapProgramDetailViewInfo(rpcResp), nil
}
