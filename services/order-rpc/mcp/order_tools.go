package ordermcp

import (
	"context"

	"damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

func registerOrderTools(server *Server) {
	server.recordTool("list_user_orders")
	gomcp.AddTool(server.server, &gomcp.Tool{
		Name:        "list_user_orders",
		Description: "List recent orders for a user",
	}, server.listUserOrdersTool)

	server.recordTool("get_order_detail_for_service")
	gomcp.AddTool(server.server, &gomcp.Tool{
		Name:        "get_order_detail_for_service",
		Description: "Get service-facing order detail",
	}, server.getOrderDetailTool)
}

func (s *Server) ListUserOrders(ctx context.Context, args ListUserOrdersArgs) (*ListUserOrdersResult, error) {
	resp, err := logic.NewListOrdersLogic(ctx, s.svcCtx).ListOrders(&pb.ListOrdersReq{
		UserId:      args.UserID,
		PageNumber:  args.PageNumber,
		PageSize:    args.PageSize,
		OrderStatus: args.OrderStatus,
	})
	if err != nil {
		return nil, err
	}

	result := &ListUserOrdersResult{
		Orders: make([]OrderSummary, 0, len(resp.GetList())),
	}
	for _, order := range resp.GetList() {
		result.Orders = append(result.Orders, OrderSummary{
			OrderID:         formatOrderID(order.GetOrderNumber()),
			Status:          normalizeOrderStatus(order.GetOrderStatus()),
			ProgramTitle:    order.GetProgramTitle(),
			ProgramShowTime: order.GetProgramShowTime(),
			CreateOrderTime: order.GetCreateOrderTime(),
		})
	}
	return result, nil
}

func (s *Server) listUserOrdersTool(ctx context.Context, _ *gomcp.CallToolRequest, args ListUserOrdersArgs) (*gomcp.CallToolResult, any, error) {
	result, err := s.ListUserOrders(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	callResult, err := s.makeJSONResult(ctx, result)
	if err != nil {
		return nil, nil, err
	}
	return callResult, nil, nil
}

func (s *Server) GetOrderDetailForService(ctx context.Context, args GetOrderDetailForServiceArgs) (*OrderDetailResult, error) {
	orderNumber, err := parseOrderID(args.OrderID)
	if err != nil {
		return nil, err
	}
	resp, err := logic.NewGetOrderServiceViewLogic(ctx, s.svcCtx).GetOrderServiceView(&pb.GetOrderServiceViewReq{
		UserId:      args.UserID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		return nil, err
	}
	return &OrderDetailResult{
		OrderID:             formatOrderID(resp.GetOrderNumber()),
		Status:              normalizeOrderStatus(resp.GetOrderStatus()),
		PaymentStatus:       normalizePayStatus(resp.GetPayStatus()),
		TicketStatus:        normalizeTicketStatus(resp.GetTicketStatus()),
		ProgramTitle:        resp.GetProgramTitle(),
		ProgramShowTime:     resp.GetProgramShowTime(),
		TicketCount:         resp.GetTicketCount(),
		OrderPrice:          resp.GetOrderPrice(),
		CanRefund:           resp.GetCanRefund(),
		RefundBlockedReason: resp.GetRefundBlockedReason(),
	}, nil
}

func (s *Server) getOrderDetailTool(ctx context.Context, _ *gomcp.CallToolRequest, args GetOrderDetailForServiceArgs) (*gomcp.CallToolResult, any, error) {
	result, err := s.GetOrderDetailForService(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	callResult, err := s.makeJSONResult(ctx, result)
	if err != nil {
		return nil, nil, err
	}
	return callResult, nil, nil
}
