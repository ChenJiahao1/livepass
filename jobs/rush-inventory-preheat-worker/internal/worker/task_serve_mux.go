package worker

import (
	"damai-go/jobs/rush-inventory-preheat-worker/internal/logic"
	"damai-go/jobs/rush-inventory-preheat-worker/internal/svc"
	"damai-go/services/program-rpc/preheatqueue"

	"github.com/hibiken/asynq"
)

func NewServeMux(svcCtx *svc.ServiceContext) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(preheatqueue.TaskTypeRushInventoryPreheat, logic.NewRushInventoryPreheatTaskLogic(svcCtx).Handle)
	return mux
}
