package logic

import (
	"fmt"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	orderevent "damai-go/services/order-rpc/internal/event"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func BuildOrderCreateEvent(
	orderNumber int64,
	in *pb.CreateOrderReq,
	preorder *programrpc.ProgramPreorderInfo,
	userResp *userrpc.GetUserAndTicketUserListResp,
	freezeResp *programrpc.AutoAssignAndFreezeSeatsResp,
	now time.Time,
	closeAfter time.Duration,
) (*orderevent.OrderCreateEvent, error) {
	return buildOrderCreateEvent(orderNumber, in, preorder, userResp, freezeResp, now, closeAfter)
}

func buildOrderCreateEvent(
	orderNumber int64,
	in *pb.CreateOrderReq,
	preorder *programrpc.ProgramPreorderInfo,
	userResp *userrpc.GetUserAndTicketUserListResp,
	freezeResp *programrpc.AutoAssignAndFreezeSeatsResp,
	now time.Time,
	closeAfter time.Duration,
) (*orderevent.OrderCreateEvent, error) {
	_ = orderNumber
	_ = in
	_ = preorder
	_ = userResp
	_ = freezeResp
	_ = now
	_ = closeAfter

	return nil, status.Error(codes.Unimplemented, "build order create event is not implemented under task1 rush contract")
}

func buildAttemptCreateEvent(attempt *rush.AttemptRecord, claims *rush.PurchaseTokenClaims) (*orderevent.OrderCreateEvent, error) {
	if attempt == nil || claims == nil {
		return nil, xerr.ErrInternal
	}
	if attempt.OrderNumber <= 0 || claims.OrderNumber <= 0 || attempt.OrderNumber != claims.OrderNumber {
		return nil, xerr.ErrInvalidParam
	}
	if attempt.UserID != claims.UserID || attempt.ProgramID != claims.ProgramID || attempt.TicketCategoryID != claims.TicketCategoryID {
		return nil, xerr.ErrInvalidParam
	}
	if attempt.TicketCount <= 0 || attempt.TicketCount != claims.TicketCount {
		return nil, xerr.ErrInvalidParam
	}
	if len(claims.TicketUserIDs) == 0 || int64(len(claims.TicketUserIDs)) != claims.TicketCount {
		return nil, xerr.ErrInvalidParam
	}

	occurredAt := attempt.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	return &orderevent.OrderCreateEvent{
		EventID:          fmt.Sprintf("%d", xid.New()),
		Version:          orderevent.OrderCreateEventVersion,
		OrderNumber:      attempt.OrderNumber,
		RequestNo:        fmt.Sprintf("order-create-%d", attempt.OrderNumber),
		OccurredAt:       formatOrderTime(occurredAt),
		UserID:           attempt.UserID,
		ProgramID:        attempt.ProgramID,
		TicketCategoryID: attempt.TicketCategoryID,
		TicketUserIDs:    append([]int64(nil), claims.TicketUserIDs...),
		TicketCount:      attempt.TicketCount,
		DistributionMode: claims.DistributionMode,
		TakeTicketMode:   claims.TakeTicketMode,
		CommitCutoffAt:   formatOrderTime(attempt.CommitCutoffAt),
		UserDeadlineAt:   formatOrderTime(attempt.UserDeadlineAt),
	}, nil
}
