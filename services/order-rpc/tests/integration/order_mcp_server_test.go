package integration_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/svc"
	ordermcp "livepass/services/order-rpc/mcp"
	"livepass/services/order-rpc/repository"
	"livepass/services/order-rpc/sharding"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

type fakeMCPOrderRepository struct {
	orders  map[int64]*model.DOrder
	tickets map[int64][]*model.DOrderTicketUser
}

type fakeMCPOrderTx struct {
	repo *fakeMCPOrderRepository
}

func newMCPServiceContext(t *testing.T, repo repository.OrderRepository) (*svc.ServiceContext, *fakeOrderProgramRPC, *fakeOrderPayRPC) {
	t.Helper()

	mustInitOrderTestXid(t)

	programRPC := &fakeOrderProgramRPC{
		releaseSoldSeatsResp: &programrpc.ReleaseSoldSeatsResp{Success: true},
	}
	payRPC := &fakeOrderPayRPC{}
	return &svc.ServiceContext{
		OrderRepository: repo,
		ProgramRpc:      programRPC,
		PayRpc:          payRPC,
	}, programRPC, payRPC
}

func TestOrderMCPServer_ListsTools(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders:  map[int64]*model.DOrder{},
		tickets: map[int64][]*model.DOrderTicketUser{},
	}
	svcCtx, _, _ := newMCPServiceContext(t, repo)
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	toolNames := server.ToolNames()
	sort.Strings(toolNames)

	if len(toolNames) != 4 {
		t.Fatalf("expected 4 tools, got %v", toolNames)
	}
	expected := []string{
		"get_order_detail_for_service",
		"list_user_orders",
		"preview_refund_order",
		"refund_order",
	}
	for idx, name := range expected {
		if toolNames[idx] != name {
			t.Fatalf("unexpected tool names: %v", toolNames)
		}
	}
}

func TestListUserOrdersTool_ReturnsStableRecentOrderList(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders: map[int64]*model.DOrder{
			10001: {
				Id:              1,
				OrderNumber:     10001,
				UserId:          3001,
				ProgramTitle:    "演出 A",
				ProgramShowTime: time.Date(2026, 4, 5, 20, 0, 0, 0, time.UTC),
				OrderStatus:     testOrderStatusPaid,
				CreateOrderTime: time.Date(2026, 4, 5, 8, 0, 0, 0, time.UTC),
			},
			10002: {
				Id:              2,
				OrderNumber:     10002,
				UserId:          3001,
				ProgramTitle:    "演出 B",
				ProgramShowTime: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				OrderStatus:     testOrderStatusPaid,
				CreateOrderTime: time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			},
		},
		tickets: map[int64][]*model.DOrderTicketUser{},
	}
	svcCtx, _, _ := newMCPServiceContext(t, repo)
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	result, err := server.ListUserOrders(context.Background(), ordermcp.ListUserOrdersArgs{
		UserID:     3001,
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("ListUserOrders returned error: %v", err)
	}
	if len(result.Orders) != 2 {
		t.Fatalf("expected 2 orders, got %+v", result)
	}
	if result.Orders[0].OrderID != "ORD-10002" {
		t.Fatalf("expected latest order first, got %+v", result.Orders)
	}
	if result.Orders[0].CreateOrderTime == "" {
		t.Fatalf("expected create_order_time to be normalized")
	}
}

func TestGetOrderDetailTool_NormalizesServiceView(t *testing.T) {
	repo := &fakeMCPOrderRepository{
		orders: map[int64]*model.DOrder{
			10001: {
				Id:              1,
				OrderNumber:     10001,
				UserId:          3001,
				ProgramId:       20001,
				ProgramTitle:    "演出 A",
				ProgramShowTime: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				TicketCount:     2,
				OrderPrice:      299,
				OrderStatus:     testOrderStatusPaid,
				CreateOrderTime: time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			},
		},
		tickets: map[int64][]*model.DOrderTicketUser{
			10001: {
				{OrderNumber: 10001, UserId: 3001, OrderStatus: testOrderStatusPaid},
			},
		},
	}
	svcCtx, programRPC, payRPC := newMCPServiceContext(t, repo)
	payRPC.getPayBillResp = &payrpc.GetPayBillResp{
		OrderNumber: 10001,
		UserId:      3001,
		PayStatus:   2,
	}
	programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
		AllowRefund:   true,
		RefundAmount:  299,
		RefundPercent: 100,
	}
	server := ordermcp.NewServer(gomcp.McpConf{}, svcCtx)

	result, err := server.GetOrderDetailForService(context.Background(), ordermcp.GetOrderDetailForServiceArgs{
		UserID:  3001,
		OrderID: "ORD-10001",
	})
	if err != nil {
		t.Fatalf("GetOrderDetailForService returned error: %v", err)
	}
	if result.OrderID != "ORD-10001" || result.Status != "paid" {
		t.Fatalf("unexpected detail result: %+v", result)
	}
	if result.PaymentStatus != "paid" || result.TicketStatus != "issued" {
		t.Fatalf("expected normalized pay/ticket statuses, got %+v", result)
	}
}

