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

type RefundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundLogic {
	return &RefundLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefundLogic) Refund(req *types.RefundReq) (resp *types.RefundResp, err error) {
	payBill, err := l.svcCtx.PayRpc.GetPayBill(l.ctx, &payrpc.GetPayBillReq{
		OrderNumber: req.OrderNumber,
	})
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.PayRpc.Refund(l.ctx, &payrpc.RefundReq{
		OrderNumber: req.OrderNumber,
		UserId:      payBill.GetUserId(),
		Amount:      req.Amount,
		Channel:     req.Channel,
		Reason:      req.Reason,
	})
	if err != nil {
		return nil, err
	}

	return &types.RefundResp{
		RefundBillNo: rpcResp.GetRefundBillNo(),
		OrderNumber:  rpcResp.GetOrderNumber(),
		RefundAmount: rpcResp.GetRefundAmount(),
		PayStatus:    rpcResp.GetPayStatus(),
		RefundTime:   rpcResp.GetRefundTime(),
	}, nil
}
