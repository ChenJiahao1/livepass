package logic

import (
	"context"

	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListHomeProgramsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHomeProgramsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHomeProgramsLogic {
	return &ListHomeProgramsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListHomeProgramsLogic) ListHomePrograms(in *pb.ListHomeProgramsReq) (*pb.ProgramHomeListResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ProgramHomeListResp{}, nil
}
