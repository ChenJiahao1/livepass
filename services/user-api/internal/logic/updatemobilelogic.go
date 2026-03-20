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

type UpdateMobileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMobileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMobileLogic {
	return &UpdateMobileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMobileLogic) UpdateMobile(req *types.UpdateMobileReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.UserRpc.UpdateMobile(l.ctx, &userrpc.UpdateMobileReq{
		Id:     req.ID,
		Mobile: req.Mobile,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
