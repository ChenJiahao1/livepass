package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/repeatguard"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"
	"livepass/services/order-rpc/repository"
	payrpc "livepass/services/pay-rpc/payrpc"
	programrpc "livepass/services/program-rpc/programrpc"
	userrpc "livepass/services/user-rpc/userrpc"
)

const (
	orderStatusUnpaid    int64 = 1
	orderStatusCancelled int64 = 2
	orderStatusPaid      int64 = 3
	orderStatusRefunded  int64 = 4

	payStatusPaid     int64 = 2
	payStatusRefunded int64 = 3

	orderDateTimeLayout = "2006-01-02 15:04:05"
)

func validateCreateOrderReq(in *pb.CreateOrderReq) error {
	if in.GetUserId() <= 0 || in.GetPurchaseToken() == "" {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func validateUserOrderReq(userID, orderNumber int64) error {
	if userID <= 0 || orderNumber <= 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func validateUserShowTimeReq(userID, showTimeID int64) error {
	if userID <= 0 || showTimeID <= 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func validateRequestedTicketUsers(userResp *userrpc.GetUserAndTicketUserListResp, userID int64, requestedIDs []int64) error {
	if userResp == nil {
		return xerr.ErrOrderTicketUserInvalid
	}

	ticketUsers := make(map[int64]*userrpc.TicketUserInfo, len(userResp.GetTicketUserVoList()))
	for _, ticketUser := range userResp.GetTicketUserVoList() {
		if ticketUser == nil {
			continue
		}
		ticketUsers[ticketUser.GetId()] = ticketUser
	}

	requestedSet := make(map[int64]struct{}, len(requestedIDs))
	for _, requestedID := range requestedIDs {
		if _, exists := requestedSet[requestedID]; exists {
			return xerr.ErrOrderTicketUserInvalid
		}
		requestedSet[requestedID] = struct{}{}

		ticketUser, ok := ticketUsers[requestedID]
		if !ok || ticketUser.GetUserId() != userID {
			return xerr.ErrOrderTicketUserInvalid
		}
	}

	return nil
}

func parseOrderTime(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(orderDateTimeLayout, value, time.Local)
	if err != nil {
		return time.Time{}, status.Error(codes.Internal, err.Error())
	}

	return parsed, nil
}

func formatOrderTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Format(orderDateTimeLayout)
}

func formatOrderNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format(orderDateTimeLayout)
}

func findPreorderTicketCategory(list []*programrpc.ProgramPreorderTicketCategoryInfo, ticketCategoryID int64) (*programrpc.ProgramPreorderTicketCategoryInfo, bool) {
	for _, item := range list {
		if item != nil && item.GetId() == ticketCategoryID {
			return item, true
		}
	}

	return nil, false
}

func mapOrderSummary(order *model.DOrder) *pb.OrderListInfo {
	if order == nil {
		return &pb.OrderListInfo{}
	}

	return &pb.OrderListInfo{
		OrderNumber:        order.OrderNumber,
		ProgramId:          order.ProgramId,
		ShowTimeId:         order.ShowTimeId,
		ProgramTitle:       order.ProgramTitle,
		ProgramItemPicture: order.ProgramItemPicture,
		ProgramPlace:       order.ProgramPlace,
		ProgramShowTime:    formatOrderTime(order.ProgramShowTime),
		TicketCount:        order.TicketCount,
		OrderPrice:         int64(order.OrderPrice),
		OrderStatus:        order.OrderStatus,
		OrderExpireTime:    formatOrderTime(order.OrderExpireTime),
		CreateOrderTime:    formatOrderTime(order.CreateOrderTime),
		CancelOrderTime:    formatOrderNullTime(order.CancelOrderTime),
	}
}

func mapOrderDetail(order *model.DOrder, details []*model.DOrderTicketUser) *pb.OrderDetailInfo {
	if order == nil {
		return &pb.OrderDetailInfo{}
	}

	resp := &pb.OrderDetailInfo{
		OrderNumber:             order.OrderNumber,
		ProgramId:               order.ProgramId,
		ShowTimeId:              order.ShowTimeId,
		ProgramTitle:            order.ProgramTitle,
		ProgramItemPicture:      order.ProgramItemPicture,
		ProgramPlace:            order.ProgramPlace,
		ProgramShowTime:         formatOrderTime(order.ProgramShowTime),
		ProgramPermitChooseSeat: order.ProgramPermitChooseSeat,
		UserId:                  order.UserId,
		DistributionMode:        order.DistributionMode,
		TakeTicketMode:          order.TakeTicketMode,
		TicketCount:             order.TicketCount,
		OrderPrice:              int64(order.OrderPrice),
		OrderStatus:             order.OrderStatus,
		FreezeToken:             order.FreezeToken,
		OrderExpireTime:         formatOrderTime(order.OrderExpireTime),
		CreateOrderTime:         formatOrderTime(order.CreateOrderTime),
		CancelOrderTime:         formatOrderNullTime(order.CancelOrderTime),
	}
	if len(details) == 0 {
		return resp
	}

	resp.OrderTicketInfoVoList = make([]*pb.OrderTicketInfo, 0, len(details))
	for _, detail := range details {
		resp.OrderTicketInfoVoList = append(resp.OrderTicketInfoVoList, &pb.OrderTicketInfo{
			TicketUserId:       detail.TicketUserId,
			TicketUserName:     detail.TicketUserName,
			TicketUserIdNumber: detail.TicketUserIdNumber,
			TicketCategoryId:   detail.TicketCategoryId,
			TicketCategoryName: detail.TicketCategoryName,
			TicketPrice:        int64(detail.TicketPrice),
			SeatId:             detail.SeatId,
			SeatRow:            detail.SeatRow,
			SeatCol:            detail.SeatCol,
			SeatPrice:          int64(detail.SeatPrice),
		})
	}

	return resp
}

func mapOrderError(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, xerr.ErrInvalidParam):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, xerr.ErrInternal):
		return status.Error(codes.Internal, err.Error())
	case errors.Is(err, xerr.ErrOrderNotFound), errors.Is(err, xerr.ErrPayBillNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrOrderSubmitTooFrequent), errors.Is(err, xerr.ErrOrderOperateTooFrequent):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, xerr.ErrOrderStatusInvalid), errors.Is(err, xerr.ErrOrderTicketUserInvalid), errors.Is(err, xerr.ErrOrderPurchaseLimitExceeded), errors.Is(err, xerr.ErrOrderLimitLedgerNotReady), errors.Is(err, xerr.ErrOrderExpired), errors.Is(err, xerr.ErrOrderAlreadyPaid), errors.Is(err, xerr.ErrOrderRefundNotAllowed), errors.Is(err, xerr.ErrOrderRefundWindowClosed), errors.Is(err, xerr.ErrSeatInventoryInsufficient):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, xerr.ErrOrderRefundRuleInvalid):
		return status.Error(codes.Internal, err.Error())
	default:
		return err
	}
}

