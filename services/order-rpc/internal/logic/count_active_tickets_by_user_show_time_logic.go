package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CountActiveTicketsByUserShowTimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCountActiveTicketsByUserShowTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CountActiveTicketsByUserShowTimeLogic {
	return &CountActiveTicketsByUserShowTimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CountActiveTicketsByUserShowTimeLogic) CountActiveTicketsByUserShowTime(in *pb.CountActiveTicketsByUserShowTimeReq) (*pb.CountActiveTicketsByUserShowTimeResp, error) {
	if err := validateUserShowTimeReq(in.GetUserId(), in.GetShowTimeId()); err != nil {
		return nil, err
	}

	count, err := l.svcCtx.OrderRepository.CountActiveTicketsByUserProgram(l.ctx, in.GetUserId(), in.GetShowTimeId())
	if err != nil {
		return nil, err
	}

	return &pb.CountActiveTicketsByUserShowTimeResp{ActiveTicketCount: count}, nil
}
