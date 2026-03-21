package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xid"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

type orderSnapshotBundle struct {
	order        *model.DOrder
	orderTickets []*model.DOrderTicketUser
}

func validateCreateOrderReq(in *pb.CreateOrderReq) error {
	if in.GetUserId() <= 0 || in.GetProgramId() <= 0 || in.GetTicketCategoryId() <= 0 || len(in.GetTicketUserIds()) == 0 {
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

func validateUserProgramReq(userID, programID int64) error {
	if userID <= 0 || programID <= 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func buildOrderSnapshotBundle(
	in *pb.CreateOrderReq,
	preorder *programrpc.ProgramPreorderInfo,
	userResp *userrpc.GetUserAndTicketUserListResp,
	freezeResp *programrpc.AutoAssignAndFreezeSeatsResp,
	now time.Time,
	closeAfter time.Duration,
) (*orderSnapshotBundle, error) {
	if preorder == nil || userResp == nil || freezeResp == nil {
		return nil, xerr.ErrInternal
	}

	ticketCategory, ok := findPreorderTicketCategory(preorder.GetTicketCategoryVoList(), in.GetTicketCategoryId())
	if !ok {
		return nil, xerr.ErrProgramTicketCategoryNotFound
	}
	if len(freezeResp.GetSeats()) != len(in.GetTicketUserIds()) {
		return nil, xerr.ErrSeatInventoryInsufficient
	}

	ticketUsers := make(map[int64]*userrpc.TicketUserInfo, len(userResp.GetTicketUserVoList()))
	for _, ticketUser := range userResp.GetTicketUserVoList() {
		if ticketUser == nil {
			continue
		}
		ticketUsers[ticketUser.GetId()] = ticketUser
	}

	showTime, err := parseOrderTime(preorder.GetShowTime())
	if err != nil {
		return nil, err
	}

	orderNumber := xid.New()
	orderExpireTime := now.Add(closeAfter)
	order := &model.DOrder{
		Id:                      xid.New(),
		OrderNumber:             orderNumber,
		ProgramId:               in.GetProgramId(),
		ProgramTitle:            preorder.GetTitle(),
		ProgramItemPicture:      preorder.GetItemPicture(),
		ProgramPlace:            preorder.GetPlace(),
		ProgramShowTime:         showTime,
		ProgramPermitChooseSeat: preorder.GetPermitChooseSeat(),
		UserId:                  in.GetUserId(),
		DistributionMode:        in.GetDistributionMode(),
		TakeTicketMode:          in.GetTakeTicketMode(),
		TicketCount:             int64(len(in.GetTicketUserIds())),
		OrderPrice:              float64(ticketCategory.GetPrice() * int64(len(in.GetTicketUserIds()))),
		OrderStatus:             orderStatusUnpaid,
		FreezeToken:             freezeResp.GetFreezeToken(),
		OrderExpireTime:         orderExpireTime,
		CreateOrderTime:         now,
		CancelOrderTime:         sql.NullTime{},
		CreateTime:              now,
		EditTime:                now,
		Status:                  1,
	}

	orderTickets := make([]*model.DOrderTicketUser, 0, len(in.GetTicketUserIds()))
	for idx, ticketUserID := range in.GetTicketUserIds() {
		ticketUser, ok := ticketUsers[ticketUserID]
		if !ok || ticketUser.GetUserId() != in.GetUserId() {
			return nil, xerr.ErrOrderTicketUserInvalid
		}
		seat := freezeResp.GetSeats()[idx]
		orderTickets = append(orderTickets, &model.DOrderTicketUser{
			Id:                 xid.New(),
			OrderNumber:        orderNumber,
			UserId:             in.GetUserId(),
			TicketUserId:       ticketUser.GetId(),
			TicketUserName:     ticketUser.GetRelName(),
			TicketUserIdNumber: ticketUser.GetIdNumber(),
			TicketCategoryId:   ticketCategory.GetId(),
			TicketCategoryName: ticketCategory.GetIntroduce(),
			TicketPrice:        float64(ticketCategory.GetPrice()),
			SeatId:             seat.GetSeatId(),
			SeatRow:            seat.GetRowCode(),
			SeatCol:            seat.GetColCode(),
			SeatPrice:          float64(seat.GetPrice()),
			OrderStatus:        orderStatusUnpaid,
			CreateOrderTime:    now,
			CreateTime:         now,
			EditTime:           now,
			Status:             1,
		})
	}

	return &orderSnapshotBundle{
		order:        order,
		orderTickets: orderTickets,
	}, nil
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

	for _, requestedID := range requestedIDs {
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
	case errors.Is(err, xerr.ErrOrderNotFound), errors.Is(err, xerr.ErrPayBillNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, xerr.ErrOrderStatusInvalid), errors.Is(err, xerr.ErrOrderTicketUserInvalid), errors.Is(err, xerr.ErrOrderPurchaseLimitExceeded), errors.Is(err, xerr.ErrOrderExpired), errors.Is(err, xerr.ErrOrderAlreadyPaid), errors.Is(err, xerr.ErrOrderRefundNotAllowed), errors.Is(err, xerr.ErrOrderRefundWindowClosed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, xerr.ErrOrderRefundRuleInvalid):
		return status.Error(codes.Internal, err.Error())
	default:
		return err
	}
}

func compensateOrderFreezeRelease(ctx context.Context, svcCtx *svc.ServiceContext, freezeToken, reason string) {
	if freezeToken == "" {
		return
	}

	if _, err := svcCtx.ProgramRpc.ReleaseSeatFreeze(ctx, &programrpc.ReleaseSeatFreezeReq{
		FreezeToken:   freezeToken,
		ReleaseReason: reason,
	}); err != nil {
		logx.WithContext(ctx).Errorf("release seat freeze compensation failed, freezeToken=%s err=%v", freezeToken, err)
	}
}

func cancelOrderWithLock(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber, userID int64, requireOwner bool, releaseReason string) (bool, error) {
	if orderNumber <= 0 {
		return false, xerr.ErrInvalidParam
	}

	var changed bool
	err := svcCtx.SqlConn.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		order, err := svcCtx.DOrderModel.FindOneByOrderNumberForUpdate(txCtx, session, orderNumber)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return xerr.ErrOrderNotFound
			}
			return err
		}
		if requireOwner && order.UserId != userID {
			return xerr.ErrOrderNotFound
		}
		if order.OrderStatus == orderStatusCancelled {
			return nil
		}
		if order.OrderStatus != orderStatusUnpaid {
			return xerr.ErrOrderStatusInvalid
		}

		if _, err := svcCtx.ProgramRpc.ReleaseSeatFreeze(txCtx, &programrpc.ReleaseSeatFreezeReq{
			FreezeToken:   order.FreezeToken,
			ReleaseReason: releaseReason,
		}); err != nil {
			return err
		}

		cancelTime := time.Now()
		if err := svcCtx.DOrderModel.UpdateCancelStatus(txCtx, session, order.OrderNumber, cancelTime); err != nil {
			return err
		}
		if err := svcCtx.DOrderTicketUserModel.UpdateCancelStatusByOrderNumber(txCtx, session, order.OrderNumber, cancelTime); err != nil {
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

func buildFreezeRequestNo() string {
	return fmt.Sprintf("order-%d", xid.New())
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
	return svcCtx.SqlConn.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		order, err := svcCtx.DOrderModel.FindOneByOrderNumberForUpdate(txCtx, session, orderNumber)
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
		if err := svcCtx.DOrderModel.UpdateRefundStatus(txCtx, session, orderNumber, refundTime); err != nil {
			return err
		}
		if err := svcCtx.DOrderTicketUserModel.UpdateRefundStatusByOrderNumber(txCtx, session, orderNumber, refundTime); err != nil {
			return err
		}

		return nil
	})
}
