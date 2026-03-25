package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BatchCreateSeatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchCreateSeatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCreateSeatsLogic {
	return &BatchCreateSeatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchCreateSeatsLogic) BatchCreateSeats(in *pb.SeatBatchAddReq) (*pb.BoolResp, error) {
	if in.GetProgramId() <= 0 || len(in.GetSeatBatchRelateInfoAddDtoList()) == 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	_, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, err
	}

	for _, item := range in.GetSeatBatchRelateInfoAddDtoList() {
		if item == nil || item.GetTicketCategoryId() <= 0 || item.GetPrice() <= 0 || item.GetCount() <= 0 {
			return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
		}
	}

	insertedCategoryIDs := make([]int64, 0, len(in.GetSeatBatchRelateInfoAddDtoList()))
	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		conn := sqlx.NewSqlConnFromSession(session)
		seatModel := model.NewDSeatModel(conn)
		ticketCategoryModel := model.NewDTicketCategoryModel(conn)

		existingSeats, err := seatModel.FindByProgramID(ctx, in.GetProgramId())
		if err != nil {
			return err
		}
		var rowIndex int64
		for _, seat := range existingSeats {
			if seat != nil && seat.RowCode > rowIndex {
				rowIndex = seat.RowCode
			}
		}

		now := time.Now()
		for _, item := range in.GetSeatBatchRelateInfoAddDtoList() {
			ticketCategory, err := ticketCategoryModel.FindOne(ctx, item.GetTicketCategoryId())
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					return status.Error(codes.NotFound, xerr.ErrProgramTicketCategoryNotFound.Error())
				}
				return err
			}
			if ticketCategory.ProgramId != in.GetProgramId() {
				return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
			}

			remaining := item.GetCount()
			for remaining > 0 {
				rowIndex++
				seatsInRow := int64(10)
				if remaining < seatsInRow {
					seatsInRow = remaining
				}
				for col := int64(1); col <= seatsInRow; col++ {
					if _, err := seatModel.Insert(ctx, &model.DSeat{
						Id:               xid.New(),
						ProgramId:        in.GetProgramId(),
						TicketCategoryId: item.GetTicketCategoryId(),
						RowCode:          rowIndex,
						ColCode:          col,
						SeatType:         1,
						Price:            float64(item.GetPrice()),
						SeatStatus:       1,
						FreezeToken:      sql.NullString{},
						FreezeExpireTime: sql.NullTime{},
						CreateTime:       now,
						EditTime:         now,
						Status:           1,
					}); err != nil {
						return err
					}
				}
				remaining -= seatsInRow
			}

			insertedCategoryIDs = append(insertedCategoryIDs, item.GetTicketCategoryId())
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := clearProgramSeatLedgers(l.ctx, l.svcCtx, in.GetProgramId(), insertedCategoryIDs); err != nil {
		return nil, err
	}

	return &pb.BoolResp{Success: true}, nil
}
