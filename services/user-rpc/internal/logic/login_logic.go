package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xjwt"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *pb.LoginReq) (*pb.LoginResp, error) {
	secret, err := channelSecret(l.svcCtx, in.Code)
	if err != nil {
		return nil, err
	}

	if in.Password == "" || (in.Mobile == "" && in.Email == "") {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	var (
		user    *model.DUser
		failKey string
	)

	switch {
	case in.Mobile != "":
		failKey = mobileFailKey(in.Mobile)
		mobileMapping, err := l.svcCtx.DUserMobileModel.FindOneByMobile(l.ctx, in.Mobile)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
			}
			return nil, err
		}
		user, err = l.svcCtx.DUserModel.FindOne(l.ctx, mobileMapping.UserId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
			}
			return nil, err
		}
	case in.Email != "":
		failKey = emailFailKey(in.Email)
		emailMapping, err := l.svcCtx.DUserEmailModel.FindOneByEmail(l.ctx, in.Email)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
			}
			return nil, err
		}
		user, err = l.svcCtx.DUserModel.FindOne(l.ctx, emailMapping.UserId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
			}
			return nil, err
		}
	}

	if l.svcCtx.Redis != nil {
		if value, err := l.svcCtx.Redis.GetCtx(l.ctx, failKey); err == nil {
			if parseFailCount(value) >= l.svcCtx.Config.UserAuth.LoginFailLimit {
				return nil, status.Error(codes.ResourceExhausted, xerr.ErrLoginFailedTooMany.Error())
			}
		}
	}

	if !user.Password.Valid || user.Password.String != md5Hex(in.Password) {
		if l.svcCtx.Redis != nil {
			count, err := l.svcCtx.Redis.IncrCtx(l.ctx, failKey)
			if err == nil && count == 1 {
				_ = l.svcCtx.Redis.ExpireCtx(l.ctx, failKey, durationSeconds(l.svcCtx.Config.UserAuth.TokenExpire))
			}
		}
		return nil, status.Error(codes.Unauthenticated, xerr.ErrInvalidPassword.Error())
	}

	token, err := xjwt.CreateToken(user.Id, secret, l.svcCtx.Config.UserAuth.TokenExpire)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.Redis != nil {
		_, _ = l.svcCtx.Redis.DelCtx(l.ctx, failKey)
		if err := l.svcCtx.Redis.SetexCtx(l.ctx, loginStateKey(user.Id), token, durationSeconds(l.svcCtx.Config.UserAuth.TokenExpire)); err != nil {
			return nil, err
		}
	}

	return &pb.LoginResp{
		UserId: user.Id,
		Token:  token,
	}, nil
}
