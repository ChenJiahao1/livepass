package logic

import (
	"context"

	"livepass/pkg/xid"
	"livepass/services/pay-rpc/internal/model"
	"livepass/services/pay-rpc/internal/svc"
	"livepass/services/pay-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type MockPayLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMockPayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MockPayLogic {
	return &MockPayLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MockPayLogic) MockPay(in *pb.MockPayReq) (*pb.MockPayResp, error) {
	if err := validateMockPayReq(in); err != nil {
		return nil, err
	}

	var resp *pb.MockPayResp
	now := nowFunc()
	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		payBill, err := l.svcCtx.DPayBillModel.FindOneByOrderNumberForUpdate(ctx, session, in.GetOrderNumber())
		if err == nil {
			resp = mapMockPayResp(payBill)
			return nil
		}
		if err != nil && err != model.ErrNotFound {
			return err
		}

		payBill = &model.DPayBill{
			Id:          xid.New(),
			PayBillNo:   xid.New(),
			OrderNumber: in.GetOrderNumber(),
			UserId:      in.GetUserId(),
			Subject:     in.GetSubject(),
			Channel:     normalizePayChannel(in.GetChannel()),
			OrderAmount: float64(in.GetAmount()),
			PayStatus:   payStatusPaid,
			PayTime:     newNullTime(now),
			CreateTime:  now,
			EditTime:    now,
			Status:      1,
		}
		if _, err := l.svcCtx.DPayBillModel.InsertWithSession(ctx, session, payBill); err != nil {
			return err
		}

		resp = mapMockPayResp(payBill)
		return nil
	})
	if err != nil {
		return nil, mapPayError(err)
	}

	return resp, nil
}
