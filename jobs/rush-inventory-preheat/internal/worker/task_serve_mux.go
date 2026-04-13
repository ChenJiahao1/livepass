package worker

import (
	"damai-go/jobs/rush-inventory-preheat/internal/svc"
	"damai-go/jobs/rush-inventory-preheat/taskdef"

	"github.com/hibiken/asynq"
)

func NewServeMux(svcCtx *svc.WorkerServiceContext) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskdef.TaskTypeRushInventoryPreheat, NewRushInventoryPreheatTaskLogic(svcCtx).Handle)
	return mux
}
