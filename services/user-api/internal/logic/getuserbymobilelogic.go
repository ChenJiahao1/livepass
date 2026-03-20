// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByMobileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserByMobileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByMobileLogic {
	return &GetUserByMobileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserByMobileLogic) GetUserByMobile(req *types.GetUserByMobileReq) (resp *types.UserVo, err error) {
	rpcResp, err := l.svcCtx.UserRpc.GetUserByMobile(l.ctx, &userrpc.GetUserByMobileReq{
		Mobile: req.Mobile,
	})
	if err != nil {
		return nil, err
	}

	return mapUserVo(rpcResp), nil
}
