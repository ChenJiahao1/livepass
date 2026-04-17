// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/services/user-api/internal/svc"
	"livepass/services/user-api/internal/types"
	"livepass/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExistLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExistLogic {
	return &ExistLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExistLogic) Exist(req *types.UserExistReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.UserRpc.Exist(l.ctx, &userrpc.ExistReq{
		Mobile: req.Mobile,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
