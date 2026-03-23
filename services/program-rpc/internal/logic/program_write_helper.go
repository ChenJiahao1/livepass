package logic

import (
	"database/sql"
	"strings"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type programWriteValues struct {
	id                              int64
	programGroupId                  int64
	prime                           int64
	areaId                          int64
	programCategoryId               int64
	parentProgramCategoryId         int64
	title                           string
	actor                           string
	place                           string
	itemPicture                     string
	preSell                         int64
	preSellInstruction              string
	importantNotice                 string
	detail                          string
	perOrderLimitPurchaseCount      int64
	perAccountLimitPurchaseCount    int64
	refundTicketRule                string
	deliveryInstruction             string
	entryRule                       string
	childPurchase                   string
	invoiceSpecification            string
	realTicketPurchaseRule          string
	abnormalOrderDescription        string
	kindReminder                    string
	performanceDuration             string
	entryTime                       string
	minPerformanceCount             int64
	mainActor                       string
	minPerformanceDuration          string
	prohibitedItem                  string
	depositSpecification            string
	totalCount                      int64
	permitRefund                    int64
	refundExplain                   string
	refundRuleJson                  string
	relNameTicketEntrance           int64
	relNameTicketEntranceExplain    string
	permitChooseSeat                int64
	chooseSeatExplain               string
	electronicDeliveryTicket        int64
	electronicDeliveryTicketExplain string
	electronicInvoice               int64
	electronicInvoiceExplain        string
	highHeat                        int64
	programStatus                   int64
	issueTime                       string
	status                          int64
}

func newCreateProgramValues(in *pb.CreateProgramReq) programWriteValues {
	return programWriteValues{
		programGroupId:                  in.GetProgramGroupId(),
		prime:                           in.GetPrime(),
		areaId:                          in.GetAreaId(),
		programCategoryId:               in.GetProgramCategoryId(),
		parentProgramCategoryId:         in.GetParentProgramCategoryId(),
		title:                           in.GetTitle(),
		actor:                           in.GetActor(),
		place:                           in.GetPlace(),
		itemPicture:                     in.GetItemPicture(),
		preSell:                         in.GetPreSell(),
		preSellInstruction:              in.GetPreSellInstruction(),
		importantNotice:                 in.GetImportantNotice(),
		detail:                          in.GetDetail(),
		perOrderLimitPurchaseCount:      in.GetPerOrderLimitPurchaseCount(),
		perAccountLimitPurchaseCount:    in.GetPerAccountLimitPurchaseCount(),
		refundTicketRule:                in.GetRefundTicketRule(),
		deliveryInstruction:             in.GetDeliveryInstruction(),
		entryRule:                       in.GetEntryRule(),
		childPurchase:                   in.GetChildPurchase(),
		invoiceSpecification:            in.GetInvoiceSpecification(),
		realTicketPurchaseRule:          in.GetRealTicketPurchaseRule(),
		abnormalOrderDescription:        in.GetAbnormalOrderDescription(),
		kindReminder:                    in.GetKindReminder(),
		performanceDuration:             in.GetPerformanceDuration(),
		entryTime:                       in.GetEntryTime(),
		minPerformanceCount:             in.GetMinPerformanceCount(),
		mainActor:                       in.GetMainActor(),
		minPerformanceDuration:          in.GetMinPerformanceDuration(),
		prohibitedItem:                  in.GetProhibitedItem(),
		depositSpecification:            in.GetDepositSpecification(),
		totalCount:                      in.GetTotalCount(),
		permitRefund:                    in.GetPermitRefund(),
		refundExplain:                   in.GetRefundExplain(),
		refundRuleJson:                  in.GetRefundRuleJson(),
		relNameTicketEntrance:           in.GetRelNameTicketEntrance(),
		relNameTicketEntranceExplain:    in.GetRelNameTicketEntranceExplain(),
		permitChooseSeat:                in.GetPermitChooseSeat(),
		chooseSeatExplain:               in.GetChooseSeatExplain(),
		electronicDeliveryTicket:        in.GetElectronicDeliveryTicket(),
		electronicDeliveryTicketExplain: in.GetElectronicDeliveryTicketExplain(),
		electronicInvoice:               in.GetElectronicInvoice(),
		electronicInvoiceExplain:        in.GetElectronicInvoiceExplain(),
		highHeat:                        in.GetHighHeat(),
		programStatus:                   in.GetProgramStatus(),
		issueTime:                       in.GetIssueTime(),
		status:                          in.GetStatus(),
	}
}

func newUpdateProgramValues(in *pb.UpdateProgramReq) programWriteValues {
	values := newCreateProgramValues(&pb.CreateProgramReq{
		ProgramGroupId:                  in.GetProgramGroupId(),
		Prime:                           in.GetPrime(),
		AreaId:                          in.GetAreaId(),
		ProgramCategoryId:               in.GetProgramCategoryId(),
		ParentProgramCategoryId:         in.GetParentProgramCategoryId(),
		Title:                           in.GetTitle(),
		Actor:                           in.GetActor(),
		Place:                           in.GetPlace(),
		ItemPicture:                     in.GetItemPicture(),
		PreSell:                         in.GetPreSell(),
		PreSellInstruction:              in.GetPreSellInstruction(),
		ImportantNotice:                 in.GetImportantNotice(),
		Detail:                          in.GetDetail(),
		PerOrderLimitPurchaseCount:      in.GetPerOrderLimitPurchaseCount(),
		PerAccountLimitPurchaseCount:    in.GetPerAccountLimitPurchaseCount(),
		RefundTicketRule:                in.GetRefundTicketRule(),
		DeliveryInstruction:             in.GetDeliveryInstruction(),
		EntryRule:                       in.GetEntryRule(),
		ChildPurchase:                   in.GetChildPurchase(),
		InvoiceSpecification:            in.GetInvoiceSpecification(),
		RealTicketPurchaseRule:          in.GetRealTicketPurchaseRule(),
		AbnormalOrderDescription:        in.GetAbnormalOrderDescription(),
		KindReminder:                    in.GetKindReminder(),
		PerformanceDuration:             in.GetPerformanceDuration(),
		EntryTime:                       in.GetEntryTime(),
		MinPerformanceCount:             in.GetMinPerformanceCount(),
		MainActor:                       in.GetMainActor(),
		MinPerformanceDuration:          in.GetMinPerformanceDuration(),
		ProhibitedItem:                  in.GetProhibitedItem(),
		DepositSpecification:            in.GetDepositSpecification(),
		TotalCount:                      in.GetTotalCount(),
		PermitRefund:                    in.GetPermitRefund(),
		RefundExplain:                   in.GetRefundExplain(),
		RefundRuleJson:                  in.GetRefundRuleJson(),
		RelNameTicketEntrance:           in.GetRelNameTicketEntrance(),
		RelNameTicketEntranceExplain:    in.GetRelNameTicketEntranceExplain(),
		PermitChooseSeat:                in.GetPermitChooseSeat(),
		ChooseSeatExplain:               in.GetChooseSeatExplain(),
		ElectronicDeliveryTicket:        in.GetElectronicDeliveryTicket(),
		ElectronicDeliveryTicketExplain: in.GetElectronicDeliveryTicketExplain(),
		ElectronicInvoice:               in.GetElectronicInvoice(),
		ElectronicInvoiceExplain:        in.GetElectronicInvoiceExplain(),
		HighHeat:                        in.GetHighHeat(),
		ProgramStatus:                   in.GetProgramStatus(),
		IssueTime:                       in.GetIssueTime(),
		Status:                          in.GetStatus(),
	})
	values.id = in.GetId()

	return values
}

func validateProgramWriteValues(values programWriteValues, requireID bool) error {
	if requireID && values.id <= 0 {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if values.programGroupId <= 0 ||
		values.areaId <= 0 ||
		values.programCategoryId <= 0 ||
		values.parentProgramCategoryId <= 0 ||
		strings.TrimSpace(values.title) == "" ||
		strings.TrimSpace(values.detail) == "" {
		return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if values.issueTime != "" {
		if _, err := time.ParseInLocation(programDateTimeLayout, values.issueTime, time.Local); err != nil {
			return status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
		}
	}

	return nil
}

func buildProgramModel(values programWriteValues, now time.Time) (*model.DProgram, error) {
	issueTime, err := parseNullTime(values.issueTime)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	return &model.DProgram{
		Id:                              values.id,
		ProgramGroupId:                  values.programGroupId,
		Prime:                           values.prime,
		AreaId:                          values.areaId,
		ProgramCategoryId:               values.programCategoryId,
		ParentProgramCategoryId:         values.parentProgramCategoryId,
		Title:                           values.title,
		Actor:                           nullString(values.actor),
		Place:                           nullString(values.place),
		ItemPicture:                     nullString(values.itemPicture),
		PreSell:                         values.preSell,
		PreSellInstruction:              nullString(values.preSellInstruction),
		ImportantNotice:                 nullString(values.importantNotice),
		Detail:                          values.detail,
		PerOrderLimitPurchaseCount:      values.perOrderLimitPurchaseCount,
		PerAccountLimitPurchaseCount:    values.perAccountLimitPurchaseCount,
		RefundTicketRule:                nullString(values.refundTicketRule),
		DeliveryInstruction:             nullString(values.deliveryInstruction),
		EntryRule:                       nullString(values.entryRule),
		ChildPurchase:                   nullString(values.childPurchase),
		InvoiceSpecification:            nullString(values.invoiceSpecification),
		RealTicketPurchaseRule:          nullString(values.realTicketPurchaseRule),
		AbnormalOrderDescription:        nullString(values.abnormalOrderDescription),
		KindReminder:                    nullString(values.kindReminder),
		PerformanceDuration:             nullString(values.performanceDuration),
		EntryTime:                       nullString(values.entryTime),
		MinPerformanceCount:             nullInt64(values.minPerformanceCount),
		MainActor:                       nullString(values.mainActor),
		MinPerformanceDuration:          nullString(values.minPerformanceDuration),
		ProhibitedItem:                  nullString(values.prohibitedItem),
		DepositSpecification:            nullString(values.depositSpecification),
		TotalCount:                      nullInt64(values.totalCount),
		PermitRefund:                    values.permitRefund,
		RefundExplain:                   nullString(values.refundExplain),
		RefundRuleJson:                  nullString(values.refundRuleJson),
		RelNameTicketEntrance:           values.relNameTicketEntrance,
		RelNameTicketEntranceExplain:    nullString(values.relNameTicketEntranceExplain),
		PermitChooseSeat:                values.permitChooseSeat,
		ChooseSeatExplain:               nullString(values.chooseSeatExplain),
		ElectronicDeliveryTicket:        values.electronicDeliveryTicket,
		ElectronicDeliveryTicketExplain: nullString(values.electronicDeliveryTicketExplain),
		ElectronicInvoice:               values.electronicInvoice,
		ElectronicInvoiceExplain:        nullString(values.electronicInvoiceExplain),
		HighHeat:                        values.highHeat,
		ProgramStatus:                   values.programStatus,
		IssueTime:                       issueTime,
		CreateTime:                      now,
		EditTime:                        now,
		Status:                          values.status,
	}, nil
}

func applyCreateProgramDefaults(values *programWriteValues) {
	if values.prime == 0 {
		values.prime = 1
	}
	if values.perOrderLimitPurchaseCount == 0 {
		values.perOrderLimitPurchaseCount = 6
	}
	if values.perAccountLimitPurchaseCount == 0 {
		values.perAccountLimitPurchaseCount = 6
	}
	if values.electronicDeliveryTicket == 0 {
		values.electronicDeliveryTicket = 1
	}
	if values.electronicInvoice == 0 {
		values.electronicInvoice = 1
	}
	if values.programStatus == 0 {
		values.programStatus = 1
	}
	if values.status == 0 {
		values.status = 1
	}
}

func parseNullTime(value string) (sql.NullTime, error) {
	if strings.TrimSpace(value) == "" {
		return sql.NullTime{}, nil
	}

	parsed, err := time.ParseInLocation(programDateTimeLayout, value, time.Local)
	if err != nil {
		return sql.NullTime{}, err
	}

	return sql.NullTime{Time: parsed, Valid: true}, nil
}

func nullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: value, Valid: true}
}

func nullInt64(value int64) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: value, Valid: true}
}

func programGroupNotFoundError() error {
	return status.Error(codes.NotFound, "program group not found")
}
