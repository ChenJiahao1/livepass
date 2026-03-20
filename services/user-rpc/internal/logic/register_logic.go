package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterLogic) Register(in *pb.RegisterReq) (*pb.BoolResp, error) {
	if in.Mobile == "" || in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if in.ConfirmPassword != "" && in.Password != in.ConfirmPassword {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	_, err := l.svcCtx.DUserMobileModel.FindOneByMobile(l.ctx, in.Mobile)
	switch {
	case err == nil:
		return nil, status.Error(codes.AlreadyExists, xerr.ErrUserAlreadyExists.Error())
	case !errors.Is(err, model.ErrNotFound):
		return nil, err
	}

	now := time.Now()
	userID := xid.New()
	user := &model.DUser{
		Id:          userID,
		Mobile:      in.Mobile,
		Gender:      1,
		Password:    sql.NullString{String: md5Hex(in.Password), Valid: true},
		Email:       sql.NullString{String: in.Mail, Valid: in.Mail != ""},
		EmailStatus: in.MailStatus,
		EditTime:    sql.NullTime{Time: now, Valid: true},
		Status:      1,
	}
	if _, err := l.svcCtx.DUserModel.Insert(l.ctx, user); err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.DUserMobileModel.Insert(l.ctx, &model.DUserMobile{
		Id:       xid.New(),
		UserId:   userID,
		Mobile:   in.Mobile,
		EditTime: sql.NullTime{Time: now, Valid: true},
		Status:   1,
	}); err != nil {
		return nil, err
	}
	if in.Mail != "" {
		if _, err := l.svcCtx.DUserEmailModel.Insert(l.ctx, &model.DUserEmail{
			Id:          xid.New(),
			UserId:      userID,
			Email:       in.Mail,
			EmailStatus: in.MailStatus,
			EditTime:    sql.NullTime{Time: now, Valid: true},
			Status:      1,
		}); err != nil {
			return nil, err
		}
	}

	return &pb.BoolResp{Success: true}, nil
}
