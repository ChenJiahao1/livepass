package logic

import (
	"fmt"
	"time"

	"damai-go/pkg/xid"
	orderevent "damai-go/services/order-rpc/internal/event"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"
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
	orderEvent := &orderevent.OrderCreateEvent{
		EventID:     fmt.Sprintf("%d", xid.New()),
		Version:     orderevent.OrderCreateEventVersion,
		OrderNumber: orderNumber,
		RequestNo:   fmt.Sprintf("order-create-%d", orderNumber),
		OccurredAt:  formatOrderTime(now),
		UserID:      in.GetUserId(),
	}

	return orderEvent, nil
}
