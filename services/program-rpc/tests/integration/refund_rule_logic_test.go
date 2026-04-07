package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/program-rpc/internal/logic"
	"damai-go/services/program-rpc/pb"
)

const refundRuleTestDateTimeLayout = "2006-01-02 15:04:05"

func TestEvaluateRefundRule(t *testing.T) {
	resetProgramDomainState(t)

	t.Run("permitRefund=0 prefers refundExplain as reject reason", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)

		const programID int64 = 54001
		const showTimeID int64 = 64001
		const rejectReason = "当前场次不支持退票"
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ShowTimeID:              showTimeID,
			ProgramGroupID:          64001,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                time.Now().Add(48 * time.Hour).Format(refundRuleTestDateTimeLayout),
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			PermitRefund:            0,
			RefundExplain:           rejectReason,
			TicketCategories: []ticketCategoryFixture{
				{ID: 164001, ShowTimeID: showTimeID, Introduce: "普通票", Price: 299, TotalNumber: 100, RemainNumber: 100},
			},
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ShowTimeId:  showTimeID,
			OrderAmount: 598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if resp.AllowRefund {
			t.Fatalf("expected refund rejected, got %+v", resp)
		}
		if resp.RejectReason != rejectReason {
			t.Fatalf("expected reject reason %q, got %+v", rejectReason, resp)
		}
	})

	t.Run("permitRefund=2 returns full refund", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)

		const programID int64 = 54002
		const showTimeID int64 = 64002
		showTime := time.Now().Add(48 * time.Hour).Format(refundRuleTestDateTimeLayout)
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ShowTimeID:              showTimeID,
			ProgramGroupID:          64002,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                showTime,
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			PermitRefund:            2,
			TicketCategories: []ticketCategoryFixture{
				{ID: 164002, ShowTimeID: showTimeID, Introduce: "普通票", Price: 299, TotalNumber: 100, RemainNumber: 100},
			},
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ShowTimeId:  showTimeID,
			OrderAmount: 598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if !resp.AllowRefund || resp.RefundPercent != 100 || resp.RefundAmount != 598 {
			t.Fatalf("unexpected full refund response: %+v", resp)
		}
	})

	t.Run("permitRefund=1 matches staged rule", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)

		const programID int64 = 54003
		const showTimeID int64 = 64003
		showTime := time.Now().Add(25 * time.Hour).Format(refundRuleTestDateTimeLayout)
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ShowTimeID:              showTimeID,
			ProgramGroupID:          64003,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                showTime,
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			PermitRefund:            1,
			RefundExplain:           "支持按时间阶梯退款",
			RefundRuleJSON:          `{"version":1,"stages":[{"beforeMinutes":10080,"refundPercent":100},{"beforeMinutes":1440,"refundPercent":80},{"beforeMinutes":120,"refundPercent":50}]}`,
			TicketCategories: []ticketCategoryFixture{
				{ID: 164003, ShowTimeID: showTimeID, Introduce: "普通票", Price: 299, TotalNumber: 100, RemainNumber: 100},
			},
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ShowTimeId:  showTimeID,
			OrderAmount: 598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if !resp.AllowRefund || resp.RefundPercent != 80 || resp.RefundAmount != 478 {
			t.Fatalf("unexpected staged refund response: %+v", resp)
		}
	})

	t.Run("permitRefund=1 no-match prefers refundTicketRule/refundExplain copy", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)

		const programID int64 = 54004
		const showTimeID int64 = 64004
		const refundTicketRule = "演出开始前 120 分钟外可退"
		const refundExplain = "请按退票规则办理"
		const rejectReason = "演出开始前 120 分钟外可退；请按退票规则办理"
		showTime := time.Now().Add(90 * time.Minute).Format(refundRuleTestDateTimeLayout)
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ShowTimeID:              showTimeID,
			ProgramGroupID:          64004,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                showTime,
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			PermitRefund:            1,
			RefundTicketRule:        refundTicketRule,
			RefundExplain:           refundExplain,
			RefundRuleJSON:          `{"version":1,"stages":[{"beforeMinutes":120,"refundPercent":50}]}`,
			TicketCategories: []ticketCategoryFixture{
				{ID: 164004, ShowTimeID: showTimeID, Introduce: "普通票", Price: 299, TotalNumber: 100, RemainNumber: 100},
			},
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ShowTimeId:  showTimeID,
			OrderAmount: 598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if resp.AllowRefund {
			t.Fatalf("expected refund rejected, got %+v", resp)
		}
		if resp.RejectReason != rejectReason {
			t.Fatalf("expected reject reason %q, got %+v", rejectReason, resp)
		}
	})

	t.Run("rush sale window blocks refund with fixed reject copy", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)

		const programID int64 = 54005
		const showTimeID int64 = 64005
		showTime := time.Now().Add(48 * time.Hour).Format(refundRuleTestDateTimeLayout)
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ShowTimeID:              showTimeID,
			ProgramGroupID:          64005,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                showTime,
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			RushSaleOpenTime:        time.Now().Add(-time.Minute).Format(refundRuleTestDateTimeLayout),
			RushSaleEndTime:         time.Now().Add(time.Minute).Format(refundRuleTestDateTimeLayout),
			PermitRefund:            2,
			TicketCategories: []ticketCategoryFixture{
				{ID: 164005, ShowTimeID: showTimeID, Introduce: "普通票", Price: 299, TotalNumber: 100, RemainNumber: 100},
			},
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ShowTimeId:  showTimeID,
			OrderAmount: 598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if resp.AllowRefund {
			t.Fatalf("expected rush sale refund blocked, got %+v", resp)
		}
		if resp.RejectReason != "秒杀活动进行中，暂不支持退票" {
			t.Fatalf("unexpected reject reason: %+v", resp)
		}
	})
}
