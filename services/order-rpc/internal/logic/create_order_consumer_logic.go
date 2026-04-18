package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"livepass/pkg/seatfreeze"
	"livepass/pkg/xerr"
	orderevent "livepass/services/order-rpc/internal/event"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/repository"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const orderCreateGuardConflictReleaseReason = "order_create_guard_conflict"
const orderCreatePersistFailureReason = "ORDER_PERSIST_FAILED"

type CreateOrderConsumerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderConsumerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderConsumerLogic {
	return &CreateOrderConsumerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateOrderConsumerLogic) Consume(body []byte) error {
	orderEvent, err := orderevent.UnmarshalOrderCreateEvent(body)
	if err != nil {
		return err
	}
	if err := validateOrderCreateEvent(orderEvent); err != nil {
		return err
	}

	now := time.Now()
	occurredAt, err := parseOrderTime(orderEvent.OccurredAt)
	if err != nil {
		return err
	}
	showTimeID := orderEvent.ShowTimeID
	if showTimeID <= 0 {
		showTimeID = orderEvent.ProgramID
	}
	attempt, processingEpoch, shouldProcess, err := l.prepareAttemptForConsume(showTimeID, orderEvent.OrderNumber, now)
	if err != nil {
		if errors.Is(err, xerr.ErrOrderNotFound) && hasEmbeddedOrderCreateSnapshots(orderEvent) {
			shouldProcess = true
		} else if errors.Is(err, xerr.ErrOrderNotFound) {
			return nil
		} else {
			return err
		}
	}
	if !shouldProcess {
		return nil
	}

	var lease *processingLease
	if attempt != nil && processingEpoch > 0 {
		lease = startProcessingLease(l.ctx, l.svcCtx.AttemptStore, attempt.OrderNumber, processingEpoch, processingLeaseInterval(l.svcCtx))
		defer lease.stop()
	}

	if existing, err := l.svcCtx.OrderRepository.FindOrderByNumber(l.ctx, orderEvent.OrderNumber); err == nil && existing != nil {
		l.finalizeSuccess(attempt, processingEpoch, extractSeatIDs(orderEvent.SeatSnapshot), now, lease)
		return nil
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}

	enrichedEvent, freezeResp, err := l.buildConsumerOrderEvent(orderEvent, attempt, processingEpoch, occurredAt)
	if err != nil {
		var freezeErr *seatFreezeError
		if errors.As(err, &freezeErr) && isTerminalSeatFreezeError(freezeErr.err) {
			return l.finalizeFailure(attempt, processingEpoch, rush.AttemptReasonSeatExhausted, "", "")
		}
		return err
	}
	if lease != nil && lease.lost.Load() {
		return nil
	}

	writeModels, err := mapEventToOrderWriteModels(enrichedEvent, now)
	if err != nil {
		return err
	}

	err = l.svcCtx.OrderRepository.TransactByOrderNumber(l.ctx, orderEvent.OrderNumber, func(ctx context.Context, tx repository.OrderTx) error {
		if err := tx.InsertOrder(ctx, writeModels.order); err != nil {
			if isDuplicateOrderNumberErr(err) {
				return nil
			}
			return err
		}
		if err := tx.InsertOrderTickets(ctx, writeModels.orderTickets); err != nil {
			return err
		}
		if err := tx.InsertUserGuard(ctx, writeModels.userGuard); err != nil {
			return err
		}
		if err := tx.InsertViewerGuards(ctx, writeModels.viewerGuards); err != nil {
			return err
		}
		if err := tx.InsertSeatGuards(ctx, writeModels.seatGuards); err != nil {
			return err
		}
		if err := tx.InsertDelayTasks(ctx, writeModels.delayTaskRows); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if isGuardConflictErr(err) {
			return l.finalizeFailure(attempt, processingEpoch, rush.AttemptReasonAlreadyHasActiveOrder, writeModels.order.FreezeToken, orderCreateGuardConflictReleaseReason)
		}
		return l.resolveOrderPersistFailure(orderEvent.OrderNumber, attempt, processingEpoch, extractSeatIDs(enrichedEvent.SeatSnapshot), freezeResp, err, now, lease)
	}

	l.finalizeSuccess(attempt, processingEpoch, extractSeatIDs(enrichedEvent.SeatSnapshot), now, lease)
	return nil
}

func extractSeatIDs(seats []orderevent.SeatSnapshot) []int64 {
	if len(seats) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(seats))
	for _, seat := range seats {
		if seat.SeatID <= 0 {
			continue
		}
		ids = append(ids, seat.SeatID)
	}

	return ids
}

