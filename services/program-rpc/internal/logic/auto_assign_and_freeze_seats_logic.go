package logic

import (
	"context"
	"errors"
	"time"

	"livepass/pkg/seatfreeze"
	"livepass/pkg/xerr"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/seatcache"
	"livepass/services/program-rpc/internal/svc"
	"livepass/services/program-rpc/pb"

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

	var reservedSeats []seatcache.Seat

	_, err := l.svcCtx.DProgramShowTimeModel.FindOne(l.ctx, in.GetShowTimeId())
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

	existingSeats, err := seatStore.FrozenSeats(l.ctx, in.GetShowTimeId(), in.GetTicketCategoryId(), in.GetFreezeToken())
	if err != nil {
		return nil, mapAutoAssignSeatError(err)
	}
	if len(existingSeats) > 0 {
		resp, err := buildExistingSeatFreezeResp(l.ctx, l.svcCtx.DSeatModel, in, existingSeats)
		if err != nil {
			return nil, mapAutoAssignSeatError(err)
		}
		return resp, nil
	}

	dbSeats, err := l.svcCtx.DSeatModel.FindByFreezeToken(l.ctx, in.GetFreezeToken())
	if err != nil {
		return nil, mapAutoAssignSeatError(err)
	}
	if len(dbSeats) > 0 {
		return nil, mapAutoAssignSeatError(xerr.ErrSeatFreezeStatusInvalid)
	}

	reservedSeats, err = seatStore.FreezeAutoAssignedSeats(l.ctx, in.GetShowTimeId(), in.GetTicketCategoryId(), in.GetFreezeToken(), in.GetCount())
	if err != nil {
		if in.GetFreezeToken() != "" && len(reservedSeats) > 0 {
			_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetShowTimeId(), in.GetTicketCategoryId(), in.GetFreezeToken())
		}
		return nil, mapAutoAssignSeatError(err)
	}

	selectedSeats := toSeatCandidatesFromLedger(reservedSeats)
	expireTime, err := parseFreezeExpireTime(in.GetFreezeExpireTime())
	if err != nil {
		_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetShowTimeId(), in.GetTicketCategoryId(), in.GetFreezeToken())
		return nil, mapAutoAssignSeatError(xerr.ErrInvalidParam)
	}
	if err := l.persistFrozenSeatsToDB(in.GetShowTimeId(), seatCandidateIDs(selectedSeats), in.GetFreezeToken(), expireTime); err != nil {
		_ = seatStore.ReleaseFrozenSeats(context.Background(), in.GetShowTimeId(), in.GetTicketCategoryId(), in.GetFreezeToken())
		return nil, mapAutoAssignSeatError(err)
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: in.GetFreezeToken(),
		ExpireTime:  expireTime.Format(programDateTimeLayout),
		Seats:       toSeatInfoList(selectedSeats),
	}, nil
}

func validateAutoAssignAndFreezeSeatsReq(in *pb.AutoAssignAndFreezeSeatsReq) error {
	if in.GetShowTimeId() <= 0 || in.GetTicketCategoryId() <= 0 || in.GetCount() <= 0 || in.GetFreezeToken() == "" {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	token, err := seatfreeze.ParseToken(in.GetFreezeToken())
	if err != nil {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if token.ShowTimeID != in.GetShowTimeId() || token.TicketCategoryID != in.GetTicketCategoryId() {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func buildExistingSeatFreezeResp(ctx context.Context, seatModel model.DSeatModel, in *pb.AutoAssignAndFreezeSeatsReq, seats []seatcache.Seat) (*pb.AutoAssignAndFreezeSeatsResp, error) {
	if int64(len(seats)) != in.GetCount() {
		return nil, xerr.ErrSeatFreezeRequestConflict
	}
	expireTime, err := validateFrozenSeatsPersistedInDB(ctx, seatModel, in.GetShowTimeId(), in.GetTicketCategoryId(), in.GetFreezeToken(), seats)
	if err != nil {
		return nil, err
	}

	return &pb.AutoAssignAndFreezeSeatsResp{
		FreezeToken: in.GetFreezeToken(),
		ExpireTime:  expireTime.Format(programDateTimeLayout),
		Seats:       toSeatInfoList(toSeatCandidatesFromLedger(seats)),
	}, nil
}

func validateFrozenSeatsPersistedInDB(ctx context.Context, seatModel model.DSeatModel, showTimeID, ticketCategoryID int64, freezeToken string, seats []seatcache.Seat) (time.Time, error) {
	if seatModel == nil {
		return time.Time{}, xerr.ErrSeatFreezeStatusInvalid
	}

	dbSeats, err := seatModel.FindByFreezeToken(ctx, freezeToken)
	if err != nil {
		return time.Time{}, err
	}
	if len(dbSeats) != len(seats) {
		return time.Time{}, xerr.ErrSeatFreezeRequestConflict
	}

	expectedSeatIDs := make(map[int64]struct{}, len(seats))
	for _, seat := range seats {
		expectedSeatIDs[seat.SeatID] = struct{}{}
	}

	var expireTime time.Time
	for _, seat := range dbSeats {
		if seat.ShowTimeId != showTimeID ||
			seat.TicketCategoryId != ticketCategoryID ||
			seat.SeatStatus != 2 {
			return time.Time{}, xerr.ErrSeatFreezeRequestConflict
		}
		if _, ok := expectedSeatIDs[seat.Id]; !ok {
			return time.Time{}, xerr.ErrSeatFreezeRequestConflict
		}
		delete(expectedSeatIDs, seat.Id)
		if !seat.FreezeExpireTime.Valid {
			return time.Time{}, xerr.ErrSeatFreezeStatusInvalid
		}
		if expireTime.IsZero() {
			expireTime = seat.FreezeExpireTime.Time
			continue
		}
		if !expireTime.Equal(seat.FreezeExpireTime.Time) {
			return time.Time{}, xerr.ErrSeatFreezeStatusInvalid
		}
	}
	if len(expectedSeatIDs) != 0 {
		return time.Time{}, xerr.ErrSeatFreezeRequestConflict
	}

	return expireTime, nil
}

func parseFreezeExpireTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, xerr.ErrInvalidParam
	}

	expireTime, err := time.ParseInLocation(programDateTimeLayout, value, time.Local)
	if err != nil || expireTime.IsZero() {
		return time.Time{}, xerr.ErrInvalidParam
	}

	return expireTime, nil
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
