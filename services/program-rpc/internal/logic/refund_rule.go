package logic

import (
	"encoding/json"
	"errors"
	"sort"
	"time"
)

var errInvalidRefundRule = errors.New("invalid refund rule")

type refundRule struct {
	Version int `json:"version"`
	Stages  []struct {
		BeforeMinutes int64 `json:"beforeMinutes"`
		RefundPercent int64 `json:"refundPercent"`
	} `json:"stages"`
}

type refundRuleResult struct {
	AllowRefund   bool
	RefundPercent int64
	RefundAmount  int64
	RejectReason  string
	NoMatch       bool
}

func evaluateRefundRule(ruleJSON string, showTime, now time.Time, orderAmount int64) (refundRuleResult, error) {
	if ruleJSON == "" || showTime.IsZero() || now.IsZero() || orderAmount <= 0 {
		return refundRuleResult{}, errInvalidRefundRule
	}

	var rule refundRule
	if err := json.Unmarshal([]byte(ruleJSON), &rule); err != nil {
		return refundRuleResult{}, errInvalidRefundRule
	}
	if rule.Version != 1 || len(rule.Stages) == 0 {
		return refundRuleResult{}, errInvalidRefundRule
	}

	sort.Slice(rule.Stages, func(i, j int) bool {
		return rule.Stages[i].BeforeMinutes > rule.Stages[j].BeforeMinutes
	})

	remainingMinutes := int64(showTime.Sub(now) / time.Minute)
	for _, stage := range rule.Stages {
		if stage.BeforeMinutes < 0 || stage.RefundPercent < 0 || stage.RefundPercent > 100 {
			return refundRuleResult{}, errInvalidRefundRule
		}
		if remainingMinutes >= stage.BeforeMinutes {
			return refundRuleResult{
				AllowRefund:   true,
				RefundPercent: stage.RefundPercent,
				RefundAmount:  orderAmount * stage.RefundPercent / 100,
			}, nil
		}
	}

	return refundRuleResult{
		AllowRefund:  false,
		RejectReason: "refund stage not matched",
		NoMatch:      true,
	}, nil
}
