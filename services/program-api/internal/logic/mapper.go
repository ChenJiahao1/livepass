package logic

import (
	"damai-go/services/program-api/internal/types"
	"damai-go/services/program-rpc/programrpc"
)

func mapProgramCategoryListResp(resp *programrpc.ProgramCategoryListResp) *types.ProgramCategoryListResp {
	if resp == nil {
		return &types.ProgramCategoryListResp{}
	}

	return &types.ProgramCategoryListResp{
		List: mapProgramCategoryInfoList(resp.List),
	}
}

func mapProgramCategoryInfoList(list []*programrpc.ProgramCategoryInfo) []types.ProgramCategoryInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.ProgramCategoryInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.ProgramCategoryInfo{})
			continue
		}
		resp = append(resp, types.ProgramCategoryInfo{
			ID:       item.Id,
			ParentID: item.ParentId,
			Name:     item.Name,
			Type:     item.Type,
		})
	}

	return resp
}

func mapProgramHomeListResp(resp *programrpc.ProgramHomeListResp) *types.ProgramHomeListResp {
	if resp == nil {
		return &types.ProgramHomeListResp{}
	}

	return &types.ProgramHomeListResp{
		Sections: mapProgramHomeSectionList(resp.Sections),
	}
}

func mapProgramHomeSectionList(list []*programrpc.ProgramHomeSection) []types.ProgramHomeSection {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.ProgramHomeSection, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.ProgramHomeSection{})
			continue
		}
		resp = append(resp, types.ProgramHomeSection{
			CategoryName:      item.CategoryName,
			CategoryID:        item.CategoryId,
			ProgramListVoList: mapProgramListInfoList(item.ProgramListVoList),
		})
	}

	return resp
}

func mapProgramPageResp(resp *programrpc.ProgramPageResp) *types.ProgramPageResp {
	if resp == nil {
		return &types.ProgramPageResp{}
	}

	return &types.ProgramPageResp{
		PageNum:   resp.PageNum,
		PageSize:  resp.PageSize,
		TotalSize: resp.TotalSize,
		List:      mapProgramListInfoList(resp.List),
	}
}

func mapProgramListInfoList(list []*programrpc.ProgramListInfo) []types.ProgramListInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.ProgramListInfo, 0, len(list))
	for _, item := range list {
		resp = append(resp, mapProgramListInfo(item))
	}

	return resp
}

func mapProgramListInfo(item *programrpc.ProgramListInfo) types.ProgramListInfo {
	if item == nil {
		return types.ProgramListInfo{}
	}

	return types.ProgramListInfo{
		ID:                        item.Id,
		Title:                     item.Title,
		Actor:                     item.Actor,
		Place:                     item.Place,
		ItemPicture:               item.ItemPicture,
		AreaID:                    item.AreaId,
		AreaName:                  item.AreaName,
		ProgramCategoryID:         item.ProgramCategoryId,
		ProgramCategoryName:       item.ProgramCategoryName,
		ParentProgramCategoryID:   item.ParentProgramCategoryId,
		ParentProgramCategoryName: item.ParentProgramCategoryName,
		ShowTime:                  item.ShowTime,
		ShowDayTime:               item.ShowDayTime,
		ShowWeekTime:              item.ShowWeekTime,
		MinPrice:                  item.MinPrice,
		MaxPrice:                  item.MaxPrice,
		EsID:                      item.EsId,
	}
}

