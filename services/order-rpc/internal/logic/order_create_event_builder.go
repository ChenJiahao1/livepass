package logic

import (
	"fmt"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xid"
	orderevent "livepass/services/order-rpc/internal/event"
	"livepass/services/order-rpc/internal/rush"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"
)

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
		DistributionMode: claims.DistributionMode,
		TakeTicketMode:   claims.TakeTicketMode,
		SaleWindowEndAt:  formatOrderTime(time.Unix(claims.SaleWindowEndAt, 0)),
		ShowEndAt:        formatOrderTime(time.Unix(claims.ShowEndAt, 0)),
	}, nil
}
