package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

type AutoAssignAndFreezeSeatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

const (
	seatFreezeStatusFrozen    int64 = 1
	seatFreezeStatusReleased  int64 = 2
	seatFreezeStatusExpired   int64 = 3
	seatFreezeStatusConfirmed int64 = 4
)

func NewAutoAssignAndFreezeSeatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AutoAssignAndFreezeSeatsLogic {
	return &AutoAssignAndFreezeSeatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AutoAssignAndFreezeSeatsLogic) AutoAssignAndFreezeSeats(in *pb.AutoAssignAndFreezeSeatsReq) (*pb.AutoAssignAndFreezeSeatsResp, error) {
	if err := validateAutoAssignAndFreezeSeatsReq(in); err != nil {
		return nil, err
	}

	now := time.Now()
	var resp *pb.AutoAssignAndFreezeSeatsResp

	err := l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		sessionConn := sqlx.NewSqlConnFromSession(session)
		programModel := model.NewDProgramModel(sessionConn)
		showTimeModel := model.NewDProgramShowTimeModel(sessionConn)
		ticketCategoryModel := model.NewDTicketCategoryModel(sessionConn)
		seatModel := model.NewDSeatModel(sessionConn)
		seatFreezeModel := model.NewDSeatFreezeModel(sessionConn)

		if _, err := programModel.FindOne(ctx, in.GetProgramId()); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return programNotFoundError()
			}
			return err
		}

		showTime, err := showTimeModel.FindFirstByProgramId(ctx, in.GetProgramId())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrProgramShowTimeNotFound
			}
			return err
		}

		ticketCategories, err := ticketCategoryModel.FindByProgramId(ctx, in.GetProgramId())
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrProgramTicketCategoryNotFound
			}
			return err
		}

		ticketCategory, ok := findTicketCategory(ticketCategories, in.GetTicketCategoryId())
		if !ok {
			return xerr.ErrProgramTicketCategoryNotFound
		}

		existingFreeze, err := seatFreezeModel.FindOneByRequestNo(ctx, in.GetRequestNo())
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return err
		}
		if err == nil {
			resp, err = buildExistingSeatFreezeResp(ctx, seatModel, existingFreeze, in, now)
			return err
		}

		if err := recycleExpiredSeatFreezes(ctx, seatModel, seatFreezeModel, session, in.GetProgramId(), in.GetTicketCategoryId(), now); err != nil {
			return err
		}

		availableSeats, err := seatModel.FindAvailableByProgramAndTicketCategoryForUpdate(ctx, session, in.GetProgramId(), in.GetTicketCategoryId())
		if err != nil {
			return err
		}

		selectedSeats, err := assignSeats(toSeatCandidates(availableSeats), int(in.GetCount()))
		if err != nil {
			return err
		}

		expireTime := calculateFreezeExpireTime(now, showTime, in.GetFreezeSeconds())
		freezeToken := generateFreezeToken()
		freeze := &model.DSeatFreeze{
			Id:               xid.New(),
			FreezeToken:      freezeToken,
			RequestNo:        in.GetRequestNo(),
			ProgramId:        in.GetProgramId(),
			TicketCategoryId: ticketCategory.Id,
			SeatCount:        int64(len(selectedSeats)),
			FreezeStatus:     seatFreezeStatusFrozen,
			ExpireTime:       expireTime,
			ReleaseReason:    sql.NullString{},
			ReleaseTime:      sql.NullTime{},
			CreateTime:       now,
			EditTime:         now,
			Status:           1,
		}
		if _, err := seatFreezeModel.InsertWithSession(ctx, session, freeze); err != nil {
			return err
		}

		if err := seatModel.BatchFreezeByIDs(ctx, session, seatCandidateIDs(selectedSeats), freezeToken, expireTime); err != nil {
			return err
		}

		resp = &pb.AutoAssignAndFreezeSeatsResp{
			FreezeToken: freezeToken,
			ExpireTime:  expireTime.Format(programDateTimeLayout),
			Seats:       toSeatInfoList(selectedSeats),
		}
		return nil
	})
	if err != nil {
		return nil, mapAutoAssignSeatError(err)
	}

	return resp, nil
}

