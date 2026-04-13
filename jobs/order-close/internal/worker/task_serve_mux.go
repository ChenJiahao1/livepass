package worker

import (
	"damai-go/jobs/order-close/internal/svc"
	"damai-go/jobs/order-close/taskdef"

	"github.com/hibiken/asynq"
)

func NewServeMux(svcCtx *svc.WorkerServiceContext) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskdef.TaskTypeCloseTimeout, NewCloseTimeoutTaskLogic(svcCtx).Handle)
	return mux
}
