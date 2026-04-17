// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/services/program-api/internal/svc"
	"livepass/services/program-api/internal/types"
	"livepass/services/program-rpc/programrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProgramLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProgramLogic {
	return &UpdateProgramLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProgramLogic) UpdateProgram(req *types.ProgramUpdateReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.ProgramRpc.UpdateProgram(l.ctx, &programrpc.UpdateProgramReq{
		Id:                              req.ID,
		ProgramGroupId:                  req.ProgramGroupID,
		Prime:                           req.Prime,
		AreaId:                          req.AreaID,
		ProgramCategoryId:               req.ProgramCategoryID,
		ParentProgramCategoryId:         req.ParentProgramCategoryID,
		Title:                           req.Title,
		Actor:                           req.Actor,
		Place:                           req.Place,
		ItemPicture:                     req.ItemPicture,
		PreSell:                         req.PreSell,
		PreSellInstruction:              req.PreSellInstruction,
		ImportantNotice:                 req.ImportantNotice,
		Detail:                          req.Detail,
		PerOrderLimitPurchaseCount:      req.PerOrderLimitPurchaseCount,
		PerAccountLimitPurchaseCount:    req.PerAccountLimitPurchaseCount,
		RefundTicketRule:                req.RefundTicketRule,
		DeliveryInstruction:             req.DeliveryInstruction,
		EntryRule:                       req.EntryRule,
		ChildPurchase:                   req.ChildPurchase,
		InvoiceSpecification:            req.InvoiceSpecification,
		RealTicketPurchaseRule:          req.RealTicketPurchaseRule,
		AbnormalOrderDescription:        req.AbnormalOrderDescription,
		KindReminder:                    req.KindReminder,
		PerformanceDuration:             req.PerformanceDuration,
		EntryTime:                       req.EntryTime,
		MinPerformanceCount:             req.MinPerformanceCount,
		MainActor:                       req.MainActor,
		MinPerformanceDuration:          req.MinPerformanceDuration,
		ProhibitedItem:                  req.ProhibitedItem,
		DepositSpecification:            req.DepositSpecification,
		TotalCount:                      req.TotalCount,
		PermitRefund:                    req.PermitRefund,
		RefundExplain:                   req.RefundExplain,
		RefundRuleJson:                  req.RefundRuleJSON,
		RelNameTicketEntrance:           req.RelNameTicketEntrance,
		RelNameTicketEntranceExplain:    req.RelNameTicketEntranceExplain,
		PermitChooseSeat:                req.PermitChooseSeat,
		ChooseSeatExplain:               req.ChooseSeatExplain,
		ElectronicDeliveryTicket:        req.ElectronicDeliveryTicket,
		ElectronicDeliveryTicketExplain: req.ElectronicDeliveryTicketExplain,
		ElectronicInvoice:               req.ElectronicInvoice,
		ElectronicInvoiceExplain:        req.ElectronicInvoiceExplain,
		HighHeat:                        req.HighHeat,
		ProgramStatus:                   req.ProgramStatus,
		IssueTime:                       req.IssueTime,
		RushSaleOpenTime:                req.RushSaleOpenTime,
		RushSaleEndTime:                 req.RushSaleEndTime,
		Status:                          req.Status,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
