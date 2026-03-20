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

type UpdateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserLogic {
	return &UpdateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateUserLogic) UpdateUser(req *types.UpdateUserReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.UserRpc.UpdateUser(l.ctx, &userrpc.UpdateUserReq{
		Id:      req.ID,
		Name:    req.Name,
		Gender:  req.Gender,
		Mobile:  req.Mobile,
		Address: req.Address,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
