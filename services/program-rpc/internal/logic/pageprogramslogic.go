package logic

import (
	"context"

	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PageProgramsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPageProgramsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PageProgramsLogic {
	return &PageProgramsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PageProgramsLogic) PagePrograms(in *pb.PageProgramsReq) (*pb.ProgramPageResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ProgramPageResp{}, nil
}
