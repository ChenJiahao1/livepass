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

type UpdateEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateEmailLogic {
	return &UpdateEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateEmailLogic) UpdateEmail(in *pb.UpdateEmailReq) (*pb.BoolResp, error) {
	user, err := l.svcCtx.DUserModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}
	if in.Email != "" && (!user.Email.Valid || user.Email.String != in.Email) {
		if existing, err := l.svcCtx.DUserEmailModel.FindOneByEmail(l.ctx, in.Email); err == nil && existing.UserId != user.Id {
			return nil, status.Error(codes.AlreadyExists, xerr.ErrEmailAlreadyUsed.Error())
		} else if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, err
		}
		if user.Email.Valid && user.Email.String != "" {
			if oldMapping, err := l.svcCtx.DUserEmailModel.FindOneByEmail(l.ctx, user.Email.String); err == nil {
				if err := l.svcCtx.DUserEmailModel.Delete(l.ctx, oldMapping.Id); err != nil {
					return nil, err
				}
			}
		}
		if _, err := l.svcCtx.DUserEmailModel.Insert(l.ctx, &model.DUserEmail{
			Id:          xid.New(),
			UserId:      user.Id,
			Email:       in.Email,
			EmailStatus: in.EmailStatus,
			EditTime:    sql.NullTime{Time: time.Now(), Valid: true},
			Status:      1,
		}); err != nil {
			return nil, err
		}
	}
	user.Email = sql.NullString{String: in.Email, Valid: in.Email != ""}
	user.EmailStatus = in.EmailStatus
	user.EditTime = sql.NullTime{Time: time.Now(), Valid: true}
	if err := l.svcCtx.DUserModel.Update(l.ctx, user); err != nil {
		return nil, err
	}
	return &pb.BoolResp{Success: true}, nil
}
