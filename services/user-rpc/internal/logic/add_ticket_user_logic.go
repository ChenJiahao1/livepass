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

type AddTicketUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddTicketUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddTicketUserLogic {
	return &AddTicketUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddTicketUserLogic) AddTicketUser(in *pb.AddTicketUserReq) (*pb.BoolResp, error) {
	if _, err := l.svcCtx.DUserModel.FindOne(l.ctx, in.UserId); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}
	if _, err := l.svcCtx.DTicketUserModel.FindOneByUserIdAndIdTypeAndIdNumber(l.ctx, in.UserId, in.IdType, in.IdNumber); err == nil {
		return nil, status.Error(codes.AlreadyExists, xerr.ErrTicketUserExists.Error())
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	if _, err := l.svcCtx.DTicketUserModel.Insert(l.ctx, &model.DTicketUser{
		Id:       xid.New(),
		UserId:   in.UserId,
		RelName:  in.RelName,
		IdType:   in.IdType,
		IdNumber: in.IdNumber,
		EditTime: sql.NullTime{Time: time.Now(), Valid: true},
		Status:   1,
	}); err != nil {
		return nil, err
	}

	return &pb.BoolResp{Success: true}, nil
}
