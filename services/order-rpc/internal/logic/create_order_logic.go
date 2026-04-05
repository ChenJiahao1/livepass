package logic

import (
	"context"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
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

	// Task 1 仅固化对外契约：CreateOrder 先返回预分配轮询订单号，后续状态机在后续任务实现。
	orderNumber := generateOrderNumberForUser(in.GetUserId(), time.Now())
	return &pb.CreateOrderResp{OrderNumber: orderNumber}, nil
}
