package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xid"
	"livepass/services/user-rpc/internal/model"
	"livepass/services/user-rpc/internal/svc"
	"livepass/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserLogic {
	return &UpdateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserLogic) UpdateUser(in *pb.UpdateUserReq) (*pb.BoolResp, error) {
	user, err := l.svcCtx.DUserModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}

	now := time.Now()
	oldMobile := user.Mobile
	if in.Name != "" {
		user.Name = sql.NullString{String: in.Name, Valid: true}
	}
	if in.Address != "" {
		user.Address = sql.NullString{String: in.Address, Valid: true}
	}
	if in.Gender != 0 {
		user.Gender = in.Gender
	}
	if in.Mobile != "" && in.Mobile != oldMobile {
		if existing, err := l.svcCtx.DUserMobileModel.FindOneByMobile(l.ctx, in.Mobile); err == nil && existing.UserId != user.Id {
			return nil, status.Error(codes.AlreadyExists, xerr.ErrMobileAlreadyUsed.Error())
		} else if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, err
		}
		user.Mobile = in.Mobile
		if oldMapping, err := l.svcCtx.DUserMobileModel.FindOneByMobile(l.ctx, oldMobile); err == nil {
			if err := l.svcCtx.DUserMobileModel.Delete(l.ctx, oldMapping.Id); err != nil {
				return nil, err
			}
		}
		if _, err := l.svcCtx.DUserMobileModel.Insert(l.ctx, &model.DUserMobile{
			Id:       xid.New(),
			UserId:   user.Id,
			Mobile:   in.Mobile,
			EditTime: sql.NullTime{Time: now, Valid: true},
			Status:   1,
		}); err != nil {
			return nil, err
		}
	}
	user.EditTime = sql.NullTime{Time: now, Valid: true}
	if err := l.svcCtx.DUserModel.Update(l.ctx, user); err != nil {
		return nil, err
	}

	return &pb.BoolResp{Success: true}, nil
}
