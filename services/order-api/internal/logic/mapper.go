package logic

import (
	"damai-go/services/order-api/internal/types"
	"damai-go/services/order-rpc/orderrpc"
)

func mapCreatePurchaseTokenResp(resp *orderrpc.CreatePurchaseTokenResp) *types.CreatePurchaseTokenResp {
	if resp == nil {
		return &types.CreatePurchaseTokenResp{}
	}

	return &types.CreatePurchaseTokenResp{
		PurchaseToken: resp.PurchaseToken,
	}
}

func mapCreateOrderResp(resp *orderrpc.CreateOrderResp) *types.CreateOrderResp {
	if resp == nil {
		return &types.CreateOrderResp{}
	}

	return &types.CreateOrderResp{
		OrderNumber: resp.OrderNumber,
	}
}

func mapPollOrderResp(resp *orderrpc.PollOrderProgressResp) *types.PollOrderResp {
	if resp == nil {
		return &types.PollOrderResp{}
	}

	return &types.PollOrderResp{
		OrderNumber: resp.OrderNumber,
		OrderStatus: resp.OrderStatus,
		Done:        resp.Done,
	}
}

func mapListOrdersResp(resp *orderrpc.ListOrdersResp) *types.ListOrdersResp {
	if resp == nil {
		return &types.ListOrdersResp{}
	}

	return &types.ListOrdersResp{
		PageNum:   resp.PageNum,
		PageSize:  resp.PageSize,
		TotalSize: resp.TotalSize,
		List:      mapOrderListInfoList(resp.List),
	}
}

func mapOrderListInfoList(list []*orderrpc.OrderListInfo) []types.OrderListInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.OrderListInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.OrderListInfo{})
			continue
		}
		resp = append(resp, types.OrderListInfo{
			OrderNumber:        item.OrderNumber,
			ProgramID:          item.ProgramId,
			ProgramTitle:       item.ProgramTitle,
			ProgramItemPicture: item.ProgramItemPicture,
			ProgramPlace:       item.ProgramPlace,
			ProgramShowTime:    item.ProgramShowTime,
			TicketCount:        item.TicketCount,
			OrderPrice:         item.OrderPrice,
			OrderStatus:        item.OrderStatus,
			OrderExpireTime:    item.OrderExpireTime,
			CreateOrderTime:    item.CreateOrderTime,
			CancelOrderTime:    item.CancelOrderTime,
		})
	}

	return resp
}

func mapOrderDetailInfo(resp *orderrpc.OrderDetailInfo) *types.OrderDetailInfo {
	if resp == nil {
		return &types.OrderDetailInfo{}
	}

	return &types.OrderDetailInfo{
		OrderNumber:             resp.OrderNumber,
		ProgramID:               resp.ProgramId,
		ProgramTitle:            resp.ProgramTitle,
		ProgramItemPicture:      resp.ProgramItemPicture,
		ProgramPlace:            resp.ProgramPlace,
		ProgramShowTime:         resp.ProgramShowTime,
		ProgramPermitChooseSeat: resp.ProgramPermitChooseSeat,
		UserID:                  resp.UserId,
		DistributionMode:        resp.DistributionMode,
		TakeTicketMode:          resp.TakeTicketMode,
		TicketCount:             resp.TicketCount,
		OrderPrice:              resp.OrderPrice,
		OrderStatus:             resp.OrderStatus,
		FreezeToken:             resp.FreezeToken,
		OrderExpireTime:         resp.OrderExpireTime,
		CreateOrderTime:         resp.CreateOrderTime,
		CancelOrderTime:         resp.CancelOrderTime,
		OrderTicketInfoVoList:   mapOrderTicketInfoList(resp.OrderTicketInfoVoList),
	}
}

func mapOrderTicketInfoList(list []*orderrpc.OrderTicketInfo) []types.OrderTicketInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.OrderTicketInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.OrderTicketInfo{})
			continue
		}
		resp = append(resp, types.OrderTicketInfo{
			TicketUserID:       item.TicketUserId,
			TicketUserName:     item.TicketUserName,
			TicketUserIDNumber: item.TicketUserIdNumber,
			TicketCategoryID:   item.TicketCategoryId,
			TicketCategoryName: item.TicketCategoryName,
			TicketPrice:        item.TicketPrice,
			SeatID:             item.SeatId,
			SeatRow:            item.SeatRow,
			SeatCol:            item.SeatCol,
			SeatPrice:          item.SeatPrice,
		})
	}

	return resp
}

func mapPayOrderResp(resp *orderrpc.PayOrderResp) *types.PayOrderResp {
	if resp == nil {
		return &types.PayOrderResp{}
	}

	return &types.PayOrderResp{
		OrderNumber: resp.OrderNumber,
		OrderStatus: resp.OrderStatus,
		PayBillNo:   resp.PayBillNo,
		PayStatus:   resp.PayStatus,
		PayTime:     resp.PayTime,
	}
}

func mapPayCheckResp(resp *orderrpc.PayCheckResp) *types.PayCheckResp {
	if resp == nil {
		return &types.PayCheckResp{}
	}

	return &types.PayCheckResp{
		OrderNumber: resp.OrderNumber,
		OrderStatus: resp.OrderStatus,
		PayBillNo:   resp.PayBillNo,
		PayStatus:   resp.PayStatus,
		PayTime:     resp.PayTime,
	}
}

func mapRefundOrderResp(resp *orderrpc.RefundOrderResp) *types.RefundOrderResp {
	if resp == nil {
		return &types.RefundOrderResp{}
	}

	return &types.RefundOrderResp{
		OrderNumber:   resp.OrderNumber,
		OrderStatus:   resp.OrderStatus,
		RefundAmount:  resp.RefundAmount,
		RefundPercent: resp.RefundPercent,
		RefundBillNo:  resp.RefundBillNo,
		RefundTime:    resp.RefundTime,
	}
}
