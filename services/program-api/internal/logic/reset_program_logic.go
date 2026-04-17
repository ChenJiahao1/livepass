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

type ResetProgramLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetProgramLogic {
	return &ResetProgramLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetProgramLogic) ResetProgram(req *types.ProgramResetReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.ResetProgram(l.ctx, &programrpc.ProgramResetReq{ProgramId: req.ProgramID})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
