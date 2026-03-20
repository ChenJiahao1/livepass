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

type UpdatePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePasswordLogic {
	return &UpdatePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePasswordLogic) UpdatePassword(in *pb.UpdatePasswordReq) (*pb.BoolResp, error) {
	user, err := l.svcCtx.DUserModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}
	user.Password = sql.NullString{String: md5Hex(in.Password), Valid: in.Password != ""}
	user.EditTime = sql.NullTime{Time: time.Now(), Valid: true}
	if err := l.svcCtx.DUserModel.Update(l.ctx, user); err != nil {
		return nil, err
	}
	return &pb.BoolResp{Success: true}, nil
}
