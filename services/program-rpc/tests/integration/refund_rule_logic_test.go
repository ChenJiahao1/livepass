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
	t.Run("permitRefund=0 rejects", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 54001
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ProgramGroupID:          64001,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                time.Now().Add(48 * time.Hour).Format(refundRuleTestDateTimeLayout),
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			PermitRefund:            0,
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ProgramId:     programID,
			OrderShowTime: time.Now().Add(48 * time.Hour).Format(refundRuleTestDateTimeLayout),
			OrderAmount:   598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if resp.AllowRefund {
			t.Fatalf("expected refund rejected, got %+v", resp)
		}
		if resp.RejectReason == "" {
			t.Fatalf("expected reject reason, got %+v", resp)
		}
	})

	t.Run("permitRefund=2 returns full refund", func(t *testing.T) {
		svcCtx := newProgramTestServiceContext(t)
		resetProgramDomainState(t)

		const programID int64 = 54002
		showTime := time.Now().Add(48 * time.Hour).Format(refundRuleTestDateTimeLayout)
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
			ProgramGroupID:          64002,
			ParentProgramCategoryID: 1,
			ProgramCategoryID:       11,
			AreaID:                  1,
			ShowTime:                showTime,
			ShowDayTime:             "2026-12-31 00:00:00",
			ShowWeekTime:            "周四",
			PermitRefund:            2,
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ProgramId:     programID,
			OrderShowTime: showTime,
			OrderAmount:   598,
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
		resetProgramDomainState(t)

		const programID int64 = 54003
		showTime := time.Now().Add(25 * time.Hour).Format(refundRuleTestDateTimeLayout)
		seedProgramFixtures(t, svcCtx, programFixture{
			ProgramID:               programID,
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
		})

		l := logicpkg.NewEvaluateRefundRuleLogic(context.Background(), svcCtx)
		resp, err := l.EvaluateRefundRule(&pb.EvaluateRefundRuleReq{
			ProgramId:     programID,
			OrderShowTime: showTime,
			OrderAmount:   598,
		})
		if err != nil {
			t.Fatalf("EvaluateRefundRule returned error: %v", err)
		}
		if !resp.AllowRefund || resp.RefundPercent != 80 || resp.RefundAmount != 478 {
			t.Fatalf("unexpected staged refund response: %+v", resp)
		}
	})
}
