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

type ExistLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExistLogic {
	return &ExistLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExistLogic) Exist(in *pb.ExistReq) (*pb.BoolResp, error) {
	if in.Mobile == "" {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	_, err := l.svcCtx.DUserMobileModel.FindOneByMobile(l.ctx, in.Mobile)
	switch {
	case err == nil:
		return nil, status.Error(codes.AlreadyExists, xerr.ErrUserAlreadyExists.Error())
	case errors.Is(err, model.ErrNotFound):
		return &pb.BoolResp{Success: true}, nil
	default:
		return nil, err
	}
}
