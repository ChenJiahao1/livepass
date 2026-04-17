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

type GetSeatRelateInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSeatRelateInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSeatRelateInfoLogic {
	return &GetSeatRelateInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSeatRelateInfoLogic) GetSeatRelateInfo(req *types.SeatListReq) (resp *types.SeatRelateInfoResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.GetSeatRelateInfo(l.ctx, &programrpc.SeatListReq{ProgramId: req.ProgramID})
	if err != nil {
		return nil, err
	}

	return mapSeatRelateInfo(rpcResp), nil
}
