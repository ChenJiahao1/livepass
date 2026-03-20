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

type ListTicketCategoriesByProgramLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTicketCategoriesByProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketCategoriesByProgramLogic {
	return &ListTicketCategoriesByProgramLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTicketCategoriesByProgramLogic) ListTicketCategoriesByProgram(req *types.ListTicketCategoriesByProgramReq) (resp *types.TicketCategoryDetailListResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.ListTicketCategoriesByProgram(l.ctx, &programrpc.ListTicketCategoriesByProgramReq{
		ProgramId: req.ProgramID,
	})
	if err != nil {
		return nil, err
	}

	return mapTicketCategoryDetailListResp(rpcResp), nil
}
