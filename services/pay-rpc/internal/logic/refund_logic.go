package logic

import (
	"context"
	"errors"

	"livepass/pkg/xerr"
	"livepass/pkg/xid"
	"livepass/services/pay-rpc/internal/model"
	"livepass/services/pay-rpc/internal/svc"
	"livepass/services/pay-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type RefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundLogic {
	return &RefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefundLogic) Refund(in *pb.RefundReq) (*pb.RefundResp, error) {
	if err := validateRefundReq(in); err != nil {
		return nil, err
	}

	now := nowFunc()
	var resp *pb.RefundResp

	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		refundBill, err := l.svcCtx.DRefundBillModel.FindOneByOrderNumberForUpdate(ctx, session, in.GetOrderNumber())
		if err == nil {
			resp = mapRefundResp(refundBill)
			return nil
		}
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return err
		}

		payBill, err := l.svcCtx.DPayBillModel.FindOneByOrderNumberForUpdate(ctx, session, in.GetOrderNumber())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrPayBillNotFound
			}
			return err
		}

		refundBill, err = l.svcCtx.DRefundBillModel.FindOneByOrderNumberForUpdate(ctx, session, in.GetOrderNumber())
		if err == nil {
			resp = mapRefundResp(refundBill)
			return nil
		}
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return err
		}

		if payBill.PayStatus != payStatusPaid {
			return xerr.ErrOrderStatusInvalid
		}
		if int64(payBill.OrderAmount) < in.GetAmount() {
			return xerr.ErrInvalidParam
		}

		if err := l.svcCtx.DPayBillModel.UpdateRefundStatus(ctx, session, in.GetOrderNumber(), now); err != nil {
			return err
		}

		refundBill = &model.DRefundBill{
			Id:           xid.New(),
			RefundBillNo: xid.New(),
			OrderNumber:  in.GetOrderNumber(),
			PayBillId:    payBill.Id,
			UserId:       in.GetUserId(),
			RefundAmount: float64(in.GetAmount()),
			RefundStatus: refundStatusRefunded,
			RefundReason: newNullString(in.GetReason()),
			RefundTime:   newNullTime(now),
			CreateTime:   now,
			EditTime:     now,
			Status:       1,
		}
		if _, err := l.svcCtx.DRefundBillModel.InsertWithSession(ctx, session, refundBill); err != nil {
			return err
		}

		resp = mapRefundResp(refundBill)
		return nil
	})
	if err != nil {
		return nil, mapPayError(err)
	}

	return resp, nil
}
