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

type GetUserByIDLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserByIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIDLogic {
	return &GetUserByIDLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserByIDLogic) GetUserByID(req *types.GetUserByIDReq) (resp *types.UserVo, err error) {
	rpcResp, err := l.svcCtx.UserRpc.GetUserById(l.ctx, &userrpc.GetUserByIdReq{
		Id: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return mapUserVo(rpcResp), nil
}
