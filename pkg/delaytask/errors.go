package delaytask

import (
	"errors"

	"github.com/hibiken/asynq"
)

func IsDuplicateEnqueueError(err error) bool {
	return errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask)
}
