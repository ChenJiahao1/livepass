package logic

import (
	"context"
	"errors"

	"livepass/pkg/xerr"
	"livepass/services/user-rpc/internal/model"
	"livepass/services/user-rpc/internal/svc"
	"livepass/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetUserAndTicketUserListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserAndTicketUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserAndTicketUserListLogic {
	return &GetUserAndTicketUserListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserAndTicketUserListLogic) GetUserAndTicketUserList(in *pb.GetUserAndTicketUserListReq) (*pb.GetUserAndTicketUserListResp, error) {
	user, err := l.svcCtx.DUserModel.FindOne(l.ctx, in.UserId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrUserNotFound.Error())
		}
		return nil, err
	}
	list, err := l.svcCtx.DTicketUserModel.FindByUserId(l.ctx, in.UserId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	resp := &pb.GetUserAndTicketUserListResp{
		UserVo: buildUserInfo(user),
	}
	for _, item := range list {
		resp.TicketUserVoList = append(resp.TicketUserVoList, buildTicketUserInfo(item))
	}
	return resp, nil
}
