package logic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &pb.ProgramDetailInfo{}, nil
}
