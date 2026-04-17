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

type CreateProgramShowTimeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateProgramShowTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProgramShowTimeLogic {
	return &CreateProgramShowTimeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateProgramShowTimeLogic) CreateProgramShowTime(req *types.ProgramShowTimeAddReq) (resp *types.IdResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.CreateProgramShowTime(l.ctx, &programrpc.ProgramShowTimeAddReq{
		ProgramId:    req.ProgramID,
		ShowTime:     req.ShowTime,
		ShowDayTime:  req.ShowDayTime,
		ShowWeekTime: req.ShowWeekTime,
	})
	if err != nil {
		return nil, err
	}

	return mapIdResp(rpcResp), nil
}
