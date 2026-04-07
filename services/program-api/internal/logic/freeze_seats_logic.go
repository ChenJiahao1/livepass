// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/program-api/internal/svc"
	"damai-go/services/program-api/internal/types"
	"damai-go/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FreezeSeatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFreezeSeatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreezeSeatsLogic {
	return &FreezeSeatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FreezeSeatsLogic) FreezeSeats(req *types.FreezeSeatsReq) (resp *types.FreezeSeatsResp, err error) {
	if req.ShowTimeID <= 0 || req.TicketCategoryID <= 0 || req.Count <= 0 || req.RequestNo == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	rpcResp, err := l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(l.ctx, &programrpc.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       req.ShowTimeID,
		TicketCategoryId: req.TicketCategoryID,
		Count:            req.Count,
		RequestNo:        req.RequestNo,
		FreezeSeconds:    req.FreezeSeconds,
	})
	if err != nil {
		return nil, err
	}

	return mapFreezeSeatsResp(rpcResp), nil
}
