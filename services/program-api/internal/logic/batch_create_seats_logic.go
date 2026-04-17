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

type BatchCreateSeatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchCreateSeatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCreateSeatsLogic {
	return &BatchCreateSeatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchCreateSeatsLogic) BatchCreateSeats(req *types.SeatBatchAddReq) (resp *types.BoolResp, err error) {
	items := make([]*programrpc.SeatBatchRelateInfoAddReq, 0, len(req.SeatBatchRelateInfoAddDtoList))
	for _, item := range req.SeatBatchRelateInfoAddDtoList {
		items = append(items, &programrpc.SeatBatchRelateInfoAddReq{
			TicketCategoryId: item.TicketCategoryID,
			Price:            item.Price,
			Count:            item.Count,
		})
	}

	rpcResp, err := l.svcCtx.ProgramRpc.BatchCreateSeats(l.ctx, &programrpc.SeatBatchAddReq{
		ProgramId:                   req.ProgramID,
		SeatBatchRelateInfoAddDtoList: items,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