func validateOrderCreateEvent(orderEvent *orderevent.OrderCreateEvent) error {
	if orderEvent == nil {
		return xerr.ErrInternal
	}
	if orderEvent.OrderNumber <= 0 || orderEvent.UserID <= 0 || orderEvent.ProgramID <= 0 || orderEvent.TicketCategoryID <= 0 {
		return xerr.ErrInternal
	}
	if orderEvent.OccurredAt == "" {
		return xerr.ErrInternal
	}
	ticketCount := orderEvent.TicketCount
	if ticketCount <= 0 {
		switch {
		case len(orderEvent.TicketUserIDs) > 0:
			ticketCount = int64(len(orderEvent.TicketUserIDs))
		case hasEmbeddedOrderCreateSnapshots(orderEvent):
			ticketCount = int64(len(orderEvent.TicketUserSnapshot))
		default:
			return xerr.ErrInternal
		}
	}
	if len(orderEvent.TicketUserIDs) > 0 && int64(len(orderEvent.TicketUserIDs)) != ticketCount {
		return xerr.ErrInternal
	}
	if hasEmbeddedOrderCreateSnapshots(orderEvent) && int64(len(orderEvent.TicketUserSnapshot)) != ticketCount {
		return xerr.ErrInternal
	}

	return nil
}

func isDuplicateOrderNumberErr(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func (l *CreateOrderConsumerLogic) prepareAttemptForConsume(showTimeID, orderNumber int64, now time.Time) (*rush.AttemptRecord, int64, bool, error) {
	if l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return nil, 0, false, xerr.ErrInternal
	}

	return l.svcCtx.AttemptStore.PrepareAttemptForConsume(l.ctx, showTimeID, orderNumber, now)
}

func (l *CreateOrderConsumerLogic) buildConsumerOrderEvent(orderEvent *orderevent.OrderCreateEvent, attempt *rush.AttemptRecord, processingEpoch int64, occurredAt time.Time) (*orderevent.OrderCreateEvent, *programrpc.AutoAssignAndFreezeSeatsResp, error) {
	if hasEmbeddedOrderCreateSnapshots(orderEvent) {
		return orderEvent, nil, nil
	}

	showTimeID := orderEvent.ShowTimeID
	if showTimeID <= 0 {
		showTimeID = orderEvent.ProgramID
	}

	preorder, err := l.svcCtx.ProgramRpc.GetProgramPreorder(l.ctx, &programrpc.GetProgramPreorderReq{
		ShowTimeId: showTimeID,
	})
	if err != nil {
		return nil, nil, err
	}
	userResp, err := l.svcCtx.UserRpc.GetUserAndTicketUserList(l.ctx, &userrpc.GetUserAndTicketUserListReq{
		UserId: orderEvent.UserID,
	})
	if err != nil {
		return nil, nil, err
	}
	freezeReq := &programrpc.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       showTimeID,
		TicketCategoryId: orderEvent.TicketCategoryID,
		Count:            orderEvent.TicketCount,
		FreezeToken:      buildSeatFreezeToken(showTimeID, orderEvent.TicketCategoryID, orderEvent.OrderNumber, processingEpoch),
		FreezeSeconds:    durationToFreezeSeconds(l.svcCtx.Config.Order.CloseAfter),
	}
	if attempt == nil || processingEpoch <= 0 {
		freezeReq.FreezeToken = buildSeatFreezeToken(showTimeID, orderEvent.TicketCategoryID, orderEvent.OrderNumber, 1)
	}
	freezeResp, err := l.freezeSeatsWithRetry(freezeReq)
	if err != nil {
		return nil, nil, &seatFreezeError{err: err}
	}

	event, err := buildOrderCreateEventFromSnapshots(
		orderEvent.OrderNumber,
		orderEvent.UserID,
		orderEvent.ProgramID,
		orderEvent.TicketCategoryID,
		append([]int64(nil), orderEvent.TicketUserIDs...),
		orderEvent.DistributionMode,
		orderEvent.TakeTicketMode,
		preorder,
		userResp,
		freezeResp,
		occurredAt,
		l.svcCtx.Config.Order.CloseAfter,
	)
	if err != nil {
		return nil, freezeResp, err
	}

	event.EventID = orderEvent.EventID
	event.Version = orderEvent.Version
	if orderEvent.RequestNo != "" {
		event.RequestNo = orderEvent.RequestNo
	}
	event.ShowTimeID = orderEvent.ShowTimeID
	event.SaleWindowEndAt = orderEvent.SaleWindowEndAt
	event.ShowEndAt = orderEvent.ShowEndAt

	return event, freezeResp, nil
}

type seatFreezeError struct {
	err error
}

func (e *seatFreezeError) Error() string {
	return e.err.Error()
}

func (e *seatFreezeError) Unwrap() error {
	return e.err
}

func durationToFreezeSeconds(value time.Duration) int64 {
	if value <= 0 {
		value = 15 * time.Minute
	}

	seconds := value / time.Second
	if value%time.Second != 0 {
		seconds++
	}
	if seconds <= 0 {
		return 1
	}

	return int64(seconds)
}

