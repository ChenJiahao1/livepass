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

type AuthenticationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthenticationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthenticationLogic {
	return &AuthenticationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuthenticationLogic) Authentication(req *types.AuthenticationReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.UserRpc.Authentication(l.ctx, &userrpc.AuthenticationReq{
		Id:       req.ID,
		RelName:  req.RelName,
		IdNumber: req.IdNumber,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
