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
	rpcResp, err := l.svcCtx.ProgramRpc.GetProgramPreorder(l.ctx, &programrpc.GetProgramDetailReq{Id: req.ID})
	if err != nil {
		return nil, err
	}

	return mapProgramPreorderInfo(rpcResp), nil
}
