package programcache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/pb"
)

const programDateTimeLayout = "2006-01-02 15:04:05"

type DetailLoader struct {
	programModel          model.DProgramModel
	programShowTimeModel  model.DProgramShowTimeModel
	programGroupModel     model.DProgramGroupModel
	categorySnapshotCache *CategorySnapshotCache
	ticketCategoryModel   model.DTicketCategoryModel
}

type DetailLoaderDeps struct {
	ProgramModel          model.DProgramModel
	ProgramShowTimeModel  model.DProgramShowTimeModel
	ProgramGroupModel     model.DProgramGroupModel
	CategorySnapshotCache *CategorySnapshotCache
	TicketCategoryModel   model.DTicketCategoryModel
}

type programGroupJSONItem struct {
	ProgramID  int64  `json:"programId"`
	AreaID     int64  `json:"areaId"`
	AreaIDName string `json:"areaIdName"`
}

func NewDetailLoader(deps DetailLoaderDeps) *DetailLoader {
	return &DetailLoader{
		programModel:          deps.ProgramModel,
		programShowTimeModel:  deps.ProgramShowTimeModel,
		programGroupModel:     deps.ProgramGroupModel,
		categorySnapshotCache: deps.CategorySnapshotCache,
		ticketCategoryModel:   deps.TicketCategoryModel,
	}
}

func (l *DetailLoader) Load(ctx context.Context, programID int64) (*pb.ProgramDetailViewInfo, error) {
	program, err := l.programModel.FindOne(ctx, programID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrProgramNotFound
		}
		return nil, err
	}

	firstShowTime, err := l.programShowTimeModel.FindFirstByProgramId(ctx, program.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrProgramNotFound
		}
		return nil, err
	}

	categories, err := l.categorySnapshotCache.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	categoryMap := buildCategoryMap(categories)

	group, err := l.programGroupModel.FindOne(ctx, program.ProgramGroupId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrProgramNotFound
		}
		return nil, err
	}

	groupInfo, err := parseProgramGroupJSON(group)
	if err != nil {
		return nil, err
	}

	ticketCategories, err := l.ticketCategoryModel.FindByProgramId(ctx, program.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			ticketCategories = []*model.DTicketCategory{}
		} else {
			return nil, err
		}
	}

	return toProgramDetailViewInfo(program, firstShowTime, groupInfo, categoryMap, ticketCategories), nil
}

func buildCategoryMap(categories []*model.DProgramCategory) map[int64]*model.DProgramCategory {
	categoryMap := make(map[int64]*model.DProgramCategory, len(categories))
	for _, category := range categories {
		categoryMap[category.Id] = category
	}

	return categoryMap
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

func categoryName(category *model.DProgramCategory) string {
	if category == nil {
		return ""
	}

	return category.Name
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