func validateAutoAssignAndFreezeSeatsReq(in *pb.AutoAssignAndFreezeSeatsReq) error {
	if in.GetProgramId() <= 0 || in.GetTicketCategoryId() <= 0 || in.GetCount() <= 0 || in.GetRequestNo() == "" {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func buildExistingSeatFreezeResp(ctx context.Context, seatModel model.DSeatModel, existingFreeze *model.DSeatFreeze, in *pb.AutoAssignAndFreezeSeatsReq, now time.Time) (*pb.AutoAssignAndFreezeSeatsResp, error) {
	if existingFreeze.ProgramId != in.GetProgramId() ||
		existingFreeze.TicketCategoryId != in.GetTicketCategoryId() ||
		existingFreeze.SeatCount != in.GetCount() {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}
	if existingFreeze.FreezeStatus != seatFreezeStatusFrozen || !existingFreeze.ExpireTime.After(now) {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}

	seats, err := seatModel.FindByFreezeToken(ctx, existingFreeze.FreezeToken)
	if err != nil {
		return nil, err
	}
	if int64(len(seats)) != existingFreeze.SeatCount {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: existingFreeze.FreezeToken,
		ExpireTime:  existingFreeze.ExpireTime.Format(programDateTimeLayout),
		Seats:       toSeatInfoList(toSeatCandidates(seats)),
	}, nil
}

func recycleExpiredSeatFreezes(ctx context.Context, seatModel model.DSeatModel, seatFreezeModel model.DSeatFreezeModel, session sqlx.Session, programId, ticketCategoryId int64, now time.Time) error {
	expiredFreezes, err := seatFreezeModel.FindExpiredByProgramAndTicketCategory(ctx, session, programId, ticketCategoryId, now)
	if err != nil {
		return err
	}
	if len(expiredFreezes) == 0 {
		return nil
	}

	freezeTokens := make([]string, 0, len(expiredFreezes))
	for _, freeze := range expiredFreezes {
		freezeTokens = append(freezeTokens, freeze.FreezeToken)
		if err := seatModel.ReleaseByFreezeToken(ctx, session, freeze.FreezeToken); err != nil {
			return err
		}
	}

	return seatFreezeModel.MarkExpiredByFreezeTokens(ctx, session, freezeTokens, now)
}

func calculateFreezeExpireTime(now time.Time, showTime *model.DProgramShowTime, freezeSeconds int64) time.Time {
	seconds := freezeSeconds
	if seconds <= 0 {
		seconds = 900
	}

	expireTime := now.Add(time.Duration(seconds) * time.Second)
	if showTime != nil && showTime.ShowTime.Before(expireTime) {
		return showTime.ShowTime
	}

	return expireTime
}

func generateFreezeToken() string {
	return fmt.Sprintf("freeze-%d", xid.New())
}

func findTicketCategory(ticketCategories []*model.DTicketCategory, ticketCategoryId int64) (*model.DTicketCategory, bool) {
	for _, ticketCategory := range ticketCategories {
		if ticketCategory.Id == ticketCategoryId {
			return ticketCategory, true
		}
	}

	return nil, false
}

func toSeatCandidates(seats []*model.DSeat) []seatCandidate {
	resp := make([]seatCandidate, 0, len(seats))
	for _, seat := range seats {
		resp = append(resp, seatCandidate{
			ID:               seat.Id,
			TicketCategoryID: seat.TicketCategoryId,
			RowCode:          seat.RowCode,
			ColCode:          seat.ColCode,
			Price:            seat.Price,
		})
	}

	return resp
}

func seatCandidateIDs(seats []seatCandidate) []int64 {
	ids := make([]int64, 0, len(seats))
	for _, seat := range seats {
		ids = append(ids, seat.ID)
	}

	return ids
}

func toSeatInfoList(seats []seatCandidate) []*pb.SeatInfo {
	resp := make([]*pb.SeatInfo, 0, len(seats))
	for _, seat := range seats {
		resp = append(resp, &pb.SeatInfo{
			SeatId:           seat.ID,
			TicketCategoryId: seat.TicketCategoryID,
			RowCode:          seat.RowCode,
			ColCode:          seat.ColCode,
			Price:            int64(seat.Price),
		})
	}

	return resp
}

func mapAutoAssignSeatError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrProgramShowTimeNotFound), errors.Is(err, xerr.ErrProgramTicketCategoryNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrSeatInventoryInsufficient), errors.Is(err, xerr.ErrSeatFreezeRequestConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return err
	}
}
