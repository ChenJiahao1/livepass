package worker

import (
	"damai-go/jobs/order-close-worker/internal/logic"
	"damai-go/jobs/order-close-worker/internal/svc"
	"damai-go/services/order-rpc/closequeue"

	"github.com/hibiken/asynq"
)

func NewServeMux(svcCtx *svc.ServiceContext) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(closequeue.TaskTypeCloseTimeout, logic.NewCloseTimeoutTaskLogic(svcCtx).Handle)
	return mux
}
