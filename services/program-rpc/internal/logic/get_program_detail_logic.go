package logic

import (
	"context"
	"errors"

	"damai-go/services/program-rpc/internal/programcache"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProgramDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProgramDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProgramDetailLogic {
	return &GetProgramDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProgramDetailLogic) GetProgramDetail(in *pb.GetProgramDetailReq) (*pb.ProgramDetailInfo, error) {
	resp, err := l.svcCtx.ProgramDetailCache.Get(l.ctx, in.GetId())
	if err != nil {
		if errors.Is(err, programcache.ErrProgramNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	return resp, nil
}
