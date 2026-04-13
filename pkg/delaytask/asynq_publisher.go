package delaytask

import (
	"context"

	"github.com/hibiken/asynq"
)

type asynqEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type asynqPublisher struct {
	enqueuer asynqEnqueuer
	options  Options
}

func NewAsynqPublisher(enqueuer asynqEnqueuer, options Options) Publisher {
	return &asynqPublisher{
		enqueuer: enqueuer,
		options:  options,
	}
}

func (p *asynqPublisher) Publish(ctx context.Context, message Message) error {
	if p.options.EnqueueTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.options.EnqueueTimeout)
		defer cancel()
	}

	opts := []asynq.Option{
		asynq.Queue(p.options.Queue),
		asynq.ProcessAt(message.ExecuteAt),
		asynq.TaskID(message.Key),
	}
	if p.options.MaxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(p.options.MaxRetry))
	}
	if p.options.UniqueTTL > 0 {
		opts = append(opts, asynq.Unique(p.options.UniqueTTL))
	}

	_, err := p.enqueuer.EnqueueContext(ctx, asynq.NewTask(message.Type, message.Payload), opts...)
	if IsDuplicateEnqueueError(err) {
		return nil
	}
	return err
}
