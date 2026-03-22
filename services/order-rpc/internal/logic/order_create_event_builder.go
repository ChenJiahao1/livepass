package logic

import (
	"fmt"
	"time"

	"damai-go/pkg/xerr"
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
	if preorder == nil || userResp == nil || freezeResp == nil {
		return nil, xerr.ErrInternal
	}
	if orderNumber <= 0 {
		return nil, xerr.ErrInternal
	}

	ticketCategory, ok := findPreorderTicketCategory(preorder.GetTicketCategoryVoList(), in.GetTicketCategoryId())
	if !ok {
		return nil, xerr.ErrProgramTicketCategoryNotFound
	}
	if len(freezeResp.GetSeats()) != len(in.GetTicketUserIds()) {
		return nil, xerr.ErrSeatInventoryInsufficient
	}

	ticketUsers := make(map[int64]*userrpc.TicketUserInfo, len(userResp.GetTicketUserVoList()))
	for _, ticketUser := range userResp.GetTicketUserVoList() {
		if ticketUser == nil {
			continue
		}
		ticketUsers[ticketUser.GetId()] = ticketUser
	}
	freezeExpireTime := freezeResp.GetExpireTime()
	if freezeExpireTime == "" {
		freezeExpireTime = formatOrderTime(now.Add(closeAfter))
	}

	orderEvent := &orderevent.OrderCreateEvent{
		EventID:          fmt.Sprintf("%d", xid.New()),
		Version:          orderevent.OrderCreateEventVersion,
		OrderNumber:      orderNumber,
		RequestNo:        fmt.Sprintf("order-create-%d", orderNumber),
		OccurredAt:       formatOrderTime(now),
		UserID:           in.GetUserId(),
		ProgramID:        in.GetProgramId(),
		TicketCategoryID: in.GetTicketCategoryId(),
		TicketUserIDs:    append([]int64(nil), in.GetTicketUserIds()...),
		DistributionMode: in.GetDistributionMode(),
		TakeTicketMode:   in.GetTakeTicketMode(),
		FreezeToken:      freezeResp.GetFreezeToken(),
		FreezeExpireTime: freezeExpireTime,
		ProgramSnapshot: orderevent.ProgramSnapshot{
			Title:            preorder.GetTitle(),
			ItemPicture:      preorder.GetItemPicture(),
			Place:            preorder.GetPlace(),
			ShowTime:         preorder.GetShowTime(),
			PermitChooseSeat: preorder.GetPermitChooseSeat(),
		},
		TicketCategorySnapshot: orderevent.TicketCategorySnapshot{
			ID:    ticketCategory.GetId(),
			Name:  ticketCategory.GetIntroduce(),
			Price: ticketCategory.GetPrice(),
		},
		TicketUserSnapshot: make([]orderevent.TicketUserSnapshot, 0, len(in.GetTicketUserIds())),
		SeatSnapshot:       make([]orderevent.SeatSnapshot, 0, len(freezeResp.GetSeats())),
	}

	for _, ticketUserID := range in.GetTicketUserIds() {
		ticketUser, ok := ticketUsers[ticketUserID]
		if !ok || ticketUser.GetUserId() != in.GetUserId() {
			return nil, xerr.ErrOrderTicketUserInvalid
		}

		orderEvent.TicketUserSnapshot = append(orderEvent.TicketUserSnapshot, orderevent.TicketUserSnapshot{
			TicketUserID: ticketUser.GetId(),
			Name:         ticketUser.GetRelName(),
			IDNumber:     ticketUser.GetIdNumber(),
		})
	}

	for _, seat := range freezeResp.GetSeats() {
		if seat == nil {
			return nil, xerr.ErrInternal
		}

		orderEvent.SeatSnapshot = append(orderEvent.SeatSnapshot, orderevent.SeatSnapshot{
			SeatID:  seat.GetSeatId(),
			RowCode: seat.GetRowCode(),
			ColCode: seat.GetColCode(),
			Price:   seat.GetPrice(),
		})
	}

	return orderEvent, nil
}
