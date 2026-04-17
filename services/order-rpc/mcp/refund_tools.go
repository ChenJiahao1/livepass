package ordermcp

import (
	"context"
	"strconv"

	"livepass/services/order-rpc/internal/logic"
	"livepass/services/order-rpc/pb"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

func registerRefundTools(server *Server) {
	server.recordTool("preview_refund_order")
	gomcp.AddTool(server.server, &gomcp.Tool{
		Name:        "preview_refund_order",
		Description: "Preview refund eligibility for an order",
	}, server.previewRefundTool)

	server.recordTool("refund_order")
	gomcp.AddTool(server.server, &gomcp.Tool{
		Name:        "refund_order",
		Description: "Submit refund for an order",
	}, server.refundOrderTool)
}

func (s *Server) PreviewRefundOrder(ctx context.Context, args PreviewRefundOrderArgs) (*RefundPreviewResult, error) {
	orderNumber, err := parseOrderID(args.OrderID)
	if err != nil {
		return nil, err
	}
	resp, err := logic.NewPreviewRefundOrderLogic(ctx, s.svcCtx).PreviewRefundOrder(&pb.PreviewRefundOrderReq{
		UserId:      args.UserID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		return nil, err
	}
	return &RefundPreviewResult{
		OrderID:       formatOrderID(resp.GetOrderNumber()),
		AllowRefund:   resp.GetAllowRefund(),
		RefundAmount:  strconv.FormatInt(resp.GetRefundAmount(), 10),
		RefundPercent: resp.GetRefundPercent(),
		RejectReason:  resp.GetRejectReason(),
	}, nil
}

func (s *Server) previewRefundTool(ctx context.Context, _ *gomcp.CallToolRequest, args PreviewRefundOrderArgs) (*gomcp.CallToolResult, any, error) {
	result, err := s.PreviewRefundOrder(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	callResult, err := s.makeJSONResult(ctx, result)
	if err != nil {
		return nil, nil, err
	}
	return callResult, nil, nil
}

func (s *Server) RefundOrder(ctx context.Context, args RefundOrderArgs) (*RefundOrderResult, error) {
	orderNumber, err := parseOrderID(args.OrderID)
	if err != nil {
		return nil, err
	}
	resp, err := logic.NewRefundOrderLogic(ctx, s.svcCtx).RefundOrder(&pb.RefundOrderReq{
		UserId:      args.UserID,
		OrderNumber: orderNumber,
		Reason:      args.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &RefundOrderResult{
		OrderID:       formatOrderID(resp.GetOrderNumber()),
		Status:        normalizeOrderStatus(resp.GetOrderStatus()),
		RefundAmount:  strconv.FormatInt(resp.GetRefundAmount(), 10),
		RefundPercent: resp.GetRefundPercent(),
		RefundBillNo:  strconv.FormatInt(resp.GetRefundBillNo(), 10),
		RefundTime:    resp.GetRefundTime(),
	}, nil
}

func (s *Server) refundOrderTool(ctx context.Context, _ *gomcp.CallToolRequest, args RefundOrderArgs) (*gomcp.CallToolResult, any, error) {
	result, err := s.RefundOrder(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	callResult, err := s.makeJSONResult(ctx, result)
	if err != nil {
		return nil, nil, err
	}
	return callResult, nil, nil
}
