package logic

import (
	"context"
	"errors"

	"damai-go/services/program-rpc/internal/model"
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
	ticketCategories, err := l.svcCtx.DTicketCategoryModel.FindByProgramId(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.TicketCategoryDetailListResp{List: []*pb.TicketCategoryDetailInfo{}}, nil
		}
		return nil, err
	}

	return &pb.TicketCategoryDetailListResp{
		List: toTicketCategoryDetailInfoList(ticketCategories),
	}, nil
}
