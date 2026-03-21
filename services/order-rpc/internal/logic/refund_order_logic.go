package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

	orderTickets, err := l.svcCtx.DOrderTicketUserModel.FindByOrderNumber(l.ctx, in.GetOrderNumber())
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
			ProgramId: order.ProgramId,
			SeatIds:   orderTicketSeatIDs(orderTickets),
			RequestNo: buildRefundRequestNo(order.OrderNumber),
		}); err != nil {
			return nil, mapOrderError(err)
		}
	}

	if err := l.markOrderRefunded(order.OrderNumber); err != nil {
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
		ProgramId:     order.ProgramId,
		OrderShowTime: formatOrderTime(order.ProgramShowTime),
		OrderAmount:   int64(order.OrderPrice),
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

func (l *RefundOrderLogic) markOrderRefunded(orderNumber int64) error {
	return l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		order, err := l.svcCtx.DOrderModel.FindOneByOrderNumberForUpdate(ctx, session, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		if order.OrderStatus == orderStatusRefunded {
			return nil
		}
		if order.OrderStatus != orderStatusPaid {
			return xerr.ErrOrderStatusInvalid
		}

		refundTime := time.Now()
		if err := l.svcCtx.DOrderModel.UpdateRefundStatus(ctx, session, orderNumber, refundTime); err != nil {
			return err
		}
		if err := l.svcCtx.DOrderTicketUserModel.UpdateRefundStatusByOrderNumber(ctx, session, orderNumber, refundTime); err != nil {
			return err
		}

		return nil
	})
}
