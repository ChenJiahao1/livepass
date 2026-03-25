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

type InvalidProgramLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewInvalidProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InvalidProgramLogic {
	return &InvalidProgramLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InvalidProgramLogic) InvalidProgram(req *types.ProgramInvalidReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.InvalidProgram(l.ctx, &programrpc.ProgramInvalidReq{Id: req.ID})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
