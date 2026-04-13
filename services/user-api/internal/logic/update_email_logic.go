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

type UpdateEmailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEmailLogic {
	return &UpdateEmailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateEmailLogic) UpdateEmail(req *types.UpdateEmailReq) (resp *types.BoolResp, err error) {
	userID, err := requireCurrentUserID(l.ctx)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.UserRpc.UpdateEmail(l.ctx, &userrpc.UpdateEmailReq{
		Id:    userID,
		Email: req.Email,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
