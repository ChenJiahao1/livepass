package logic

import (
	"context"
	"database/sql"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PayOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPayOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayOrderLogic {
	return &PayOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PayOrderLogic) PayOrder(in *pb.PayOrderReq) (*pb.PayOrderResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	unlock, err := lockOrderStatusGuard(l.ctx, l.svcCtx, in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}
	if unlock != nil {
		defer unlock()
	}

	order, err := loadOrderSnapshot(l.ctx, l.svcCtx, in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}
	if order.UserId != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}

	if order.OrderStatus == orderStatusPaid {
		payBill, err := mustGetPayBillForOrder(l.ctx, l.svcCtx, order.OrderNumber)
		if err != nil {
			return nil, mapOrderError(err)
		}
		return mapPayOrderResp(order, payBill), nil
	}
	if order.OrderStatus != orderStatusUnpaid {
		return nil, mapOrderError(xerr.ErrOrderStatusInvalid)
	}
	if !order.OrderExpireTime.After(time.Now()) {
		return nil, mapOrderError(xerr.ErrOrderExpired)
	}

	subject := in.GetSubject()
	if subject == "" {
		subject = order.ProgramTitle
	}
	mockResp, err := l.svcCtx.PayRpc.MockPay(l.ctx, &payrpc.MockPayReq{
		OrderNumber: order.OrderNumber,
		UserId:      order.UserId,
		Subject:     subject,
		Channel:     in.GetChannel(),
		Amount:      int64(order.OrderPrice),
	})
	if err != nil {
		return nil, mapOrderError(err)
	}
	if _, err := l.svcCtx.ProgramRpc.ConfirmSeatFreeze(l.ctx, &programrpc.ConfirmSeatFreezeReq{
		FreezeToken: order.FreezeToken,
	}); err != nil {
		return nil, mapOrderError(err)
	}

	payTime := time.Now()
	if mockResp.GetPayTime() != "" {
		parsed, err := parseOrderTime(mockResp.GetPayTime())
		if err != nil {
			return nil, mapOrderError(err)
		}
		payTime = parsed
	}
	if err := finalizeOrderPay(l.ctx, l.svcCtx, order.OrderNumber, payTime); err != nil {
		return nil, mapOrderError(err)
	}

	order.OrderStatus = orderStatusPaid
	order.PayOrderTime = sql.NullTime{Time: payTime, Valid: true}
	resp := &pb.PayOrderResp{
		OrderNumber: order.OrderNumber,
		OrderStatus: orderStatusPaid,
		PayBillNo:   mockResp.GetPayBillNo(),
		PayStatus:   mockResp.GetPayStatus(),
		PayTime:     mockResp.GetPayTime(),
	}

	return resp, nil
}
