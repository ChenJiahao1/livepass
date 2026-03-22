package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	payrpc "damai-go/services/pay-rpc/payrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PayCheckLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPayCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayCheckLogic {
	return &PayCheckLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PayCheckLogic) PayCheck(in *pb.PayCheckReq) (*pb.PayCheckResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	order, err := l.svcCtx.DOrderModel.FindOneByOrderNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapOrderError(xerr.ErrOrderNotFound)
		}
		return nil, mapOrderError(err)
	}
	if order.UserId != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}
	if order.OrderStatus != orderStatusPaid && order.OrderStatus != orderStatusRefunded && order.OrderStatus != orderStatusCancelled {
		return mapPayCheckResp(order, nil), nil
	}
	if order.OrderStatus == orderStatusCancelled {
		payBill, err := l.svcCtx.PayRpc.GetPayBill(l.ctx, &payrpc.GetPayBillReq{OrderNumber: order.OrderNumber})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return mapPayCheckResp(order, nil), nil
			}
			return nil, mapOrderError(err)
		}
		if payBill.GetPayStatus() == payStatusPaid {
			refundResp, err := l.svcCtx.PayRpc.Refund(l.ctx, &payrpc.RefundReq{
				OrderNumber: order.OrderNumber,
				UserId:      in.GetUserId(),
				Amount:      int64(order.OrderPrice),
				Channel:     "mock",
				Reason:      compensationRefundReason(),
			})
			if err != nil {
				return nil, mapOrderError(err)
			}
			if err := convergeOrderRefunded(l.ctx, l.svcCtx, order.OrderNumber); err != nil {
				return nil, mapOrderError(err)
			}

			order.OrderStatus = orderStatusRefunded
			return mapPayCheckResp(order, applyRefundPayStatus(payBill, refundResp)), nil
		}
		if payBill.GetPayStatus() == payStatusRefunded {
			if err := convergeOrderRefunded(l.ctx, l.svcCtx, order.OrderNumber); err != nil {
				return nil, mapOrderError(err)
			}

			order.OrderStatus = orderStatusRefunded
		}

		return mapPayCheckResp(order, payBill), nil
	}

	payBill, err := mustGetPayBillForOrder(l.ctx, l.svcCtx, order.OrderNumber)
	if err != nil {
		return nil, mapOrderError(err)
	}

	return mapPayCheckResp(order, payBill), nil
}
