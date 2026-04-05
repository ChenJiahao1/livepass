package logic

import (
	"context"
	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateOrderLogic) CreateOrder(in *pb.CreateOrderReq) (*pb.CreateOrderResp, error) {
	if err := validateCreateOrderReq(in); err != nil {
		return nil, err
	}
	if l.svcCtx == nil || l.svcCtx.PurchaseTokenCodec == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	claims, err := l.svcCtx.PurchaseTokenCodec.Verify(in.GetPurchaseToken())
	if err != nil {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}
	if claims.UserID != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}

	return &pb.CreateOrderResp{OrderNumber: claims.OrderNumber}, nil
}
