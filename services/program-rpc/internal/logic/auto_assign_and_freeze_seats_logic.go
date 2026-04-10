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
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

	unlock := ensureSeatFreezeLocker(l.svcCtx).Lock(seatFreezeLockKey(in.GetShowTimeId(), in.GetTicketCategoryId()))
	defer unlock()

	now := time.Now()
	var freezeToken string
	var reservedSeats []seatcache.Seat

	showTime, err := l.svcCtx.DProgramShowTimeModel.FindOne(l.ctx, in.GetShowTimeId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapAutoAssignSeatError(xerr.ErrProgramShowTimeNotFound)
		}
		return nil, mapAutoAssignSeatError(err)
	}

	ticketCategories, err := l.svcCtx.DTicketCategoryModel.FindByShowTimeId(l.ctx, in.GetShowTimeId())
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
		resp, err := buildExistingSeatFreezeResp(l.ctx, seatStore, l.svcCtx.DSeatModel, existingFreeze, in, now)
		if err != nil {
			return nil, mapAutoAssignSeatError(err)
		}
		return resp, nil
	}

	if err := recycleExpiredSeatFreezes(l.ctx, seatStore, in.GetShowTimeId(), in.GetTicketCategoryId(), now); err != nil {
		return nil, mapAutoAssignSeatError(err)
	}

	freezeToken = generateFreezeToken()
	reservedSeats, err = seatStore.FreezeAutoAssignedSeats(l.ctx, in.GetShowTimeId(), in.GetTicketCategoryId(), freezeToken, in.GetCount())
	if err != nil {
		if freezeToken != "" && len(reservedSeats) > 0 {
			_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetShowTimeId(), in.GetTicketCategoryId(), freezeToken, in.GetOwnerOrderNumber(), in.GetOwnerEpoch())
		}
		return nil, mapAutoAssignSeatError(err)
	}

	selectedSeats := toSeatCandidatesFromLedger(reservedSeats)
	expireTime := calculateFreezeExpireTime(now, showTime, in.GetFreezeSeconds())
	if err := l.persistFrozenSeatsToDB(in.GetShowTimeId(), seatCandidateIDs(selectedSeats), freezeToken, expireTime); err != nil {
		_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetShowTimeId(), in.GetTicketCategoryId(), freezeToken, in.GetOwnerOrderNumber(), in.GetOwnerEpoch())
		return nil, mapAutoAssignSeatError(err)
	}

	freeze := &seatcache.SeatFreezeMetadata{
		FreezeToken:      freezeToken,
		RequestNo:        in.GetRequestNo(),
		ProgramID:        showTime.ProgramId,
		ShowTimeID:       in.GetShowTimeId(),
		TicketCategoryID: in.GetTicketCategoryId(),
		OwnerOrderNumber: in.GetOwnerOrderNumber(),
		OwnerEpoch:       in.GetOwnerEpoch(),
		SeatCount:        int64(len(selectedSeats)),
		FreezeStatus:     seatcache.SeatFreezeStatusFrozen,
		ExpireAt:         expireTime.Unix(),
		UpdatedAt:        now.Unix(),
	}
	if err := seatStore.SaveFreezeMetadata(l.ctx, freeze); err != nil {
		if rollbackErr := l.rollbackFrozenSeatsInDB(in.GetShowTimeId(), freezeToken); rollbackErr != nil {
			l.Errorf("rollback frozen seats in db failed, showTimeId=%d freezeToken=%s err=%v", in.GetShowTimeId(), freezeToken, rollbackErr)
		}
		_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetShowTimeId(), in.GetTicketCategoryId(), freezeToken, in.GetOwnerOrderNumber(), in.GetOwnerEpoch())
		return nil, mapAutoAssignSeatError(err)
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: freezeToken,
		ExpireTime:  expireTime.Format(programDateTimeLayout),
		Seats:       toSeatInfoList(selectedSeats),
	}, nil
}

