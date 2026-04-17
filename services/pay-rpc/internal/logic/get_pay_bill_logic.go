package logic

import (
	"context"
	"errors"

	"livepass/pkg/xerr"
	"livepass/services/pay-rpc/internal/model"
	"livepass/services/pay-rpc/internal/svc"
	"livepass/services/pay-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPayBillLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPayBillLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPayBillLogic {
	return &GetPayBillLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPayBillLogic) GetPayBill(in *pb.GetPayBillReq) (*pb.GetPayBillResp, error) {
	if err := validateGetPayBillReq(in); err != nil {
		return nil, err
	}

	payBill, err := l.svcCtx.DPayBillModel.FindOneByOrderNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapPayError(xerr.ErrPayBillNotFound)
		}
		return nil, mapPayError(err)
	}

	return mapGetPayBillResp(payBill), nil
}
