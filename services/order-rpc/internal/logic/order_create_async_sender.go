package logic

import (
	"context"
	"time"

	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type asyncOrderCreateEvent struct {
	orderNumber int64
	showTimeID  int64
	key         string
	body        []byte
	reasonAt    time.Time
}

func dispatchOrderCreateEventAsync(svcCtx *svc.ServiceContext, logger logx.Logger, item asyncOrderCreateEvent) {
	if svcCtx == nil || svcCtx.OrderCreateProducer == nil {
		return
	}

	payload := append([]byte(nil), item.body...)
	go func() {
		sendStartedAt := time.Now()
		sendCtx := context.Background()
		cancel := func() {}
		if timeout := svcCtx.Config.Kafka.ProducerTimeout; timeout > 0 {
			sendCtx, cancel = context.WithTimeout(sendCtx, timeout)
		}
		defer cancel()

		if err := svcCtx.OrderCreateProducer.Send(sendCtx, item.key, payload); err != nil {
			logger.Errorw(
				createOrderAsyncSendFailureLogContent,
				logx.Field("orderNumber", item.orderNumber),
				logx.Field("showTimeId", item.showTimeID),
				logx.Field("reasonCode", mapAsyncKafkaSendReason(err)),
				logx.Field("sendMs", time.Since(sendStartedAt).Milliseconds()),
				logx.Field("error", err.Error()),
			)
			failPendingAttemptAfterAsyncSendError(context.Background(), svcCtx, logger, item, err)
		}
	}()
}

func failPendingAttemptAfterAsyncSendError(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, item asyncOrderCreateEvent, sendErr error) {
	if svcCtx == nil || svcCtx.AttemptStore == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = logx.WithContext(ctx)
	}

	record, err := svcCtx.AttemptStore.GetByShowTimeAndOrderNumber(ctx, item.showTimeID, item.orderNumber)
	if err != nil {
		logger.Errorf("load attempt after async kafka send failed, orderNumber=%d err=%v", item.orderNumber, err)
		return
	}

	now := item.reasonAt
	if now.IsZero() {
		now = time.Now()
	}
	outcome, err := svcCtx.AttemptStore.FailBeforeProcessing(ctx, record, mapAsyncKafkaSendReason(sendErr), now)
	if err != nil {
		logger.Errorf("fail pending attempt after async kafka send failed, orderNumber=%d err=%v", item.orderNumber, err)
		return
	}
	if outcome != rush.AttemptTransitioned &&
		outcome != rush.AttemptLostOwnership &&
		outcome != rush.AttemptAlreadyFailed &&
		outcome != rush.AttemptAlreadySucceeded {
		logger.Errorf("unexpected async kafka compensation outcome, orderNumber=%d outcome=%s", item.orderNumber, outcome)
	}
}
