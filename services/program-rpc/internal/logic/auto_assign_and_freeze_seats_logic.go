package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/seatcache"
	"damai-go/services/program-rpc/internal/svc"
	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AutoAssignAndFreezeSeatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAutoAssignAndFreezeSeatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AutoAssignAndFreezeSeatsLogic {
	return &AutoAssignAndFreezeSeatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func ensureSeatStockStore(svcCtx *svc.ServiceContext) *seatcache.SeatStockStore {
	if svcCtx == nil {
		return nil
	}

	return svcCtx.SeatStockStore
}

func (l *AutoAssignAndFreezeSeatsLogic) AutoAssignAndFreezeSeats(in *pb.AutoAssignAndFreezeSeatsReq) (*pb.AutoAssignAndFreezeSeatsResp, error) {
	if err := validateAutoAssignAndFreezeSeatsReq(in); err != nil {
		return nil, err
	}

	seatStore := ensureSeatStockStore(l.svcCtx)
	if seatStore == nil {
		return nil, mapAutoAssignSeatError(xerr.ErrProgramSeatLedgerNotReady)
	}

	unlock := ensureSeatFreezeLocker(l.svcCtx).Lock(seatFreezeLockKey(in.GetProgramId(), in.GetTicketCategoryId()))
	defer unlock()

	now := time.Now()
	var freezeToken string
	var reservedSeats []seatcache.Seat

	if _, err := l.svcCtx.DProgramModel.FindOne(l.ctx, in.GetProgramId()); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, programNotFoundError()
		}
		return nil, mapAutoAssignSeatError(err)
	}

	showTime, err := l.svcCtx.DProgramShowTimeModel.FindFirstByProgramId(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapAutoAssignSeatError(xerr.ErrProgramShowTimeNotFound)
		}
		return nil, mapAutoAssignSeatError(err)
	}

	ticketCategories, err := l.svcCtx.DTicketCategoryModel.FindByProgramId(l.ctx, in.GetProgramId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapAutoAssignSeatError(xerr.ErrProgramTicketCategoryNotFound)
		}
		return nil, mapAutoAssignSeatError(err)
	}

	if _, ok := findTicketCategory(ticketCategories, in.GetTicketCategoryId()); !ok {
		return nil, mapAutoAssignSeatError(xerr.ErrProgramTicketCategoryNotFound)
	}

	existingFreeze, err := seatStore.GetFreezeMetadataByRequestNo(l.ctx, in.GetRequestNo())
	if err != nil {
		return nil, mapAutoAssignSeatError(err)
	}
	if existingFreeze != nil {
		resp, err := buildExistingSeatFreezeResp(l.ctx, seatStore, existingFreeze, in, now)
		if err != nil {
			return nil, mapAutoAssignSeatError(err)
		}
		return resp, nil
	}

	if err := recycleExpiredSeatFreezes(l.ctx, seatStore, in.GetProgramId(), in.GetTicketCategoryId(), now); err != nil {
		return nil, mapAutoAssignSeatError(err)
	}

	freezeToken = generateFreezeToken()
	reservedSeats, err = seatStore.FreezeAutoAssignedSeats(l.ctx, in.GetProgramId(), in.GetTicketCategoryId(), freezeToken, in.GetCount())
	if err != nil {
		if freezeToken != "" && len(reservedSeats) > 0 {
			_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetProgramId(), in.GetTicketCategoryId(), freezeToken)
		}
		return nil, mapAutoAssignSeatError(err)
	}

	selectedSeats := toSeatCandidatesFromLedger(reservedSeats)
	expireTime := calculateFreezeExpireTime(now, showTime, in.GetFreezeSeconds())
	freeze := &seatcache.SeatFreezeMetadata{
		FreezeToken:      freezeToken,
		RequestNo:        in.GetRequestNo(),
		ProgramID:        in.GetProgramId(),
		TicketCategoryID: in.GetTicketCategoryId(),
		SeatCount:        int64(len(selectedSeats)),
		FreezeStatus:     seatcache.SeatFreezeStatusFrozen,
		ExpireAt:         expireTime.Unix(),
		UpdatedAt:        now.Unix(),
	}
	if err := seatStore.SaveFreezeMetadata(l.ctx, freeze); err != nil {
		_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetProgramId(), in.GetTicketCategoryId(), freezeToken)
		return nil, mapAutoAssignSeatError(err)
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: freezeToken,
		ExpireTime:  expireTime.Format(programDateTimeLayout),
		Seats:       toSeatInfoList(selectedSeats),
	}, nil
}

func validateAutoAssignAndFreezeSeatsReq(in *pb.AutoAssignAndFreezeSeatsReq) error {
	if in.GetProgramId() <= 0 || in.GetTicketCategoryId() <= 0 || in.GetCount() <= 0 || in.GetRequestNo() == "" {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func buildExistingSeatFreezeResp(ctx context.Context, seatStore *seatcache.SeatStockStore, existingFreeze *seatcache.SeatFreezeMetadata, in *pb.AutoAssignAndFreezeSeatsReq, now time.Time) (*pb.AutoAssignAndFreezeSeatsResp, error) {
	if existingFreeze.ProgramID != in.GetProgramId() ||
		existingFreeze.TicketCategoryID != in.GetTicketCategoryId() ||
		existingFreeze.SeatCount != in.GetCount() {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}
	if existingFreeze.FreezeStatus != seatcache.SeatFreezeStatusFrozen || !existingFreeze.ExpireTime().After(now) {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}

	seats, err := seatStore.FrozenSeats(ctx, existingFreeze.ProgramID, existingFreeze.TicketCategoryID, existingFreeze.FreezeToken)
	if err != nil {
		return nil, err
	}
	if int64(len(seats)) != existingFreeze.SeatCount {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: existingFreeze.FreezeToken,
		ExpireTime:  existingFreeze.ExpireTime().Format(programDateTimeLayout),
		Seats:       toSeatInfoList(toSeatCandidatesFromLedger(seats)),
	}, nil
}

func recycleExpiredSeatFreezes(ctx context.Context, seatStore *seatcache.SeatStockStore, programId, ticketCategoryId int64, now time.Time) error {
	expiredFreezeTokens, err := seatStore.ListExpiredFreezeTokens(ctx, programId, ticketCategoryId, now)
	if err != nil {
		return err
	}
	if len(expiredFreezeTokens) == 0 {
		return nil
	}

	for _, freezeToken := range expiredFreezeTokens {
		if err := seatStore.ReleaseFrozenSeats(ctx, programId, ticketCategoryId, freezeToken); err != nil {
			return err
		}
		if _, err := seatStore.MarkFreezeExpired(ctx, freezeToken, now); err != nil {
			return err
		}
	}

	return nil
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

func toSeatCandidatesFromLedger(seats []seatcache.Seat) []seatCandidate {
	resp := make([]seatCandidate, 0, len(seats))
	for _, seat := range seats {
		resp = append(resp, seatCandidate{
			ID:               seat.SeatID,
			TicketCategoryID: seat.TicketCategoryID,
			RowCode:          seat.RowCode,
			ColCode:          seat.ColCode,
			Price:            float64(seat.Price),
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
	case errors.Is(err, xerr.ErrSeatInventoryInsufficient), errors.Is(err, xerr.ErrSeatFreezeRequestConflict), errors.Is(err, xerr.ErrProgramSeatLedgerNotReady):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return err
	}
}
