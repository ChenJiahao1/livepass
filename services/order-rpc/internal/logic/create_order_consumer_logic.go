package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	orderevent "damai-go/services/order-rpc/internal/event"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

	occurredAt, err := parseOrderTime(orderEvent.OccurredAt)
	if err != nil {
		return err
	}
	if maxDelay := l.svcCtx.Config.Kafka.MaxMessageDelay; maxDelay > 0 && time.Since(occurredAt) > maxDelay {
		compensateOrderCreateExpired(l.ctx, l.svcCtx, orderEvent.UserID, orderEvent.ProgramID, orderEvent.OrderNumber, orderEvent.FreezeToken)
		l.Infof("skip expired order create event, orderNumber=%d occurredAt=%s", orderEvent.OrderNumber, orderEvent.OccurredAt)
		return nil
	}

	if existing, err := l.svcCtx.DOrderModel.FindOneByOrderNumber(l.ctx, orderEvent.OrderNumber); err == nil && existing != nil {
		return nil
	} else if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}

	order, orderTickets, err := mapEventToOrderModels(orderEvent, time.Now())
	if err != nil {
		return err
	}

	err = l.svcCtx.SqlConn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := l.svcCtx.DOrderModel.InsertWithSession(ctx, session, order); err != nil {
			if isDuplicateOrderNumberErr(err) {
				return nil
			}
			return err
		}
		if err := l.svcCtx.DOrderTicketUserModel.InsertBatch(ctx, session, orderTickets); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
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
	if orderEvent.FreezeToken == "" || orderEvent.OccurredAt == "" || orderEvent.FreezeExpireTime == "" {
		return xerr.ErrInternal
	}
	if len(orderEvent.TicketUserSnapshot) == 0 || len(orderEvent.SeatSnapshot) == 0 {
		return xerr.ErrInternal
	}
	if len(orderEvent.TicketUserSnapshot) != len(orderEvent.SeatSnapshot) {
		return xerr.ErrInternal
	}

	return nil
}

func isDuplicateOrderNumberErr(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
