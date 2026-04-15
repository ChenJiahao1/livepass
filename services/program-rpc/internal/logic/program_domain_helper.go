package logic

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	programDateTimeLayout       = "2006-01-02 15:04:05"
	rushSaleRefundBlockedReason = "秒杀活动进行中，暂不支持退票"
)

type ticketPriceRange struct {
	MinPrice int64
	MaxPrice int64
}

type programGroupJSONItem struct {
	ProgramID  int64  `json:"programId"`
	AreaID     int64  `json:"areaId"`
	AreaIDName string `json:"areaIdName"`
}

func validatePageProgramsReq(in *pb.PageProgramsReq) error {
	if in.GetTimeType() == 5 && (in.GetStartDateTime() == "" || in.GetEndDateTime() == "") {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return nil
}

func programNotFoundError() error {
	return status.Error(codes.NotFound, "program not found")
}

func buildCategoryMap(categories []*model.DProgramCategory) map[int64]*model.DProgramCategory {
	categoryMap := make(map[int64]*model.DProgramCategory, len(categories))
	for _, category := range categories {
		categoryMap[category.Id] = category
	}

	return categoryMap
}

func mapTicketPriceRange(aggregates []*model.TicketCategoryPriceAggregate) map[int64]ticketPriceRange {
	priceRangeMap := make(map[int64]ticketPriceRange, len(aggregates))
	for _, aggregate := range aggregates {
		priceRangeMap[aggregate.ProgramId] = ticketPriceRange{
			MinPrice: int64(aggregate.MinPrice),
			MaxPrice: int64(aggregate.MaxPrice),
		}
	}

	return priceRangeMap
}

func mapFirstShowTime(showTimes []*model.DProgramShowTime) map[int64]*model.DProgramShowTime {
	showTimeMap := make(map[int64]*model.DProgramShowTime, len(showTimes))
	for _, showTime := range showTimes {
		if _, ok := showTimeMap[showTime.ProgramId]; ok {
			continue
		}
		showTimeMap[showTime.ProgramId] = showTime
	}

	return showTimeMap
}

func parseProgramGroupJSON(group *model.DProgramGroup) (*pb.ProgramGroupInfo, error) {
	if group == nil {
		return nil, nil
	}

	items := make([]programGroupJSONItem, 0)
	if group.ProgramJson != "" {
		if err := json.Unmarshal([]byte(group.ProgramJson), &items); err != nil {
			return nil, err
		}
	}

	infoList := make([]*pb.ProgramSimpleInfo, 0, len(items))
	for _, item := range items {
		infoList = append(infoList, &pb.ProgramSimpleInfo{
			ProgramId:  item.ProgramID,
			AreaId:     item.AreaID,
			AreaIdName: item.AreaIDName,
		})
	}

	return &pb.ProgramGroupInfo{
		Id:                      group.Id,
		ProgramSimpleInfoVoList: infoList,
		RecentShowTime:          formatTime(group.RecentShowTime),
	}, nil
}

func toProgramListInfo(program *model.DProgram, firstShowTime *model.DProgramShowTime, categories map[int64]*model.DProgramCategory, priceRange ticketPriceRange) *pb.ProgramListInfo {
	return &pb.ProgramListInfo{
		Id:                        program.Id,
		Title:                     program.Title,
		Actor:                     nullStringValue(program.Actor),
		Place:                     nullStringValue(program.Place),
		ItemPicture:               nullStringValue(program.ItemPicture),
		AreaId:                    program.AreaId,
		AreaName:                  "",
		ProgramCategoryId:         program.ProgramCategoryId,
		ProgramCategoryName:       categoryName(categories[program.ProgramCategoryId]),
		ParentProgramCategoryId:   program.ParentProgramCategoryId,
		ParentProgramCategoryName: categoryName(categories[program.ParentProgramCategoryId]),
		ShowTime:                  formatProgramShowTime(firstShowTime),
		ShowDayTime:               formatProgramShowDayTime(firstShowTime),
		ShowWeekTime:              programShowWeekTime(firstShowTime),
		MinPrice:                  priceRange.MinPrice,
		MaxPrice:                  priceRange.MaxPrice,
	}
}

func toProgramDetailViewInfo(program *model.DProgram, firstShowTime *model.DProgramShowTime, groupInfo *pb.ProgramGroupInfo, categories map[int64]*model.DProgramCategory, ticketCategories []*model.DTicketCategory) *pb.ProgramDetailViewInfo {
	return &pb.ProgramDetailViewInfo{
		Id:                              program.Id,
		ProgramGroupId:                  program.ProgramGroupId,
		Prime:                           program.Prime,
		ProgramGroupVo:                  groupInfo,
		Title:                           program.Title,
		Actor:                           nullStringValue(program.Actor),
		Place:                           nullStringValue(program.Place),
		ItemPicture:                     nullStringValue(program.ItemPicture),
		PreSell:                         program.PreSell,
		PreSellInstruction:              nullStringValue(program.PreSellInstruction),
		ImportantNotice:                 nullStringValue(program.ImportantNotice),
		AreaId:                          program.AreaId,
		AreaName:                        "",
		ProgramCategoryId:               program.ProgramCategoryId,
		ProgramCategoryName:             categoryName(categories[program.ProgramCategoryId]),
		ParentProgramCategoryId:         program.ParentProgramCategoryId,
		ParentProgramCategoryName:       categoryName(categories[program.ParentProgramCategoryId]),
		Detail:                          program.Detail,
		PerOrderLimitPurchaseCount:      program.PerOrderLimitPurchaseCount,
		PerAccountLimitPurchaseCount:    program.PerAccountLimitPurchaseCount,
		RefundTicketRule:                nullStringValue(program.RefundTicketRule),
		DeliveryInstruction:             nullStringValue(program.DeliveryInstruction),
		EntryRule:                       nullStringValue(program.EntryRule),
		ChildPurchase:                   nullStringValue(program.ChildPurchase),
		InvoiceSpecification:            nullStringValue(program.InvoiceSpecification),
		RealTicketPurchaseRule:          nullStringValue(program.RealTicketPurchaseRule),
		AbnormalOrderDescription:        nullStringValue(program.AbnormalOrderDescription),
		KindReminder:                    nullStringValue(program.KindReminder),
		PerformanceDuration:             nullStringValue(program.PerformanceDuration),
		EntryTime:                       nullStringValue(program.EntryTime),
		MinPerformanceCount:             nullInt64Value(program.MinPerformanceCount),
		MainActor:                       nullStringValue(program.MainActor),
		MinPerformanceDuration:          nullStringValue(program.MinPerformanceDuration),
		ProhibitedItem:                  nullStringValue(program.ProhibitedItem),
		DepositSpecification:            nullStringValue(program.DepositSpecification),
		TotalCount:                      nullInt64Value(program.TotalCount),
		PermitRefund:                    program.PermitRefund,
		RefundExplain:                   nullStringValue(program.RefundExplain),
		RelNameTicketEntrance:           program.RelNameTicketEntrance,
		RelNameTicketEntranceExplain:    nullStringValue(program.RelNameTicketEntranceExplain),
		PermitChooseSeat:                program.PermitChooseSeat,
		ChooseSeatExplain:               nullStringValue(program.ChooseSeatExplain),
		ElectronicDeliveryTicket:        program.ElectronicDeliveryTicket,
		ElectronicDeliveryTicketExplain: nullStringValue(program.ElectronicDeliveryTicketExplain),
		ElectronicInvoice:               program.ElectronicInvoice,
		ElectronicInvoiceExplain:        nullStringValue(program.ElectronicInvoiceExplain),
		HighHeat:                        program.HighHeat,
		ProgramStatus:                   program.ProgramStatus,
		IssueTime:                       formatNullTime(program.IssueTime),
		RushSaleOpenTime:                formatNullTime(program.RushSaleOpenTime),
		RushSaleEndTime:                 formatNullTime(program.RushSaleEndTime),
		InventoryPreheatStatus:          program.InventoryPreheatStatus,
		ShowTime:                        formatProgramShowTime(firstShowTime),
		ShowDayTime:                     formatProgramShowDayTime(firstShowTime),
		ShowWeekTime:                    programShowWeekTime(firstShowTime),
		TicketCategoryVoList:            toTicketCategoryInfoList(ticketCategories),
	}
}

func toProgramPreorderInfo(program *model.DProgram, firstShowTime *model.DProgramShowTime, ticketCategories []*model.DTicketCategory, remainMap map[int64]int64) *pb.ProgramPreorderInfo {
	return &pb.ProgramPreorderInfo{
		ProgramId:                    program.Id,
		ShowTimeId:                   firstShowTime.Id,
		ProgramGroupId:               program.ProgramGroupId,
		Title:                        program.Title,
		Actor:                        nullStringValue(program.Actor),
		Place:                        nullStringValue(program.Place),
		ItemPicture:                  nullStringValue(program.ItemPicture),
		ShowTime:                     formatProgramShowTime(firstShowTime),
		ShowDayTime:                  formatProgramShowDayTime(firstShowTime),
		ShowWeekTime:                 programShowWeekTime(firstShowTime),
		RushSaleOpenTime:             formatNullTime(program.RushSaleOpenTime),
		RushSaleEndTime:              formatNullTime(program.RushSaleEndTime),
		PerOrderLimitPurchaseCount:   program.PerOrderLimitPurchaseCount,
		PerAccountLimitPurchaseCount: program.PerAccountLimitPurchaseCount,
		PermitChooseSeat:             program.PermitChooseSeat,
		ChooseSeatExplain:            nullStringValue(program.ChooseSeatExplain),
		TicketCategoryVoList:         toProgramPreorderTicketCategoryInfoList(ticketCategories, remainMap),
	}
}

func toTicketCategoryInfoList(ticketCategories []*model.DTicketCategory) []*pb.TicketCategoryInfo {
	list := make([]*pb.TicketCategoryInfo, 0, len(ticketCategories))
	for _, ticketCategory := range ticketCategories {
		list = append(list, &pb.TicketCategoryInfo{
			Id:        ticketCategory.Id,
			Introduce: ticketCategory.Introduce,
			Price:     int64(ticketCategory.Price),
		})
	}

	return list
}

func toProgramPreorderTicketCategoryInfoList(ticketCategories []*model.DTicketCategory, remainMap map[int64]int64) []*pb.ProgramPreorderTicketCategoryInfo {
	list := make([]*pb.ProgramPreorderTicketCategoryInfo, 0, len(ticketCategories))
	for _, ticketCategory := range ticketCategories {
		admissionQuota := remainMap[ticketCategory.Id]
		list = append(list, &pb.ProgramPreorderTicketCategoryInfo{
			Id:             ticketCategory.Id,
			Introduce:      ticketCategory.Introduce,
			Price:          int64(ticketCategory.Price),
			TotalNumber:    ticketCategory.TotalNumber,
			RemainNumber:   admissionQuota,
			AdmissionQuota: admissionQuota,
		})
	}

	return list
}

func toTicketCategoryDetailInfoList(ticketCategories []*model.DTicketCategory) []*pb.TicketCategoryDetailInfo {
	list := make([]*pb.TicketCategoryDetailInfo, 0, len(ticketCategories))
	for _, ticketCategory := range ticketCategories {
		list = append(list, &pb.TicketCategoryDetailInfo{
			ProgramId:    ticketCategory.ProgramId,
			Introduce:    ticketCategory.Introduce,
			Price:        int64(ticketCategory.Price),
			TotalNumber:  ticketCategory.TotalNumber,
			RemainNumber: ticketCategory.RemainNumber,
		})
	}

	return list
}

func mapSeatRemainAggregates(aggregates []*model.SeatRemainAggregate) map[int64]int64 {
	remainMap := make(map[int64]int64, len(aggregates))
	for _, aggregate := range aggregates {
		remainMap[aggregate.TicketCategoryId] = aggregate.RemainNumber
	}

	return remainMap
}

func homeListLimit(parentCategoryIDs []int64) int64 {
	if len(parentCategoryIDs) == 0 {
		return 7
	}

	return int64(len(parentCategoryIDs) * 7)
}

func orderedHomeParentCategoryIDs(parentCategoryIDs []int64, grouped map[int64][]*pb.ProgramListInfo) []int64 {
	if len(parentCategoryIDs) > 0 {
		return parentCategoryIDs
	}

	ids := make([]int64, 0, len(grouped))
	for id := range grouped {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	return ids
}

func categoryName(category *model.DProgramCategory) string {
	if category == nil {
		return ""
	}
	return category.Name
}

func programRefundDisabledReason(program *model.DProgram) string {
	if reason := nullStringValue(program.RefundExplain); reason != "" {
		return reason
	}

	return "program does not permit refund"
}

func programRefundNoMatchReason(program *model.DProgram, fallback string) string {
	parts := make([]string, 0, 2)
	if rule := nullStringValue(program.RefundTicketRule); rule != "" {
		parts = append(parts, rule)
	}
	if explain := nullStringValue(program.RefundExplain); explain != "" {
		parts = append(parts, explain)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "；")
	}

	return fallback
}

func isRefundBlockedDuringRushSale(program *model.DProgram, now time.Time) bool {
	if program == nil || !program.RushSaleOpenTime.Valid || !program.RushSaleEndTime.Valid {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}

	openAt := program.RushSaleOpenTime.Time
	endAt := program.RushSaleEndTime.Time
	if endAt.Before(openAt) {
		return false
	}

	return !now.Before(openAt) && !now.After(endAt)
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullInt64Value(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(programDateTimeLayout)
}

func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format(programDateTimeLayout)
}

func formatProgramShowTime(showTime *model.DProgramShowTime) string {
	if showTime == nil {
		return ""
	}
	return formatTime(showTime.ShowTime)
}

func formatProgramShowDayTime(showTime *model.DProgramShowTime) string {
	if showTime == nil {
		return ""
	}
	return formatNullTime(showTime.ShowDayTime)
}

func programShowWeekTime(showTime *model.DProgramShowTime) string {
	if showTime == nil {
		return ""
	}
	return showTime.ShowWeekTime
}