func logDelayTaskConsumeTransition(logger logx.Logger, taskType, taskKey string, fromStatus, toStatus, consumeAttempts int64) {
	if logger == nil || consumeAttempts <= 0 {
		return
	}

	logger.Infow("delay_task_consume_state_transition",
		logx.Field("task_type", taskType),
		logx.Field("task_key", taskKey),
		logx.Field("from_status", fromStatus),
		logx.Field("to_status", toStatus),
		logx.Field("consume_attempts", consumeAttempts),
	)
}

func cancelOrderWithLock(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber, userID int64, requireOwner bool, releaseReason string) (bool, error) {
	if orderNumber <= 0 {
		return false, xerr.ErrInvalidParam
	}

	unlock, err := lockOrderStatusGuard(ctx, svcCtx, orderNumber)
	if err != nil {
		return false, err
	}
	if unlock != nil {
		defer unlock()
	}

	order, err := loadOrderSnapshot(ctx, svcCtx, orderNumber)
	if err != nil {
		return false, err
	}
	if requireOwner && order.UserId != userID {
		return false, xerr.ErrOrderNotFound
	}
	if order.OrderStatus == orderStatusCancelled {
		if err := syncClosedRushAttempt(ctx, svcCtx, order.ShowTimeId, order.OrderNumber, time.Now()); err != nil {
			logx.WithContext(ctx).Errorf("sync closed rush attempt failed, orderNumber=%d err=%v", order.OrderNumber, err)
		}
		return false, nil
	}
	if order.OrderStatus != orderStatusUnpaid {
		return false, xerr.ErrOrderStatusInvalid
	}

	if _, err := svcCtx.ProgramRpc.ReleaseSeatFreeze(ctx, &programrpc.ReleaseSeatFreezeReq{
		FreezeToken:   order.FreezeToken,
		ReleaseReason: releaseReason,
	}); err != nil {
		return false, err
	}

	changed, err := finalizeOrderCancel(ctx, svcCtx, order.OrderNumber)
	if err != nil {
		return false, err
	}
	if changed {
		if err := syncClosedRushAttempt(ctx, svcCtx, order.ShowTimeId, order.OrderNumber, time.Now()); err != nil {
			logx.WithContext(ctx).Errorf("sync closed rush attempt failed, orderNumber=%d err=%v", order.OrderNumber, err)
		}
	}

	return changed, nil
}

func loadOrderSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64) (*model.DOrder, error) {
	if svcCtx == nil || svcCtx.OrderRepository == nil {
		return nil, xerr.ErrInternal
	}

	var snapshot *model.DOrder
	err := svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		order, err := tx.FindOrderByNumberForUpdate(txCtx, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		snapshot = cloneOrderSnapshot(order)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func finalizeOrderCancel(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64) (bool, error) {
	var changed bool
	err := svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		order, err := tx.FindOrderByNumberForUpdate(txCtx, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		if order.OrderStatus == orderStatusCancelled {
			return nil
		}
		if order.OrderStatus != orderStatusUnpaid {
			return xerr.ErrOrderStatusInvalid
		}

		cancelTime := time.Now()
		if err := tx.UpdateCancelStatus(txCtx, order.OrderNumber, cancelTime); err != nil {
			return err
		}
		if err := tx.DeleteGuardsByOrderNumber(txCtx, order.OrderNumber); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}

	return changed, nil
}

func finalizeOrderPay(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64, payTime time.Time) error {
	return svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		order, err := tx.FindOrderByNumberForUpdate(txCtx, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		if order.OrderStatus == orderStatusPaid {
			return nil
		}
		if order.OrderStatus != orderStatusUnpaid {
			return xerr.ErrOrderStatusInvalid
		}

		return tx.UpdatePayStatus(txCtx, order.OrderNumber, payTime)
	})
}

func cloneOrderSnapshot(order *model.DOrder) *model.DOrder {
	if order == nil {
		return nil
	}

	cloned := *order
	return &cloned
}

func lockOrderStatusGuard(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64) (repeatguard.UnlockFunc, error) {
	if svcCtx == nil || svcCtx.RepeatGuard == nil {
		return nil, nil
	}

	unlock, err := svcCtx.RepeatGuard.Lock(ctx, repeatguard.OrderStatusKey(orderNumber))
	if err != nil {
		if errors.Is(err, repeatguard.ErrLocked) {
			return nil, xerr.ErrOrderOperateTooFrequent
		}
		return nil, err
	}

	return unlock, nil
}

func mapPayOrderResp(order *model.DOrder, payBill *payrpc.GetPayBillResp) *pb.PayOrderResp {
	if order == nil {
		return &pb.PayOrderResp{}
	}

	resp := &pb.PayOrderResp{
		OrderNumber: order.OrderNumber,
		OrderStatus: order.OrderStatus,
	}
	if payBill == nil {
		return resp
	}

	resp.PayBillNo = payBill.GetPayBillNo()
	resp.PayStatus = payBill.GetPayStatus()
	resp.PayTime = payBill.GetPayTime()
	return resp
}

func mapPayCheckResp(order *model.DOrder, payBill *payrpc.GetPayBillResp) *pb.PayCheckResp {
	if order == nil {
		return &pb.PayCheckResp{}
	}

	resp := &pb.PayCheckResp{
		OrderNumber: order.OrderNumber,
		OrderStatus: order.OrderStatus,
	}
	if payBill == nil {
		return resp
	}

	resp.PayBillNo = payBill.GetPayBillNo()
	resp.PayStatus = payBill.GetPayStatus()
	resp.PayTime = payBill.GetPayTime()
	return resp
}

func mapRefundOrderResp(order *model.DOrder, refundBill *payrpc.RefundResp, refundPercent int64) *pb.RefundOrderResp {
	if order == nil {
		return &pb.RefundOrderResp{}
	}

	resp := &pb.RefundOrderResp{
		OrderNumber: order.OrderNumber,
		OrderStatus: orderStatusRefunded,
	}
	if refundBill == nil {
		return resp
	}

	resp.RefundAmount = refundBill.GetRefundAmount()
	resp.RefundPercent = refundPercent
	resp.RefundBillNo = refundBill.GetRefundBillNo()
	resp.RefundTime = refundBill.GetRefundTime()
	return resp
}

func calculateRefundPercent(orderPrice, refundAmount int64) int64 {
	if orderPrice <= 0 || refundAmount <= 0 {
		return 0
	}

	return (refundAmount*100 + orderPrice/2) / orderPrice
}

func orderTicketSeatIDs(orderTickets []*model.DOrderTicketUser) []int64 {
	if len(orderTickets) == 0 {
		return nil
	}

	seatIDs := make([]int64, 0, len(orderTickets))
	for _, orderTicket := range orderTickets {
		if orderTicket == nil || orderTicket.SeatId <= 0 {
			continue
		}
		seatIDs = append(seatIDs, orderTicket.SeatId)
	}

	return seatIDs
}

func buildRefundRequestNo(orderNumber int64) string {
	return fmt.Sprintf("refund-%d", orderNumber)
}

func refundRejectReasonToError(reason string) error {
	switch reason {
	case "":
		return xerr.ErrOrderRefundNotAllowed
	case "refund stage not matched":
		return xerr.ErrOrderRefundWindowClosed
	case "program does not permit refund":
		return xerr.ErrOrderRefundNotAllowed
	default:
		return status.Error(codes.FailedPrecondition, reason)
	}
}

func mustGetPayBillForOrder(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64) (*payrpc.GetPayBillResp, error) {
	resp, err := svcCtx.PayRpc.GetPayBill(ctx, &payrpc.GetPayBillReq{OrderNumber: orderNumber})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, xerr.ErrPayBillNotFound
		}
		return nil, err
	}

	return resp, nil
}

