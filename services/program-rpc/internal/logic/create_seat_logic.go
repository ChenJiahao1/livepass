package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xid"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateSeatLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSeatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSeatLogic {
	return &CreateSeatLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateSeatLogic) CreateSeat(in *pb.SeatAddReq) (*pb.IdResp, error) {
	if in.GetProgramId() <= 0 || in.GetTicketCategoryId() <= 0 || in.GetRowCode() <= 0 || in.GetColCode() <= 0 || in.GetSeatType() <= 0 || in.GetPrice() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	if _, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId()); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}
	ticketCategory, err := l.svcCtx.DTicketCategoryModel.FindOne(l.ctx, in.GetTicketCategoryId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, xerr.ErrProgramTicketCategoryNotFound.Error())
		}
		return nil, err
	}
	if ticketCategory.ProgramId != in.GetProgramId() {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if err := ensureShowTimeInventoryMutable(l.ctx, l.svcCtx, ticketCategory.ShowTimeId); err != nil {
		return nil, mapInventoryMutationError(err)
	}

	_, err = l.svcCtx.DSeatModel.FindOneByProgramIdRowCodeColCode(l.ctx, in.GetProgramId(), in.GetRowCode(), in.GetColCode())
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "seat already exists")
	}
	if !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}

	seatStatus := in.GetSeatStatus()
	if seatStatus == 0 {
		seatStatus = 1
	}
	id := xid.New()
	now := time.Now()
	if _, err := l.svcCtx.DSeatModel.Insert(l.ctx, &model.DSeat{
		Id:               id,
		ProgramId:        in.GetProgramId(),
		TicketCategoryId: in.GetTicketCategoryId(),
		RowCode:          in.GetRowCode(),
		ColCode:          in.GetColCode(),
		SeatType:         in.GetSeatType(),
		Price:            float64(in.GetPrice()),
		SeatStatus:       seatStatus,
		FreezeToken:      sql.NullString{},
		FreezeExpireTime: sql.NullTime{},
		CreateTime:       now,
		EditTime:         now,
		Status:           1,
	}); err != nil {
		return nil, err
	}

	if err := clearProgramSeatLedgers(l.ctx, l.svcCtx, in.GetProgramId(), []int64{in.GetTicketCategoryId()}); err != nil {
		return nil, err
	}

	return &pb.IdResp{Id: id}, nil
}
