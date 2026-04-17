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

type GetProgramPreorderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProgramPreorderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramPreorderLogic {
	return &GetProgramPreorderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProgramPreorderLogic) GetProgramPreorder(req *types.GetProgramPreorderReq) (resp *types.ProgramPreorderInfo, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.GetProgramPreorder(l.ctx, &programrpc.GetProgramPreorderReq{ShowTimeId: req.ShowTimeID})
	if err != nil {
		return nil, err
	}

	return mapProgramPreorderInfo(rpcResp), nil
}
