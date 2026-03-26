package logic

import (
	"database/sql"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	orderevent "damai-go/services/order-rpc/internal/event"
	"damai-go/services/order-rpc/internal/model"
)

type orderWriteModels struct {
	order        *model.DOrder
	orderTickets []*model.DOrderTicketUser
}

func MapEventToOrderModels(orderEvent *orderevent.OrderCreateEvent, now time.Time) (*model.DOrder, []*model.DOrderTicketUser, error) {
	return mapEventToOrderModels(orderEvent, now)
}

func mapEventToOrderModels(orderEvent *orderevent.OrderCreateEvent, now time.Time) (*model.DOrder, []*model.DOrderTicketUser, error) {
	writeModels, err := mapEventToOrderWriteModels(orderEvent, now)
	if err != nil {
		return nil, nil, err
	}

	return writeModels.order, writeModels.orderTickets, nil
}

func mapEventToOrderWriteModels(orderEvent *orderevent.OrderCreateEvent, now time.Time) (*orderWriteModels, error) {
	if orderEvent == nil {
		return nil, xerr.ErrInternal
	}
	if len(orderEvent.TicketUserSnapshot) == 0 || len(orderEvent.TicketUserSnapshot) != len(orderEvent.SeatSnapshot) {
		return nil, xerr.ErrInternal
	}

	showTime, err := parseOrderTime(orderEvent.ProgramSnapshot.ShowTime)
	if err != nil {
		return nil, err
	}
	orderExpireTime, err := parseOrderTime(orderEvent.FreezeExpireTime)
	if err != nil {
		return nil, err
	}
	createOrderTime, err := parseOrderTime(orderEvent.OccurredAt)
	if err != nil {
		return nil, err
	}

	order := &model.DOrder{
		Id:                      xid.New(),
		OrderNumber:             orderEvent.OrderNumber,
		ProgramId:               orderEvent.ProgramID,
		ProgramTitle:            orderEvent.ProgramSnapshot.Title,
		ProgramItemPicture:      orderEvent.ProgramSnapshot.ItemPicture,
		ProgramPlace:            orderEvent.ProgramSnapshot.Place,
		ProgramShowTime:         showTime,
		ProgramPermitChooseSeat: orderEvent.ProgramSnapshot.PermitChooseSeat,
		UserId:                  orderEvent.UserID,
		DistributionMode:        orderEvent.DistributionMode,
		TakeTicketMode:          orderEvent.TakeTicketMode,
		TicketCount:             int64(len(orderEvent.TicketUserSnapshot)),
		OrderPrice:              float64(orderEvent.TicketCategorySnapshot.Price * int64(len(orderEvent.TicketUserSnapshot))),
		OrderStatus:             orderStatusUnpaid,
		FreezeToken:             orderEvent.FreezeToken,
		OrderExpireTime:         orderExpireTime,
		CreateOrderTime:         createOrderTime,
		CancelOrderTime:         sql.NullTime{},
		PayOrderTime:            sql.NullTime{},
		CreateTime:              now,
		EditTime:                now,
		Status:                  1,
	}

	orderTickets := make([]*model.DOrderTicketUser, 0, len(orderEvent.TicketUserSnapshot))
	for idx, ticketUser := range orderEvent.TicketUserSnapshot {
		seat := orderEvent.SeatSnapshot[idx]
		orderTickets = append(orderTickets, &model.DOrderTicketUser{
			Id:                 xid.New(),
			OrderNumber:        orderEvent.OrderNumber,
			UserId:             orderEvent.UserID,
			TicketUserId:       ticketUser.TicketUserID,
			TicketUserName:     ticketUser.Name,
			TicketUserIdNumber: ticketUser.IDNumber,
			TicketCategoryId:   orderEvent.TicketCategorySnapshot.ID,
			TicketCategoryName: orderEvent.TicketCategorySnapshot.Name,
			TicketPrice:        float64(orderEvent.TicketCategorySnapshot.Price),
			SeatId:             seat.SeatID,
			SeatRow:            seat.RowCode,
			SeatCol:            seat.ColCode,
			SeatPrice:          float64(seat.Price),
			OrderStatus:        orderStatusUnpaid,
			CreateOrderTime:    createOrderTime,
			CreateTime:         now,
			EditTime:           now,
			Status:             1,
		})
	}

	return &orderWriteModels{
		order:        order,
		orderTickets: orderTickets,
	}, nil
}
