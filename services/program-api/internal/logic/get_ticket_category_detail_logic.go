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

type GetTicketCategoryDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTicketCategoryDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTicketCategoryDetailLogic {
	return &GetTicketCategoryDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTicketCategoryDetailLogic) GetTicketCategoryDetail(req *types.TicketCategoryReq) (resp *types.TicketCategoryDetailInfo, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.GetTicketCategoryDetail(l.ctx, &programrpc.TicketCategoryReq{Id: req.ID})
	if err != nil {
		return nil, err
	}

	return mapTicketCategoryDetailInfo(rpcResp), nil
}