func buildSeatFreezeToken(showTimeID, ticketCategoryID, orderNumber, processingEpoch int64) string {
	if processingEpoch <= 0 {
		processingEpoch = 1
	}

	return seatfreeze.FormatToken(showTimeID, ticketCategoryID, orderNumber, processingEpoch)
}

func isTerminalSeatFreezeError(err error) bool {
	if err == nil {
		return false
	}

	code := status.Code(err)
	switch code {
	case codes.FailedPrecondition, codes.ResourceExhausted:
		return status.Convert(err).Message() == xerr.ErrSeatInventoryInsufficient.Error()
	default:
		return false
	}
}

func hasEmbeddedOrderCreateSnapshots(orderEvent *orderevent.OrderCreateEvent) bool {
	if orderEvent == nil {
		return false
	}
	if orderEvent.FreezeToken == "" || orderEvent.FreezeExpireTime == "" {
		return false
	}
	if len(orderEvent.TicketUserSnapshot) == 0 || len(orderEvent.SeatSnapshot) == 0 {
		return false
	}
	return len(orderEvent.TicketUserSnapshot) == len(orderEvent.SeatSnapshot)
}

func isGuardConflictErr(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return false
	}

	message := mysqlErr.Message
	if strings.Contains(message, "uk_show_time_user") {
		return true
	}
	if strings.Contains(message, "uk_show_time_viewer") {
		return true
	}
	if strings.Contains(message, "uk_show_time_seat") {
		return true
	}

	return false
}

func processingLeaseInterval(svcCtx *svc.ServiceContext) time.Duration {
	if svcCtx == nil {
		return 100 * time.Millisecond
	}

	ttl := svcCtx.Config.RushOrder.InFlightTTL
	if ttl <= 0 {
		return 100 * time.Millisecond
	}

	interval := ttl / 3
	if interval < 100*time.Millisecond {
		return 100 * time.Millisecond
	}

	return interval
}

func (l *CreateOrderConsumerLogic) freezeSeatsWithRetry(req *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
	resp, err := l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(l.ctx, req)
	if !isSeatFreezeTimeout(err) {
		return resp, err
	}

	return l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(l.ctx, req)
}

func isSeatFreezeTimeout(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return status.Code(err) == codes.DeadlineExceeded
}

func (l *CreateOrderConsumerLogic) finalizeSuccess(attempt *rush.AttemptRecord, processingEpoch int64, seatIDs []int64, now time.Time, lease *processingLease) {
	if attempt == nil || processingEpoch <= 0 || l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return
	}
	if lease != nil && lease.lost.Load() {
		return
	}
	if err := l.svcCtx.AttemptStore.FinalizeSuccess(l.ctx, attempt, processingEpoch, seatIDs, now); err != nil {
		l.Errorf("finalize rush attempt success failed, orderNumber=%d epoch=%d err=%v", attempt.OrderNumber, processingEpoch, err)
	}
}

func (l *CreateOrderConsumerLogic) finalizeFailure(attempt *rush.AttemptRecord, processingEpoch int64, reason, freezeToken, releaseReason string) error {
	if attempt == nil || processingEpoch <= 0 || l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return nil
	}

	outcome, err := l.svcCtx.AttemptStore.FinalizeFailure(l.ctx, attempt, processingEpoch, reason, time.Now())
	if err != nil {
		latest, getErr := l.svcCtx.AttemptStore.Get(l.ctx, attempt.OrderNumber)
		if getErr == nil && shouldRetryFinalizeFailure(attempt, latest, err) {
			return err
		}
		if getErr != nil && !errors.Is(getErr, xerr.ErrOrderNotFound) {
			return err
		}
		return err
	}

	releaseFreeze, outcomeErr := handleFinalizeFailureOutcome(outcome, nil)
	if outcomeErr != nil {
		return outcomeErr
	}
	if releaseFreeze && freezeToken != "" {
		releaseOrderCreateFreeze(l.ctx, l.svcCtx, freezeToken, releaseReason)
	}

	return nil
}

func (l *CreateOrderConsumerLogic) resolveOrderPersistFailure(
	orderNumber int64,
	attempt *rush.AttemptRecord,
	processingEpoch int64,
	seatIDs []int64,
	freezeResp *programrpc.AutoAssignAndFreezeSeatsResp,
	persistErr error,
	now time.Time,
	lease *processingLease,
) error {
	if lease != nil && lease.lost.Load() {
		return nil
	}

	if order, err := l.svcCtx.OrderRepository.FindOrderByNumber(l.ctx, orderNumber); err == nil && order != nil {
		l.finalizeSuccess(attempt, processingEpoch, seatIDs, now, lease)
		return nil
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		return persistErr
	}

	freezeToken := ""
	if freezeResp != nil {
		freezeToken = freezeResp.GetFreezeToken()
	}
	if err := l.finalizeFailure(attempt, processingEpoch, orderCreatePersistFailureReason, freezeToken, orderCreatePersistFailureReason); err != nil {
		return err
	}

	return nil
}
