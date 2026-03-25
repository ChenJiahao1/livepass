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

type CreateSeatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateSeatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSeatLogic {
	return &CreateSeatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateSeatLogic) CreateSeat(req *types.SeatAddReq) (resp *types.IdResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.CreateSeat(l.ctx, &programrpc.SeatAddReq{
		ProgramId:        req.ProgramID,
		TicketCategoryId: req.TicketCategoryID,
		RowCode:          req.RowCode,
		ColCode:          req.ColCode,
		SeatType:         req.SeatType,
		Price:            req.Price,
		SeatStatus:       req.SeatStatus,
	})
	if err != nil {
		return nil, err
	}

	return mapIdResp(rpcResp), nil
}
