package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &pb.CreatePurchaseTokenResp{}, nil
}
