package logic

import (
	"context"
	"errors"

	"livepass/services/program-rpc/internal/programcache"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProgramDetailViewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProgramDetailViewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramDetailViewLogic {
	return &GetProgramDetailViewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProgramDetailViewLogic) GetProgramDetailView(in *pb.GetProgramDetailViewReq) (*pb.ProgramDetailViewInfo, error) {
	resp, err := l.svcCtx.ProgramDetailViewCache.Get(l.ctx, in.GetId())
	if err != nil {
		if errors.Is(err, programcache.ErrProgramNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	return resp, nil
}