func applyRefundPayStatus(payBill *payrpc.GetPayBillResp, refundResp *payrpc.RefundResp) *payrpc.GetPayBillResp {
	if payBill == nil {
		return nil
	}

	resp := *payBill
	if refundResp != nil && refundResp.GetPayStatus() > 0 {
		resp.PayStatus = refundResp.GetPayStatus()
	}

	return &resp
}

func compensationRefundReason() string {
	return "订单已取消，支付晚到补偿退款"
}

func convergeOrderRefunded(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64) error {
	return svcCtx.OrderRepository.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		order, err := tx.FindOrderByNumberForUpdate(txCtx, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		if order.OrderStatus == orderStatusRefunded {
			return nil
		}
		if order.OrderStatus != orderStatusPaid && order.OrderStatus != orderStatusCancelled {
			return xerr.ErrOrderStatusInvalid
		}

		refundTime := time.Now()
		if err := tx.UpdateRefundStatus(txCtx, orderNumber, refundTime); err != nil {
			return err
		}
		if err := tx.DeleteGuardsByOrderNumber(txCtx, order.OrderNumber); err != nil {
			return err
		}
		return nil
	})
}

func findOwnedOrder(ctx context.Context, svcCtx *svc.ServiceContext, userID, orderNumber int64) (*model.DOrder, error) {
	order, err := svcCtx.OrderRepository.FindOrderByNumber(ctx, orderNumber)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, xerr.ErrOrderNotFound
		}
		return nil, err
	}
	if order.UserId != userID {
		return nil, xerr.ErrOrderNotFound
	}

	return order, nil
}

