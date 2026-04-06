package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"damai-go/pkg/xerr"
	orderevent "damai-go/services/order-rpc/internal/event"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/repository"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
	attempt, shouldProcess, err := l.prepareAttemptForConsume(orderEvent.OrderNumber, now)
	if err != nil {
		if !errors.Is(err, xerr.ErrOrderNotFound) || !hasEmbeddedOrderCreateSnapshots(orderEvent) {
			return err
		}
		shouldProcess = true
	}

	if existing, err := l.svcCtx.OrderRepository.FindOrderByNumber(l.ctx, orderEvent.OrderNumber); err == nil && existing != nil {
		if attempt != nil && l.svcCtx.AttemptStore != nil {
			if err := l.svcCtx.AttemptStore.CommitProjection(l.ctx, attempt, now); err != nil {
				l.Errorf("commit rush attempt projection failed after duplicate consume, orderNumber=%d err=%v", orderEvent.OrderNumber, err)
			}
		}
		return nil
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}
	if !shouldProcess {
		return nil
	}
	if maxDelay := l.svcCtx.Config.Kafka.MaxMessageDelay; maxDelay > 0 && now.Sub(occurredAt) > maxDelay {
		if attempt != nil {
			if err := l.releaseAttemptProjection(attempt, rush.AttemptReasonCommitCutoffExceed, "", ""); err != nil {
				l.Errorf("release expired rush attempt failed, orderNumber=%d err=%v", orderEvent.OrderNumber, err)
			}
		} else {
			compensateOrderCreateExpired(l.ctx, l.svcCtx, orderEvent.UserID, orderEvent.ProgramID, orderEvent.OrderNumber, orderEvent.FreezeToken)
		}
		l.Infof("skip expired rush order create event, orderNumber=%d occurredAt=%s", orderEvent.OrderNumber, orderEvent.OccurredAt)
		return nil
	}
	if expired, err := isCommitCutoffExceeded(orderEvent.CommitCutoffAt, now); err != nil {
		return err
	} else if expired {
		if err := l.releaseAttemptProjection(attempt, rush.AttemptReasonCommitCutoffExceed, "", ""); err != nil {
			l.Errorf("release commit-cutoff rush attempt failed, orderNumber=%d err=%v", orderEvent.OrderNumber, err)
		}
		l.Infof("skip commit-cutoff-exceeded rush order create event, orderNumber=%d cutoffAt=%s", orderEvent.OrderNumber, orderEvent.CommitCutoffAt)
		return nil
	}

	enrichedEvent, freezeResp, err := l.buildConsumerOrderEvent(orderEvent, attempt, occurredAt)
	if err != nil {
		var freezeErr *seatFreezeError
		if errors.As(err, &freezeErr) && isTerminalSeatFreezeError(freezeErr.err) {
			if releaseErr := l.releaseAttemptProjection(attempt, rush.AttemptReasonSeatExhausted, "", ""); releaseErr != nil {
				return releaseErr
			}
			return nil
		}
		return err
	}

	writeModels, err := mapEventToOrderWriteModels(enrichedEvent, now)
	if err != nil {
		freezeToken := ""
		if freezeResp != nil {
			freezeToken = freezeResp.GetFreezeToken()
		}
		_ = l.releaseAttemptProjection(attempt, rush.AttemptReasonCommitCutoffExceed, freezeToken, orderCreateSendFailedReleaseReason)
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
		if err := tx.InsertOutbox(ctx, writeModels.outboxRows); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if attempt != nil && l.svcCtx.AttemptStore != nil {
		if err := l.svcCtx.AttemptStore.CommitProjection(l.ctx, attempt, now); err != nil {
			return err
		}
	}
	if l.svcCtx.AsyncCloseClient != nil {
		if err := l.svcCtx.AsyncCloseClient.EnqueueCloseTimeout(l.ctx, orderEvent.OrderNumber, writeModels.order.OrderExpireTime); err != nil {
			l.Errorf("enqueue order async close failed, orderNumber=%d err=%v", orderEvent.OrderNumber, err)
		}
	}

	return nil
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

func (l *CreateOrderConsumerLogic) prepareAttemptForConsume(orderNumber int64, now time.Time) (*rush.AttemptRecord, bool, error) {
	if l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return nil, false, xerr.ErrInternal
	}

	record, err := l.svcCtx.AttemptStore.Get(l.ctx, orderNumber)
	if err != nil {
		return nil, false, err
	}
	switch record.State {
	case rush.AttemptStateCommitted, rush.AttemptStateReleased:
		return record, false, nil
	case rush.AttemptStateProcessing:
		return record, true, nil
	case rush.AttemptStatePendingPublish, rush.AttemptStateQueued:
		if _, _, err := l.svcCtx.AttemptStore.ClaimProcessing(l.ctx, orderNumber, now); err != nil && !errors.Is(err, xerr.ErrOrderNotFound) {
			return nil, false, err
		}
		record, err = l.svcCtx.AttemptStore.Get(l.ctx, orderNumber)
		if err != nil {
			return nil, false, err
		}
		return record, true, nil
	default:
		return record, false, fmt.Errorf("unexpected attempt state %s", record.State)
	}
}

