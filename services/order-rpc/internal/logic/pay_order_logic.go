package logic

import (
	"context"
	"database/sql"
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

	var resp *pb.PayOrderResp
	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		order, err := l.svcCtx.DOrderModel.FindOneByOrderNumberForUpdate(ctx, session, in.GetOrderNumber())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		if order.UserId != in.GetUserId() {
			return xerr.ErrOrderNotFound
		}

		if order.OrderStatus == orderStatusPaid {
			payBill, err := mustGetPayBillForOrder(ctx, l.svcCtx, order.OrderNumber)
			if err != nil {
				return err
			}
			resp = mapPayOrderResp(order, payBill)
			return nil
		}
		if order.OrderStatus != orderStatusUnpaid {
			return xerr.ErrOrderStatusInvalid
		}
		if !order.OrderExpireTime.After(time.Now()) {
			return xerr.ErrOrderExpired
		}

		subject := in.GetSubject()
		if subject == "" {
			subject = order.ProgramTitle
		}
		mockResp, err := l.svcCtx.PayRpc.MockPay(ctx, &payrpc.MockPayReq{
			OrderNumber: order.OrderNumber,
			UserId:      order.UserId,
			Subject:     subject,
			Channel:     in.GetChannel(),
			Amount:      int64(order.OrderPrice),
		})
		if err != nil {
			return err
		}
		if _, err := l.svcCtx.ProgramRpc.ConfirmSeatFreeze(ctx, &programrpc.ConfirmSeatFreezeReq{
			FreezeToken: order.FreezeToken,
		}); err != nil {
			return err
		}

		payTime := time.Now()
		if mockResp.GetPayTime() != "" {
			parsed, err := parseOrderTime(mockResp.GetPayTime())
			if err != nil {
				return err
			}
			payTime = parsed
		}
		if err := l.svcCtx.DOrderModel.UpdatePayStatus(ctx, session, order.OrderNumber, payTime); err != nil {
			return err
		}
		if err := l.svcCtx.DOrderTicketUserModel.UpdatePayStatusByOrderNumber(ctx, session, order.OrderNumber, payTime); err != nil {
			return err
		}

		order.OrderStatus = orderStatusPaid
		order.PayOrderTime = sql.NullTime{Time: payTime, Valid: true}
		resp = &pb.PayOrderResp{
			OrderNumber: order.OrderNumber,
			OrderStatus: orderStatusPaid,
			PayBillNo:   mockResp.GetPayBillNo(),
			PayStatus:   mockResp.GetPayStatus(),
			PayTime:     mockResp.GetPayTime(),
		}
		return nil
	})
	if err != nil {
		return nil, mapOrderError(err)
	}

	return resp, nil
}
