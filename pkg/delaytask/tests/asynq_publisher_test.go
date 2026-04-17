package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"livepass/pkg/delaytask"

	"github.com/hibiken/asynq"
)

type fakeAsynqEnqueuer struct {
	task *asynq.Task
	opts []asynq.Option
	err  error
}

func (f *fakeAsynqEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.task = task
	f.opts = append([]asynq.Option(nil), opts...)
	if f.err != nil {
		return nil, f.err
	}
	return &asynq.TaskInfo{}, nil
}

func TestAsynqPublisherPublishUsesExpectedAsynqOptions(t *testing.T) {
	executeAt := time.Date(2026, time.April, 13, 15, 0, 0, 0, time.Local)
	enqueuer := &fakeAsynqEnqueuer{}
	publisher := delaytask.NewAsynqPublisher(enqueuer, delaytask.Options{
		Queue:     "order-close",
		MaxRetry:  5,
		UniqueTTL: 2 * time.Minute,
	})

	err := publisher.Publish(context.Background(), delaytask.Message{
		Type:      "order.close_timeout",
		Key:       "order.close_timeout:91001",
		Payload:   []byte(`{"orderNumber":91001}`),
		ExecuteAt: executeAt,
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if enqueuer.task == nil {
		t.Fatalf("expected task enqueued")
	}
	if enqueuer.task.Type() != "order.close_timeout" {
		t.Fatalf("task type = %s, want order.close_timeout", enqueuer.task.Type())
	}
	if string(enqueuer.task.Payload()) != `{"orderNumber":91001}` {
		t.Fatalf("task payload = %s", string(enqueuer.task.Payload()))
	}

	options := map[asynq.OptionType]interface{}{}
	for _, opt := range enqueuer.opts {
		options[opt.Type()] = opt.Value()
	}

	if options[asynq.QueueOpt] != "order-close" {
		t.Fatalf("queue option = %v, want order-close", options[asynq.QueueOpt])
	}
	if got, ok := options[asynq.ProcessAtOpt].(time.Time); !ok || !got.Equal(executeAt) {
		t.Fatalf("process at option = %v, want %v", options[asynq.ProcessAtOpt], executeAt)
	}
	if options[asynq.TaskIDOpt] != "order.close_timeout:91001" {
		t.Fatalf("task id option = %v, want order.close_timeout:91001", options[asynq.TaskIDOpt])
	}
	if options[asynq.MaxRetryOpt] != 5 {
		t.Fatalf("max retry option = %v, want 5", options[asynq.MaxRetryOpt])
	}
	if options[asynq.UniqueOpt] != 2*time.Minute {
		t.Fatalf("unique option = %v, want %v", options[asynq.UniqueOpt], 2*time.Minute)
	}
}

func TestAsynqPublisherTreatsDuplicateErrorsAsSuccess(t *testing.T) {
	for _, err := range []error{asynq.ErrTaskIDConflict, asynq.ErrDuplicateTask} {
		enqueuer := &fakeAsynqEnqueuer{err: err}
		publisher := delaytask.NewAsynqPublisher(enqueuer, delaytask.Options{Queue: "order-close"})

		if publishErr := publisher.Publish(context.Background(), delaytask.Message{
			Type:      "order.close_timeout",
			Key:       "order.close_timeout:91001",
			Payload:   []byte(`{"orderNumber":91001}`),
			ExecuteAt: time.Now(),
		}); publishErr != nil {
			t.Fatalf("Publish returned error %v for duplicate err %v", publishErr, err)
		}
	}
}

func TestAsynqPublisherReturnsNonDuplicateError(t *testing.T) {
	expectedErr := errors.New("enqueue failed")
	enqueuer := &fakeAsynqEnqueuer{err: expectedErr}
	publisher := delaytask.NewAsynqPublisher(enqueuer, delaytask.Options{Queue: "order-close"})

	err := publisher.Publish(context.Background(), delaytask.Message{
		Type:      "order.close_timeout",
		Key:       "order.close_timeout:91001",
		Payload:   []byte(`{"orderNumber":91001}`),
		ExecuteAt: time.Now(),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Publish error = %v, want %v", err, expectedErr)
	}
}

type blockingAsynqEnqueuer struct {
	deadline    time.Time
	hasDeadline bool
}

func (f *blockingAsynqEnqueuer) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.deadline, f.hasDeadline = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestAsynqPublisherPublishUsesEnqueueTimeout(t *testing.T) {
	enqueuer := &blockingAsynqEnqueuer{}
	publisher := delaytask.NewAsynqPublisher(enqueuer, delaytask.Options{
		Queue:          "order-close",
		EnqueueTimeout: 30 * time.Millisecond,
	})

	start := time.Now()
	err := publisher.Publish(context.Background(), delaytask.Message{
		Type:      "order.close_timeout",
		Key:       "order.close_timeout:91002",
		Payload:   []byte(`{"orderNumber":91002}`),
		ExecuteAt: time.Now().Add(time.Minute),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Publish error = %v, want %v", err, context.DeadlineExceeded)
	}
	if !enqueuer.hasDeadline {
		t.Fatalf("expected enqueue context deadline to be set")
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("Publish elapsed = %s, want <= 300ms", elapsed)
	}
}
