package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CountActiveTicketsByUserProgramLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCountActiveTicketsByUserProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CountActiveTicketsByUserProgramLogic {
	return &CountActiveTicketsByUserProgramLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CountActiveTicketsByUserProgramLogic) CountActiveTicketsByUserProgram(in *pb.CountActiveTicketsByUserProgramReq) (*pb.CountActiveTicketsByUserProgramResp, error) {
	if err := validateUserProgramReq(in.GetUserId(), in.GetProgramId()); err != nil {
		return nil, err
	}

	count, err := l.svcCtx.OrderRepository.CountActiveTicketsByUserProgram(l.ctx, in.GetUserId(), in.GetProgramId())
	if err != nil {
		return nil, err
	}

	return &pb.CountActiveTicketsByUserProgramResp{ActiveTicketCount: count}, nil
}
