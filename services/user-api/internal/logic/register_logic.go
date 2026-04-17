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

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.UserRegisterReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.UserRpc.Register(l.ctx, &userrpc.RegisterReq{
		Mobile:          req.Mobile,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
		Mail:            req.Mail,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
