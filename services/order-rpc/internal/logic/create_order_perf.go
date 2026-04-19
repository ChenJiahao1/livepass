package logic

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/status"
)

const (
	OrderRpcEnableCreatePerfLogEnv        = "ORDER_RPC_ENABLE_CREATE_PERF_LOG"
	OrderRpcCreatePerfLogSampleEveryEnv   = "ORDER_RPC_CREATE_PERF_LOG_SAMPLE_EVERY"
	createOrderPerfStageLogContent        = "create order perf stage"
	createOrderAsyncSendFailureLogContent = "create order async send failed"
)

type createOrderPerfState struct {
	orderNumber             int64
	userID                  int64
	rejectCode              int64
	grpcCode                string
	reasonCode              string
	result                  string
	purchaseTokenVerifyCost time.Duration
	redisAdmitCost          time.Duration
	asyncDispatchCost       time.Duration
	forceLog                bool
}

func ShouldLogCreateOrderPerf(enableValue, sampleEveryValue string, orderNumber int64) bool {
	if !isEnvEnabled(enableValue) {
		return false
	}

	sampleEvery, err := strconv.ParseInt(sampleEveryValue, 10, 64)
	if err != nil || sampleEvery <= 1 {
		return true
	}
	if orderNumber <= 0 {
		return true
	}

	return orderNumber%sampleEvery == 0
}

func isEnvEnabled(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func shouldLogCreateOrderPerfByEnv(orderNumber int64) bool {
	return ShouldLogCreateOrderPerf(
		os.Getenv(OrderRpcEnableCreatePerfLogEnv),
		os.Getenv(OrderRpcCreatePerfLogSampleEveryEnv),
		orderNumber,
	)
}

func logCreateOrderPerfState(ctx context.Context, state createOrderPerfState) {
	if !state.forceLog && !shouldLogCreateOrderPerfByEnv(state.orderNumber) {
		return
	}

	logx.WithContext(ctx).Infow(
		createOrderPerfStageLogContent,
		logx.Field("orderNumber", state.orderNumber),
		logx.Field("userId", state.userID),
		logx.Field("result", state.result),
		logx.Field("rejectCode", state.rejectCode),
		logx.Field("grpcCode", state.grpcCode),
		logx.Field("reasonCode", state.reasonCode),
		logx.Field("purchaseTokenVerifyMs", state.purchaseTokenVerifyCost.Milliseconds()),
		logx.Field("redisAdmitMs", state.redisAdmitCost.Milliseconds()),
		logx.Field("asyncDispatchScheduleMs", state.asyncDispatchCost.Milliseconds()),
	)
}

func grpcCodeOf(err error) string {
	return status.Code(err).String()
}
