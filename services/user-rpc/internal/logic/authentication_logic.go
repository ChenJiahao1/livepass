package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthenticationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthenticationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthenticationLogic {
	return &AuthenticationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AuthenticationLogic) Authentication(in *pb.AuthenticationReq) (*pb.BoolResp, error) {
	user, err := l.svcCtx.DUserModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}
	user.RelName = sql.NullString{String: in.RelName, Valid: in.RelName != ""}
	user.IdNumber = sql.NullString{String: in.IdNumber, Valid: in.IdNumber != ""}
	user.RelAuthenticationStatus = 1
	user.EditTime = sql.NullTime{Time: time.Now(), Valid: true}
	if err := l.svcCtx.DUserModel.Update(l.ctx, user); err != nil {
		return nil, err
	}
	return &pb.BoolResp{Success: true}, nil
}
