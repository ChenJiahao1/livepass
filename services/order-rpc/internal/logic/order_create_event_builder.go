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

	return nil, status.Error(codes.Unimplemented, "build order create event is not implemented under token-only create-order contract")
}

func buildOrderCreateEventFromSnapshots(
	orderNumber, userID, programID, ticketCategoryID int64,
	ticketUserIDs []int64,
	distributionMode, takeTicketMode string,
	preorder *programrpc.ProgramPreorderInfo,
	userResp *userrpc.GetUserAndTicketUserListResp,
	freezeResp *programrpc.AutoAssignAndFreezeSeatsResp,
	now time.Time,
	closeAfter time.Duration,
) (*orderevent.OrderCreateEvent, error) {
	if orderNumber <= 0 || userID <= 0 || programID <= 0 || ticketCategoryID <= 0 || preorder == nil || userResp == nil || freezeResp == nil {
		return nil, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := validateRequestedTicketUsers(userResp, userID, ticketUserIDs); err != nil {
		return nil, err
	}
	if len(ticketUserIDs) == 0 || len(freezeResp.GetSeats()) != len(ticketUserIDs) {
		return nil, xerr.ErrInvalidParam
	}

	ticketCategory, ok := findPreorderTicketCategory(preorder.GetTicketCategoryVoList(), ticketCategoryID)
	if !ok || ticketCategory == nil {
		return nil, xerr.ErrProgramTicketCategoryNotFound
	}

	ticketUsers := make(map[int64]*userrpc.TicketUserInfo, len(userResp.GetTicketUserVoList()))
	for _, ticketUser := range userResp.GetTicketUserVoList() {
		if ticketUser == nil {
			continue
		}
		ticketUsers[ticketUser.GetId()] = ticketUser
	}

	ticketUserSnapshot := make([]orderevent.TicketUserSnapshot, 0, len(ticketUserIDs))
	for _, ticketUserID := range ticketUserIDs {
		ticketUser, ok := ticketUsers[ticketUserID]
		if !ok || ticketUser == nil {
			return nil, xerr.ErrOrderTicketUserInvalid
		}
		ticketUserSnapshot = append(ticketUserSnapshot, orderevent.TicketUserSnapshot{
			TicketUserID: ticketUser.GetId(),
			Name:         ticketUser.GetRelName(),
			IDNumber:     ticketUser.GetIdNumber(),
		})
	}

	seatSnapshot := make([]orderevent.SeatSnapshot, 0, len(freezeResp.GetSeats()))
	for _, seat := range freezeResp.GetSeats() {
		if seat == nil {
			return nil, xerr.ErrInvalidParam
		}
		seatSnapshot = append(seatSnapshot, orderevent.SeatSnapshot{
			SeatID:  seat.GetSeatId(),
			RowCode: seat.GetRowCode(),
			ColCode: seat.GetColCode(),
			Price:   seat.GetPrice(),
		})
	}

	freezeExpireTime := freezeResp.GetExpireTime()
	if freezeExpireTime == "" {
		if closeAfter <= 0 {
			closeAfter = 15 * time.Minute
		}
		freezeExpireTime = formatOrderTime(now.Add(closeAfter))
	}

	return &orderevent.OrderCreateEvent{
		EventID:          fmt.Sprintf("%d", xid.New()),
		Version:          orderevent.OrderCreateEventVersion,
		OrderNumber:      orderNumber,
		RequestNo:        fmt.Sprintf("order-create-%d", orderNumber),
		OccurredAt:       formatOrderTime(now),
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       preorder.GetShowTimeId(),
		TicketCategoryID: ticketCategoryID,
		TicketUserIDs:    append([]int64(nil), ticketUserIDs...),
		TicketCount:      int64(len(ticketUserIDs)),
		DistributionMode: distributionMode,
		TakeTicketMode:   takeTicketMode,
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
		TicketUserSnapshot: ticketUserSnapshot,
		SeatSnapshot:       seatSnapshot,
	}, nil
}

func buildAttemptCreateEvent(orderNumber int64, claims *rush.PurchaseTokenClaims, occurredAt time.Time) (*orderevent.OrderCreateEvent, error) {
	if claims == nil {
		return nil, xerr.ErrInternal
	}
	if orderNumber <= 0 || claims.OrderNumber <= 0 || orderNumber != claims.OrderNumber {
		return nil, xerr.ErrInvalidParam
	}
	if claims.UserID <= 0 || claims.ProgramID <= 0 || claims.ShowTimeID <= 0 || claims.TicketCategoryID <= 0 {
		return nil, xerr.ErrInvalidParam
	}
	if claims.TicketCount <= 0 {
		claims.TicketCount = int64(len(claims.TicketUserIDs))
	}
	if claims.TicketCount <= 0 || int64(len(claims.TicketUserIDs)) != claims.TicketCount {
		return nil, xerr.ErrInvalidParam
	}
	generation := claims.Generation
	if generation == "" {
		generation = rush.BuildRushGeneration(claims.ShowTimeID)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	return &orderevent.OrderCreateEvent{
		EventID:          fmt.Sprintf("%d", xid.New()),
		Version:          orderevent.OrderCreateEventVersion,
		OrderNumber:      orderNumber,
		RequestNo:        fmt.Sprintf("order-create-%d", orderNumber),
		OccurredAt:       formatOrderTime(occurredAt),
		UserID:           claims.UserID,
		ProgramID:        claims.ProgramID,
		ShowTimeID:       claims.ShowTimeID,
		TicketCategoryID: claims.TicketCategoryID,
		TicketUserIDs:    append([]int64(nil), claims.TicketUserIDs...),
		TicketCount:      claims.TicketCount,
		Generation:       generation,
		DistributionMode: claims.DistributionMode,
		TakeTicketMode:   claims.TakeTicketMode,
		SaleWindowEndAt:  formatOrderTime(time.Unix(claims.SaleWindowEndAt, 0)),
		ShowEndAt:        formatOrderTime(time.Unix(claims.ShowEndAt, 0)),
	}, nil
}
