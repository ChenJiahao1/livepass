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

type CreateTicketCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTicketCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTicketCategoryLogic {
	return &CreateTicketCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateTicketCategoryLogic) CreateTicketCategory(req *types.TicketCategoryAddReq) (resp *types.IdResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.CreateTicketCategory(l.ctx, &programrpc.TicketCategoryAddReq{
		ProgramId:    req.ProgramID,
		Introduce:    req.Introduce,
		Price:        req.Price,
		TotalNumber:  req.TotalNumber,
		RemainNumber: req.RemainNumber,
	})
	if err != nil {
		return nil, err
	}

	return mapIdResp(rpcResp), nil
}