func deriveTicketStatus(order *model.DOrder, details []*model.DOrderTicketUser) int64 {
	if len(details) == 0 {
		if order == nil {
			return 0
		}
		return order.OrderStatus
	}

	candidate := details[0].OrderStatus
	for _, detail := range details[1:] {
		if detail == nil || detail.OrderStatus != candidate {
			if order == nil {
				return candidate
			}
			return order.OrderStatus
		}
	}

	return candidate
}

func derivePayStatus(ctx context.Context, svcCtx *svc.ServiceContext, order *model.DOrder) (int64, error) {
	if order == nil {
		return 0, nil
	}
	if order.OrderStatus != orderStatusPaid && order.OrderStatus != orderStatusRefunded {
		return 0, nil
	}

	payBill, err := mustGetPayBillForOrder(ctx, svcCtx, order.OrderNumber)
	if err != nil {
		if order.OrderStatus == orderStatusRefunded && errors.Is(err, xerr.ErrPayBillNotFound) {
			return payStatusRefunded, nil
		}
		return 0, err
	}

	return payBill.GetPayStatus(), nil
}

func previewRefundOrder(ctx context.Context, svcCtx *svc.ServiceContext, order *model.DOrder) (*pb.PreviewRefundOrderResp, error) {
	resp := &pb.PreviewRefundOrderResp{}
	if order == nil {
		return resp, nil
	}

	resp.OrderNumber = order.OrderNumber
	switch order.OrderStatus {
	case orderStatusRefunded:
		resp.RejectReason = "订单已退款"
		return resp, nil
	case orderStatusPaid:
	default:
		resp.RejectReason = "当前订单不可退"
		return resp, nil
	}

	payBill, err := mustGetPayBillForOrder(ctx, svcCtx, order.OrderNumber)
	if err != nil {
		return nil, err
	}
	if payBill.GetPayStatus() == payStatusRefunded {
		resp.RejectReason = "订单已退款"
		return resp, nil
	}
	if payBill.GetPayStatus() != payStatusPaid {
		resp.RejectReason = "当前订单不可退"
		return resp, nil
	}

	evaluateResp, err := svcCtx.ProgramRpc.EvaluateRefundRule(ctx, &programrpc.EvaluateRefundRuleReq{
		ShowTimeId:  order.ShowTimeId,
		OrderAmount: int64(order.OrderPrice),
	})
	if err != nil {
		return nil, err
	}
	if !evaluateResp.GetAllowRefund() {
		resp.RejectReason = evaluateResp.GetRejectReason()
		if resp.RejectReason == "" {
			resp.RejectReason = "当前订单不可退"
		}
		return resp, nil
	}

	resp.AllowRefund = true
	resp.RefundAmount = evaluateResp.GetRefundAmount()
	resp.RefundPercent = evaluateResp.GetRefundPercent()
	return resp, nil
}

func mapOrderServiceView(order *model.DOrder, payStatus, ticketStatus int64, preview *pb.PreviewRefundOrderResp) *pb.OrderServiceViewResp {
	if order == nil {
		return &pb.OrderServiceViewResp{}
	}

	resp := &pb.OrderServiceViewResp{
		OrderNumber:     order.OrderNumber,
		ProgramId:       order.ProgramId,
		ShowTimeId:      order.ShowTimeId,
		OrderStatus:     order.OrderStatus,
		PayStatus:       payStatus,
		TicketStatus:    ticketStatus,
		ProgramTitle:    order.ProgramTitle,
		ProgramShowTime: formatOrderTime(order.ProgramShowTime),
		TicketCount:     order.TicketCount,
		OrderPrice:      int64(order.OrderPrice),
	}
	if preview == nil {
		return resp
	}

	resp.CanRefund = preview.GetAllowRefund()
	resp.RefundBlockedReason = preview.GetRejectReason()
	return resp
}
