package logic

import (
	"context"

	"damai-go/jobs/order-migrate/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyOrdersResp struct {
	VerifiedSlots int64
}

type VerifyOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyOrdersLogic {
	return &VerifyOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyOrdersLogic) VerifyOrders() (*VerifyOrdersResp, error) {
	resp := &VerifyOrdersResp{}
	for _, logicSlot := range l.svcCtx.Config.Verify.Slots {
		route, err := l.svcCtx.RouteMap.RouteByLogicSlot(logicSlot)
		if err != nil {
			return nil, err
		}

		legacyOrders, err := listOrdersByTable(l.ctx, l.svcCtx.LegacySqlConn, "d_order")
		if err != nil {
			return nil, err
		}
		legacyTickets, err := listTicketsByTable(l.ctx, l.svcCtx.LegacySqlConn, "d_order_ticket_user")
		if err != nil {
			return nil, err
		}
		legacyIndexes, err := listUserOrderIndexesByTable(l.ctx, l.svcCtx.LegacySqlConn, "d_user_order_index")
		if err != nil {
			return nil, err
		}
		shardConn := l.svcCtx.ShardSqlConns[route.DBKey]
		shardOrders, err := listOrdersByTable(l.ctx, shardConn, "d_order_"+route.TableSuffix)
		if err != nil {
			return nil, err
		}
		shardTickets, err := listTicketsByTable(l.ctx, shardConn, "d_order_ticket_user_"+route.TableSuffix)
		if err != nil {
			return nil, err
		}
		shardIndexes, err := listUserOrderIndexesByTable(l.ctx, shardConn, "d_user_order_index_"+route.TableSuffix)
		if err != nil {
			return nil, err
		}

		legacyScoped := filterOrdersBySlot(legacyOrders, logicSlot)
		shardScoped := filterOrdersBySlot(shardOrders, logicSlot)
		orderNumbers := collectOrderNumbers(legacyScoped, shardScoped)
		if err := compareAggregates(logicSlot, buildVerifyAggregate(legacyScoped), buildVerifyAggregate(shardScoped)); err != nil {
			return nil, err
		}
		if err := compareOrderSamples(l.svcCtx.Config.Verify.SampleSize, legacyScoped, shardScoped); err != nil {
			return nil, err
		}
		if err := compareUserListSamples(l.svcCtx.Config.Verify.SampleSize, legacyScoped, shardScoped); err != nil {
			return nil, err
		}
		if err := compareTicketSnapshots(
			filterTicketsByOrderNumbers(legacyTickets, orderNumbers),
			filterTicketsByOrderNumbers(shardTickets, orderNumbers),
		); err != nil {
			return nil, err
		}
		if err := compareUserOrderIndexSnapshots(
			filterUserOrderIndexesByOrderNumbers(legacyIndexes, orderNumbers),
			filterUserOrderIndexesByOrderNumbers(shardIndexes, orderNumbers),
		); err != nil {
			return nil, err
		}

		resp.VerifiedSlots++
	}

	return resp, nil
}
