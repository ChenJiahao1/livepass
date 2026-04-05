package ordermcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ListUserOrdersArgs struct {
	UserID      int64 `json:"user_id"`
	PageNumber  int64 `json:"page_number,omitempty"`
	PageSize    int64 `json:"page_size,omitempty"`
	OrderStatus int64 `json:"order_status,omitempty"`
}

type GetOrderDetailForServiceArgs struct {
	UserID  int64  `json:"user_id"`
	OrderID string `json:"order_id"`
}

type PreviewRefundOrderArgs struct {
	UserID  int64  `json:"user_id"`
	OrderID string `json:"order_id"`
}

type RefundOrderArgs struct {
	UserID  int64  `json:"user_id"`
	OrderID string `json:"order_id"`
	Reason  string `json:"reason,omitempty"`
}

type OrderSummary struct {
	OrderID         string `json:"order_id"`
	Status          string `json:"status"`
	ProgramTitle    string `json:"program_title"`
	ProgramShowTime string `json:"program_show_time"`
	CreateOrderTime string `json:"create_order_time"`
}

type ListUserOrdersResult struct {
	Orders []OrderSummary `json:"orders"`
}

type OrderDetailResult struct {
	OrderID             string `json:"order_id"`
	Status              string `json:"status"`
	PaymentStatus       string `json:"payment_status"`
	TicketStatus        string `json:"ticket_status"`
	ProgramTitle        string `json:"program_title"`
	ProgramShowTime     string `json:"program_show_time"`
	TicketCount         int64  `json:"ticket_count"`
	OrderPrice          int64  `json:"order_price"`
	CanRefund           bool   `json:"can_refund"`
	RefundBlockedReason string `json:"refund_blocked_reason"`
}

type RefundPreviewResult struct {
	OrderID       string `json:"order_id"`
	AllowRefund   bool   `json:"allow_refund"`
	RefundAmount  string `json:"refund_amount"`
	RefundPercent int64  `json:"refund_percent"`
	RejectReason  string `json:"reject_reason"`
}

type RefundOrderResult struct {
	OrderID       string `json:"order_id"`
	Status        string `json:"status"`
	RefundAmount  string `json:"refund_amount"`
	RefundPercent int64  `json:"refund_percent"`
	RefundBillNo  string `json:"refund_bill_no"`
	RefundTime    string `json:"refund_time"`
}

func formatOrderID(orderNumber int64) string {
	return fmt.Sprintf("ORD-%d", orderNumber)
}

func parseOrderID(orderID string) (int64, error) {
	normalized := strings.TrimSpace(strings.ToUpper(orderID))
	normalized = strings.TrimPrefix(normalized, "ORD-")
	if normalized == "" {
		return 0, fmt.Errorf("invalid order_id")
	}
	orderNumber, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid order_id: %w", err)
	}
	return orderNumber, nil
}

func normalizeOrderStatus(status int64) string {
	switch status {
	case 1:
		return "unpaid"
	case 2:
		return "cancelled"
	case 3:
		return "paid"
	case 4:
		return "refunded"
	default:
		return fmt.Sprintf("unknown:%d", status)
	}
}

func normalizePayStatus(status int64) string {
	switch status {
	case 1:
		return "unpaid"
	case 2:
		return "paid"
	case 3:
		return "refunded"
	default:
		return fmt.Sprintf("unknown:%d", status)
	}
}

func normalizeTicketStatus(status int64) string {
	switch status {
	case 1:
		return "unpaid"
	case 2:
		return "cancelled"
	case 3:
		return "issued"
	case 4:
		return "refunded"
	default:
		return fmt.Sprintf("unknown:%d", status)
	}
}

func marshalPayload(payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