func (l *CreateOrderConsumerLogic) buildConsumerOrderEvent(orderEvent *orderevent.OrderCreateEvent, attempt *rush.AttemptRecord, occurredAt time.Time) (*orderevent.OrderCreateEvent, *programrpc.AutoAssignAndFreezeSeatsResp, error) {
	if hasEmbeddedOrderCreateSnapshots(orderEvent) {
		return orderEvent, nil, nil
	}

	preorder, err := l.svcCtx.ProgramRpc.GetProgramPreorder(l.ctx, &programrpc.GetProgramDetailReq{
		Id: orderEvent.ProgramID,
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
		ProgramId:        orderEvent.ProgramID,
		TicketCategoryId: orderEvent.TicketCategoryID,
		Count:            orderEvent.TicketCount,
		RequestNo:        orderEvent.RequestNo,
		FreezeSeconds:    durationToFreezeSeconds(l.svcCtx.Config.Order.CloseAfter),
	}
	if attempt != nil {
		freezeReq.OwnerOrderNumber = attempt.OrderNumber
		freezeReq.OwnerEpoch = attempt.ProcessingEpoch
		if attempt.ProcessingEpoch > 0 {
			freezeReq.RequestNo = fmt.Sprintf("%d-%d", attempt.OrderNumber, attempt.ProcessingEpoch)
		}
	}
	if freezeReq.RequestNo == "" {
		freezeReq.RequestNo = fmt.Sprintf("order-create-%d", orderEvent.OrderNumber)
	}
	freezeResp, err := l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(l.ctx, freezeReq)
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
	event.CommitCutoffAt = orderEvent.CommitCutoffAt
	event.UserDeadlineAt = orderEvent.UserDeadlineAt

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

func (l *CreateOrderConsumerLogic) releaseAttemptProjection(attempt *rush.AttemptRecord, reason, freezeToken, releaseReason string) error {
	if attempt == nil || l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return nil
	}
	if err := l.svcCtx.AttemptStore.Release(l.ctx, attempt, reason, time.Now()); err != nil {
		return err
	}
	if freezeToken != "" {
		releaseOrderCreateFreezeWithOwner(l.ctx, l.svcCtx, freezeToken, releaseReason, attempt.OrderNumber, attempt.ProcessingEpoch)
	}

	return nil
}

func isCommitCutoffExceeded(cutoffAt string, now time.Time) (bool, error) {
	if cutoffAt == "" {
		return false, nil
	}

	parsed, err := parseOrderTime(cutoffAt)
	if err != nil {
		return false, err
	}

	return !now.Before(parsed), nil
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