func (r *fakeMCPOrderRepository) TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, repository.OrderTx) error) error {
	return fn(ctx, &fakeMCPOrderTx{repo: r})
}

func (r *fakeMCPOrderRepository) TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, repository.OrderTx) error) error {
	return fn(ctx, &fakeMCPOrderTx{repo: r})
}

func (r *fakeMCPOrderRepository) FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	order, ok := r.orders[orderNumber]
	if !ok {
		return nil, model.ErrNotFound
	}
	return order, nil
}

func (r *fakeMCPOrderRepository) FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error) {
	return r.tickets[orderNumber], nil
}

func (r *fakeMCPOrderRepository) FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error) {
	orders := make([]*model.DOrder, 0, len(r.orders))
	for _, order := range r.orders {
		if order.UserId != userID {
			continue
		}
		if orderStatus > 0 && order.OrderStatus != orderStatus {
			continue
		}
		orders = append(orders, order)
	}
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].CreateOrderTime.Equal(orders[j].CreateOrderTime) {
			return orders[i].Id > orders[j].Id
		}
		return orders[i].CreateOrderTime.After(orders[j].CreateOrderTime)
	})
	return orders, int64(len(orders)), nil
}

func (r *fakeMCPOrderRepository) FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error) {
	return nil, nil
}

func (r *fakeMCPOrderRepository) CountActiveTicketsByUserShowTime(ctx context.Context, userID, showTimeID int64) (int64, error) {
	return 0, nil
}

func (r *fakeMCPOrderRepository) ListUnpaidReservationsByUserShowTime(ctx context.Context, userID, showTimeID int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

func (r *fakeMCPOrderRepository) WalkActiveUserGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderUserGuard) error) error {
	return nil
}

func (r *fakeMCPOrderRepository) WalkActiveViewerGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderViewerGuard) error) error {
	return nil
}

func (r *fakeMCPOrderRepository) RouteByUserID(ctx context.Context, userID int64) (sharding.Route, error) {
	return sharding.Route{}, nil
}

func (r *fakeMCPOrderRepository) RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error) {
	return sharding.Route{}, nil
}

func (tx *fakeMCPOrderTx) Route() sharding.Route {
	return sharding.Route{}
}

func (tx *fakeMCPOrderTx) InsertOrder(ctx context.Context, order *model.DOrder) error {
	return nil
}

func (tx *fakeMCPOrderTx) InsertOrderTickets(ctx context.Context, tickets []*model.DOrderTicketUser) error {
	return nil
}

func (tx *fakeMCPOrderTx) InsertUserGuard(ctx context.Context, guard *model.DOrderUserGuard) error {
	return nil
}

func (tx *fakeMCPOrderTx) InsertViewerGuards(ctx context.Context, guards []*model.DOrderViewerGuard) error {
	return nil
}

func (tx *fakeMCPOrderTx) InsertSeatGuards(ctx context.Context, guards []*model.DOrderSeatGuard) error {
	return nil
}

func (tx *fakeMCPOrderTx) InsertDelayTasks(ctx context.Context, rows []*model.DDelayTaskOutbox) error {
	return nil
}

func (tx *fakeMCPOrderTx) MarkDelayTaskProcessed(ctx context.Context, taskType, taskKey string, processedAt time.Time) (int64, int64, error) {
	return 1, 1, nil
}

func (tx *fakeMCPOrderTx) DeleteGuardsByOrderNumber(ctx context.Context, orderNumber int64) error {
	return nil
}

func (tx *fakeMCPOrderTx) FindOrderByNumberForUpdate(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	return tx.repo.FindOrderByNumber(ctx, orderNumber)
}

func (tx *fakeMCPOrderTx) UpdateCancelStatus(ctx context.Context, orderNumber int64, cancelTime time.Time) error {
	return nil
}

func (tx *fakeMCPOrderTx) UpdatePayStatus(ctx context.Context, orderNumber int64, payTime time.Time) error {
	return nil
}

func (tx *fakeMCPOrderTx) UpdateRefundStatus(ctx context.Context, orderNumber int64, refundTime time.Time) error {
	order, ok := tx.repo.orders[orderNumber]
	if !ok {
		return fmt.Errorf("order not found: %d", orderNumber)
	}
	order.OrderStatus = testOrderStatusRefunded
	for _, ticket := range tx.repo.tickets[orderNumber] {
		ticket.OrderStatus = testOrderStatusRefunded
	}
	return nil
}
