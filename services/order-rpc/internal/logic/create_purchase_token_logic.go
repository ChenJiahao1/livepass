package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePurchaseTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePurchaseTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePurchaseTokenLogic {
	return &CreatePurchaseTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreatePurchaseTokenLogic) CreatePurchaseToken(in *pb.CreatePurchaseTokenReq) (*pb.CreatePurchaseTokenResp, error) {
	if in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid create purchase token request")
	}
	orderNumber := allocateRushContractOrderNumber(in.GetUserId())
	token, err := encodeRushContractPurchaseToken(in.GetUserId(), orderNumber)
	if err != nil {
		return nil, err
	}

	return &pb.CreatePurchaseTokenResp{PurchaseToken: token}, nil
}
