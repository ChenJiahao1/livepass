// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"

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

func (l *GetUserByIDLogic) GetUserByID(req *types.GetUserByIDReq) (resp *types.UserInfoResp, err error) {
	// todo: add your logic here and delete this line

	return
}
