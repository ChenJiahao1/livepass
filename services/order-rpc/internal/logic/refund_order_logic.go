package logic

import (
	"context"
	"errors"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefundOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundOrderLogic {
	return &RefundOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefundOrderLogic) RefundOrder(in *pb.RefundOrderReq) (*pb.RefundOrderResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	order, err := l.svcCtx.OrderRepository.FindOrderByNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapOrderError(xerr.ErrOrderNotFound)
		}
		return nil, mapOrderError(err)
	}
	if order.UserId != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}

	orderTickets, err := l.svcCtx.OrderRepository.FindOrderTicketsByNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}

	if order.OrderStatus == orderStatusRefunded {
		refundResp, err := l.svcCtx.PayRpc.Refund(l.ctx, &payrpc.RefundReq{
			OrderNumber: order.OrderNumber,
			UserId:      in.GetUserId(),
			Amount:      int64(order.OrderPrice),
			Channel:     "mock",
			Reason:      in.GetReason(),
		})
		if err != nil {
			return nil, mapOrderError(err)
		}

		return mapRefundOrderResp(order, refundResp, calculateRefundPercent(int64(order.OrderPrice), refundResp.GetRefundAmount())), nil
	}
	if order.OrderStatus != orderStatusPaid {
		return nil, mapOrderError(xerr.ErrOrderStatusInvalid)
	}

	payBill, err := mustGetPayBillForOrder(l.ctx, l.svcCtx, order.OrderNumber)
	if err != nil {
		return nil, mapOrderError(err)
	}

	refundResp, refundPercent, err := l.refundOrReuse(order, payBill, in)
	if err != nil {
		return nil, mapOrderError(err)
	}

	if len(orderTicketSeatIDs(orderTickets)) > 0 {
		if _, err := l.svcCtx.ProgramRpc.ReleaseSoldSeats(l.ctx, &programrpc.ReleaseSoldSeatsReq{
			ShowTimeId: order.ShowTimeId,
			SeatIds:    orderTicketSeatIDs(orderTickets),
			RequestNo:  buildRefundRequestNo(order.OrderNumber),
		}); err != nil {
			return nil, mapOrderError(err)
		}
	}

	if err := convergeOrderRefunded(l.ctx, l.svcCtx, order.OrderNumber); err != nil {
		return nil, mapOrderError(err)
	}

	order.OrderStatus = orderStatusRefunded
	return mapRefundOrderResp(order, refundResp, refundPercent), nil
}

func (l *RefundOrderLogic) refundOrReuse(order *model.DOrder, payBill *payrpc.GetPayBillResp, in *pb.RefundOrderReq) (*payrpc.RefundResp, int64, error) {
	if payBill == nil {
		return nil, 0, xerr.ErrPayBillNotFound
	}

	if payBill.GetPayStatus() == payStatusRefunded {
		refundResp, err := l.svcCtx.PayRpc.Refund(l.ctx, &payrpc.RefundReq{
			OrderNumber: order.OrderNumber,
			UserId:      in.GetUserId(),
			Amount:      int64(order.OrderPrice),
			Channel:     "mock",
			Reason:      in.GetReason(),
		})
		if err != nil {
			return nil, 0, err
		}

		return refundResp, calculateRefundPercent(int64(order.OrderPrice), refundResp.GetRefundAmount()), nil
	}
	if payBill.GetPayStatus() != payStatusPaid {
		return nil, 0, xerr.ErrOrderStatusInvalid
	}

	evaluateResp, err := l.svcCtx.ProgramRpc.EvaluateRefundRule(l.ctx, &programrpc.EvaluateRefundRuleReq{
		ShowTimeId:  order.ShowTimeId,
		OrderAmount: int64(order.OrderPrice),
	})
	if err != nil {
		return nil, 0, err
	}
	if !evaluateResp.GetAllowRefund() {
		return nil, 0, refundRejectReasonToError(evaluateResp.GetRejectReason())
	}

	refundResp, err := l.svcCtx.PayRpc.Refund(l.ctx, &payrpc.RefundReq{
		OrderNumber: order.OrderNumber,
		UserId:      in.GetUserId(),
		Amount:      evaluateResp.GetRefundAmount(),
		Channel:     "mock",
		Reason:      in.GetReason(),
	})
	if err != nil {
		return nil, 0, err
	}

	return refundResp, evaluateResp.GetRefundPercent(), nil
}
