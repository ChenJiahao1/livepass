// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/pay-api/internal/svc"
	"damai-go/services/pay-api/internal/types"
	payrpc "damai-go/services/pay-rpc/payrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPayBillLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPayBillLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPayBillLogic {
	return &GetPayBillLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPayBillLogic) GetPayBill(req *types.PayBillReq) (resp *types.PayBillDetailResp, err error) {
	rpcResp, err := l.svcCtx.PayRpc.GetPayBill(l.ctx, &payrpc.GetPayBillReq{
		OrderNumber: req.OrderNumber,
	})
	if err != nil {
		return nil, err
	}

	return &types.PayBillDetailResp{
		PayBillNo:   rpcResp.GetPayBillNo(),
		OrderNumber: rpcResp.GetOrderNumber(),
		UserID:      rpcResp.GetUserId(),
		Subject:     rpcResp.GetSubject(),
		Channel:     rpcResp.GetChannel(),
		Amount:      rpcResp.GetAmount(),
		PayStatus:   rpcResp.GetPayStatus(),
		PayTime:     rpcResp.GetPayTime(),
	}, nil
}
