package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetTicketCategoryDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTicketCategoryDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTicketCategoryDetailLogic {
	return &GetTicketCategoryDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTicketCategoryDetailLogic) GetTicketCategoryDetail(in *pb.TicketCategoryReq) (*pb.TicketCategoryDetailInfo, error) {
	if in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	item, err := l.svcCtx.DTicketCategoryModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrProgramTicketCategoryNotFound.Error())
		}
		return nil, err
	}

	return &pb.TicketCategoryDetailInfo{
		ProgramId:    item.ProgramId,
		Introduce:    item.Introduce,
		Price:        int64(item.Price),
		TotalNumber:  item.TotalNumber,
		RemainNumber: item.RemainNumber,
	}, nil
}
