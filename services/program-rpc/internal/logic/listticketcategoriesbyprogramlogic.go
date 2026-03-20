package logic

import (
	"context"

	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTicketCategoriesByProgramLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTicketCategoriesByProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketCategoriesByProgramLogic {
	return &ListTicketCategoriesByProgramLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTicketCategoriesByProgramLogic) ListTicketCategoriesByProgram(in *pb.ListTicketCategoriesByProgramReq) (*pb.TicketCategoryDetailListResp, error) {
	// todo: add your logic here and delete this line

	return &pb.TicketCategoryDetailListResp{}, nil
}