func mapProgramDetailInfo(resp *programrpc.ProgramDetailInfo) *types.ProgramDetailInfo {
	if resp == nil {
		return &types.ProgramDetailInfo{}
	}

	return &types.ProgramDetailInfo{
		ID:                              resp.Id,
		ProgramGroupID:                  resp.ProgramGroupId,
		Prime:                           resp.Prime,
		ProgramGroupVo:                  mapProgramGroupInfo(resp.ProgramGroupVo),
		Title:                           resp.Title,
		Actor:                           resp.Actor,
		Place:                           resp.Place,
		ItemPicture:                     resp.ItemPicture,
		PreSell:                         resp.PreSell,
		PreSellInstruction:              resp.PreSellInstruction,
		ImportantNotice:                 resp.ImportantNotice,
		AreaID:                          resp.AreaId,
		AreaName:                        resp.AreaName,
		ProgramCategoryID:               resp.ProgramCategoryId,
		ProgramCategoryName:             resp.ProgramCategoryName,
		ParentProgramCategoryID:         resp.ParentProgramCategoryId,
		ParentProgramCategoryName:       resp.ParentProgramCategoryName,
		Detail:                          resp.Detail,
		PerOrderLimitPurchaseCount:      resp.PerOrderLimitPurchaseCount,
		PerAccountLimitPurchaseCount:    resp.PerAccountLimitPurchaseCount,
		RefundTicketRule:                resp.RefundTicketRule,
		DeliveryInstruction:             resp.DeliveryInstruction,
		EntryRule:                       resp.EntryRule,
		ChildPurchase:                   resp.ChildPurchase,
		InvoiceSpecification:            resp.InvoiceSpecification,
		RealTicketPurchaseRule:          resp.RealTicketPurchaseRule,
		AbnormalOrderDescription:        resp.AbnormalOrderDescription,
		KindReminder:                    resp.KindReminder,
		PerformanceDuration:             resp.PerformanceDuration,
		EntryTime:                       resp.EntryTime,
		MinPerformanceCount:             resp.MinPerformanceCount,
		MainActor:                       resp.MainActor,
		MinPerformanceDuration:          resp.MinPerformanceDuration,
		ProhibitedItem:                  resp.ProhibitedItem,
		DepositSpecification:            resp.DepositSpecification,
		TotalCount:                      resp.TotalCount,
		PermitRefund:                    resp.PermitRefund,
		RefundExplain:                   resp.RefundExplain,
		RelNameTicketEntrance:           resp.RelNameTicketEntrance,
		RelNameTicketEntranceExplain:    resp.RelNameTicketEntranceExplain,
		PermitChooseSeat:                resp.PermitChooseSeat,
		ChooseSeatExplain:               resp.ChooseSeatExplain,
		ElectronicDeliveryTicket:        resp.ElectronicDeliveryTicket,
		ElectronicDeliveryTicketExplain: resp.ElectronicDeliveryTicketExplain,
		ElectronicInvoice:               resp.ElectronicInvoice,
		ElectronicInvoiceExplain:        resp.ElectronicInvoiceExplain,
		HighHeat:                        resp.HighHeat,
		ProgramStatus:                   resp.ProgramStatus,
		IssueTime:                       resp.IssueTime,
		ShowTime:                        resp.ShowTime,
		ShowDayTime:                     resp.ShowDayTime,
		ShowWeekTime:                    resp.ShowWeekTime,
		TicketCategoryVoList:            mapTicketCategoryInfoList(resp.TicketCategoryVoList),
	}
}

func mapProgramPreorderInfo(resp *programrpc.ProgramPreorderInfo) *types.ProgramPreorderInfo {
	if resp == nil {
		return &types.ProgramPreorderInfo{}
	}

	return &types.ProgramPreorderInfo{
		ID:                           resp.Id,
		ProgramGroupID:               resp.ProgramGroupId,
		Title:                        resp.Title,
		Actor:                        resp.Actor,
		Place:                        resp.Place,
		ItemPicture:                  resp.ItemPicture,
		ShowTime:                     resp.ShowTime,
		ShowDayTime:                  resp.ShowDayTime,
		ShowWeekTime:                 resp.ShowWeekTime,
		PerOrderLimitPurchaseCount:   resp.PerOrderLimitPurchaseCount,
		PerAccountLimitPurchaseCount: resp.PerAccountLimitPurchaseCount,
		PermitChooseSeat:             resp.PermitChooseSeat,
		ChooseSeatExplain:            resp.ChooseSeatExplain,
		TicketCategoryVoList:         mapProgramPreorderTicketCategoryInfoList(resp.TicketCategoryVoList),
	}
}

