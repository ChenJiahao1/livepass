package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetUserByMobileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserByMobileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByMobileLogic {
	return &GetUserByMobileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserByMobileLogic) GetUserByMobile(in *pb.GetUserByMobileReq) (*pb.UserInfo, error) {
	mobileMapping, err := l.svcCtx.DUserMobileModel.FindOneByMobile(l.ctx, in.Mobile)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}
	user, err := l.svcCtx.DUserModel.FindOne(l.ctx, mobileMapping.UserId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}

	return buildUserInfo(user), nil
}
