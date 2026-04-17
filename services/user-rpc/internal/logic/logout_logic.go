package logic

import (
	"context"

	"livepass/pkg/xjwt"
	"livepass/services/user-rpc/internal/svc"
	"livepass/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *pb.LogoutReq) (*pb.BoolResp, error) {
	secret, err := accessSecret(l.svcCtx)
	if err != nil {
		return nil, err
	}
	claims, err := xjwt.ParseToken(in.Token, secret)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.Redis != nil {
		if _, err := l.svcCtx.Redis.DelCtx(l.ctx, loginStateKey(claims.UserID)); err != nil {
			return nil, err
		}
	}

	return &pb.BoolResp{Success: true}, nil
}