func mapProgramGroupInfo(resp *programrpc.ProgramGroupInfo) types.ProgramGroupInfo {
	if resp == nil {
		return types.ProgramGroupInfo{}
	}

	return types.ProgramGroupInfo{
		ID:                      resp.Id,
		ProgramSimpleInfoVoList: mapProgramSimpleInfoList(resp.ProgramSimpleInfoVoList),
		RecentShowTime:          resp.RecentShowTime,
	}
}

func mapProgramSimpleInfoList(list []*programrpc.ProgramSimpleInfo) []types.ProgramSimpleInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.ProgramSimpleInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.ProgramSimpleInfo{})
			continue
		}
		resp = append(resp, types.ProgramSimpleInfo{
			ProgramID:  item.ProgramId,
			AreaID:     item.AreaId,
			AreaIDName: item.AreaIdName,
		})
	}

	return resp
}

func mapProgramPreorderTicketCategoryInfoList(list []*programrpc.ProgramPreorderTicketCategoryInfo) []types.ProgramPreorderTicketCategoryInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.ProgramPreorderTicketCategoryInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.ProgramPreorderTicketCategoryInfo{})
			continue
		}
		resp = append(resp, types.ProgramPreorderTicketCategoryInfo{
			ID:           item.Id,
			Introduce:    item.Introduce,
			Price:        item.Price,
			TotalNumber:  item.TotalNumber,
			RemainNumber: item.RemainNumber,
		})
	}

	return resp
}

func mapTicketCategoryInfoList(list []*programrpc.TicketCategoryInfo) []types.TicketCategoryInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.TicketCategoryInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.TicketCategoryInfo{})
			continue
		}
		resp = append(resp, types.TicketCategoryInfo{
			ID:        item.Id,
			Introduce: item.Introduce,
			Price:     item.Price,
		})
	}

	return resp
}

func mapTicketCategoryDetailListResp(resp *programrpc.TicketCategoryDetailListResp) *types.TicketCategoryDetailListResp {
	if resp == nil {
		return &types.TicketCategoryDetailListResp{}
	}

	return &types.TicketCategoryDetailListResp{
		List: mapTicketCategoryDetailInfoList(resp.List),
	}
}

func mapTicketCategoryDetailInfoList(list []*programrpc.TicketCategoryDetailInfo) []types.TicketCategoryDetailInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.TicketCategoryDetailInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.TicketCategoryDetailInfo{})
			continue
		}
		resp = append(resp, types.TicketCategoryDetailInfo{
			ProgramID:    item.ProgramId,
			Introduce:    item.Introduce,
			Price:        item.Price,
			TotalNumber:  item.TotalNumber,
			RemainNumber: item.RemainNumber,
		})
	}

	return resp
}

func mapFreezeSeatsResp(resp *programrpc.AutoAssignAndFreezeSeatsResp) *types.FreezeSeatsResp {
	if resp == nil {
		return &types.FreezeSeatsResp{}
	}

	return &types.FreezeSeatsResp{
		FreezeToken: resp.FreezeToken,
		ExpireTime:  resp.ExpireTime,
		Seats:       mapSeatInfoList(resp.Seats),
	}
}

func mapSeatInfoList(list []*programrpc.SeatInfo) []types.SeatInfo {
	if len(list) == 0 {
		return nil
	}

	resp := make([]types.SeatInfo, 0, len(list))
	for _, item := range list {
		if item == nil {
			resp = append(resp, types.SeatInfo{})
			continue
		}
		resp = append(resp, types.SeatInfo{
			SeatID:           item.SeatId,
			TicketCategoryID: item.TicketCategoryId,
			RowCode:          item.RowCode,
			ColCode:          item.ColCode,
			Price:            item.Price,
		})
	}

	return resp
}
