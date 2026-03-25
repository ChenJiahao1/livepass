package logic

import (
	"context"
	"errors"
	"sort"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSeatRelateInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSeatRelateInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSeatRelateInfoLogic {
	return &GetSeatRelateInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSeatRelateInfoLogic) GetSeatRelateInfo(in *pb.SeatListReq) (*pb.SeatRelateInfo, error) {
	if in.GetProgramId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	program, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}
	showTime, err := l.svcCtx.DProgramShowTimeModel.FindFirstByProgramId(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrProgramShowTimeNotFound.Error())
		}
		return nil, err
	}
	seats, err := l.svcCtx.DSeatModel.FindByProgramID(l.ctx, in.GetProgramId())
	if err != nil {
		return nil, err
	}

	groupMap := make(map[string][]*pb.SeatInfo)
	for _, seat := range seats {
		if seat == nil {
			continue
		}
		priceKey := ticketCategoryPriceString(seat.Price)
		groupMap[priceKey] = append(groupMap[priceKey], &pb.SeatInfo{
			SeatId:           seat.Id,
			TicketCategoryId: seat.TicketCategoryId,
			RowCode:          seat.RowCode,
			ColCode:          seat.ColCode,
			Price:            int64(seat.Price),
		})
	}

	priceList := make([]string, 0, len(groupMap))
	for price := range groupMap {
		priceList = append(priceList, price)
	}
	sort.Slice(priceList, func(i, j int) bool {
		return priceList[i] < priceList[j]
	})

	groups := make([]*pb.PriceSeatGroup, 0, len(priceList))
	for _, price := range priceList {
		groups = append(groups, &pb.PriceSeatGroup{
			Price: price,
			Seats: groupMap[price],
		})
	}

	return &pb.SeatRelateInfo{
		ProgramId:          program.Id,
		Place:              nullStringValue(program.Place),
		ShowTime:           showTime.ShowTime.Format(programDateTimeLayout),
		ShowWeekTime:       showTime.ShowWeekTime,
		PriceList:          priceList,
		PriceSeatGroupList: groups,
	}, nil
}