func validateAutoAssignAndFreezeSeatsReq(in *pb.AutoAssignAndFreezeSeatsReq) error {
	if in.GetShowTimeId() <= 0 || in.GetTicketCategoryId() <= 0 || in.GetCount() <= 0 || in.GetRequestNo() == "" {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if !hasSeatFreezeOwner(in.GetOwnerOrderNumber(), in.GetOwnerEpoch()) &&
		(in.GetOwnerOrderNumber() != 0 || in.GetOwnerEpoch() != 0) {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func buildExistingSeatFreezeResp(ctx context.Context, seatStore *seatcache.SeatStockStore, seatModel model.DSeatModel, existingFreeze *seatcache.SeatFreezeMetadata, in *pb.AutoAssignAndFreezeSeatsReq, now time.Time) (*pb.AutoAssignAndFreezeSeatsResp, error) {
	if existingFreeze.ShowTimeID != in.GetShowTimeId() ||
		existingFreeze.TicketCategoryID != in.GetTicketCategoryId() ||
		existingFreeze.SeatCount != in.GetCount() ||
		existingFreeze.OwnerOrderNumber != in.GetOwnerOrderNumber() ||
		existingFreeze.OwnerEpoch != in.GetOwnerEpoch() {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}
	if existingFreeze.FreezeStatus != seatcache.SeatFreezeStatusFrozen || !existingFreeze.ExpireTime().After(now) {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}

	seats, err := seatStore.FrozenSeats(ctx, existingFreeze.ShowTimeID, existingFreeze.TicketCategoryID, existingFreeze.FreezeToken)
	if err != nil {
		return nil, err
	}
	if int64(len(seats)) != existingFreeze.SeatCount {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}
	if err := validateFrozenSeatsPersistedInDB(ctx, seatModel, existingFreeze, seats); err != nil {
		return nil, err
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: existingFreeze.FreezeToken,
		ExpireTime:  existingFreeze.ExpireTime().Format(programDateTimeLayout),
		Seats:       toSeatInfoList(toSeatCandidatesFromLedger(seats)),
	}, nil
}

func validateFrozenSeatsPersistedInDB(ctx context.Context, seatModel model.DSeatModel, existingFreeze *seatcache.SeatFreezeMetadata, seats []seatcache.Seat) error {
	if seatModel == nil {
		return xerr.ErrSeatFreezeStatusInvalid
	}

	dbSeats, err := seatModel.FindByFreezeToken(ctx, existingFreeze.FreezeToken)
	if err != nil {
		return err
	}
	if len(dbSeats) != len(seats) {
		return xerr.ErrSeatFreezeRequestConflict
	}

	expectedSeatIDs := make(map[int64]struct{}, len(seats))
	for _, seat := range seats {
		expectedSeatIDs[seat.SeatID] = struct{}{}
	}
	for _, seat := range dbSeats {
		if seat.ShowTimeId != existingFreeze.ShowTimeID ||
			seat.TicketCategoryId != existingFreeze.TicketCategoryID ||
			seat.SeatStatus != 2 {
			return xerr.ErrSeatFreezeRequestConflict
		}
		if _, ok := expectedSeatIDs[seat.Id]; !ok {
			return xerr.ErrSeatFreezeRequestConflict
		}
		delete(expectedSeatIDs, seat.Id)
	}
	if len(expectedSeatIDs) != 0 {
		return xerr.ErrSeatFreezeRequestConflict
	}

	return nil
}

func recycleExpiredSeatFreezes(ctx context.Context, seatStore *seatcache.SeatStockStore, showTimeID, ticketCategoryId int64, now time.Time) error {
	expiredFreezeTokens, err := seatStore.ListExpiredFreezeTokens(ctx, showTimeID, ticketCategoryId, now)
	if err != nil {
		return err
	}
	if len(expiredFreezeTokens) == 0 {
		return nil
	}

	for _, freezeToken := range expiredFreezeTokens {
		if err := seatStore.ReleaseFrozenSeats(ctx, showTimeID, ticketCategoryId, freezeToken, 0, 0); err != nil {
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

func hasSeatFreezeOwner(orderNumber, epoch int64) bool {
	return orderNumber > 0 && epoch > 0
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

func (l *AutoAssignAndFreezeSeatsLogic) persistFrozenSeatsToDB(showTimeID int64, seatIDs []int64, freezeToken string, expireTime time.Time) error {
	if len(seatIDs) == 0 {
		return xerr.ErrSeatInventoryInsufficient
	}

	return l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		seatModel := model.NewDSeatModel(sqlx.NewSqlConnFromSession(session))
		seats, err := seatModel.FindByShowTimeAndIDsForUpdate(ctx, session, showTimeID, seatIDs)
		if err != nil {
			return err
		}
		if len(seats) != len(seatIDs) {
			return xerr.ErrSeatFreezeStatusInvalid
		}
		for _, seat := range seats {
			if seat.SeatStatus != 1 {
				return xerr.ErrSeatFreezeStatusInvalid
			}
		}

		return seatModel.BatchFreezeByShowTimeAndIDs(ctx, session, showTimeID, seatIDs, freezeToken, expireTime)
	})
}

func (l *AutoAssignAndFreezeSeatsLogic) rollbackFrozenSeatsInDB(showTimeID int64, freezeToken string) error {
	if freezeToken == "" {
		return nil
	}

	return l.svcCtx.SqlConn.TransactCtx(context.Background(), func(ctx context.Context, session sqlx.Session) error {
		seatModel := model.NewDSeatModel(sqlx.NewSqlConnFromSession(session))
		return seatModel.ReleaseByFreezeToken(ctx, session, freezeToken)
	})
}

func mapAutoAssignSeatError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrProgramShowTimeNotFound), errors.Is(err, xerr.ErrProgramTicketCategoryNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrSeatInventoryInsufficient), errors.Is(err, xerr.ErrSeatFreezeRequestConflict), errors.Is(err, xerr.ErrProgramSeatLedgerNotReady), errors.Is(err, xerr.ErrSeatFreezeStatusInvalid):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return err
	}
}
