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

type CommonPayLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCommonPayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CommonPayLogic {
	return &CommonPayLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CommonPayLogic) CommonPay(req *types.CommonPayReq) (resp *types.CommonPayResp, err error) {
	rpcResp, err := l.svcCtx.PayRpc.MockPay(l.ctx, &payrpc.MockPayReq{
		OrderNumber: req.OrderNumber,
		UserId:      req.UserID,
		Subject:     req.Subject,
		Channel:     req.Channel,
		Amount:      req.Amount,
	})
	if err != nil {
		return nil, err
	}

	return &types.CommonPayResp{
		PayBillNo: rpcResp.GetPayBillNo(),
		PayStatus: rpcResp.GetPayStatus(),
		PayTime:   rpcResp.GetPayTime(),
	}, nil
}
