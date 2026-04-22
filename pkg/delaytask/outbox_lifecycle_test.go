package delaytask

import "testing"

func TestOutboxLifecycleStatusConstants(t *testing.T) {
	if OutboxTaskStatusPending != 0 {
		t.Fatalf("OutboxTaskStatusPending = %d, want 0", OutboxTaskStatusPending)
	}
	if OutboxTaskStatusPublished != 1 {
		t.Fatalf("OutboxTaskStatusPublished = %d, want 1", OutboxTaskStatusPublished)
	}
	if OutboxTaskStatusProcessed != 3 {
		t.Fatalf("OutboxTaskStatusProcessed = %d, want 3", OutboxTaskStatusProcessed)
	}
	if OutboxTaskStatusFailed != 4 {
		t.Fatalf("OutboxTaskStatusFailed = %d, want 4", OutboxTaskStatusFailed)
	}
}

func TestOutboxLifecycleCanTransition(t *testing.T) {
	t.Parallel()

	allowed := []struct {
		name string
		from int64
		to   int64
	}{
		{name: "pending_to_published", from: OutboxTaskStatusPending, to: OutboxTaskStatusPublished},
		{name: "pending_to_failed", from: OutboxTaskStatusPending, to: OutboxTaskStatusFailed},
		{name: "published_to_published", from: OutboxTaskStatusPublished, to: OutboxTaskStatusPublished},
		{name: "published_to_processed", from: OutboxTaskStatusPublished, to: OutboxTaskStatusProcessed},
		{name: "published_to_failed", from: OutboxTaskStatusPublished, to: OutboxTaskStatusFailed},
		{name: "failed_to_published", from: OutboxTaskStatusFailed, to: OutboxTaskStatusPublished},
		{name: "failed_to_processed", from: OutboxTaskStatusFailed, to: OutboxTaskStatusProcessed},
	}

	for _, tc := range allowed {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !CanTransition(tc.from, tc.to) {
				t.Fatalf("CanTransition(%d, %d) = false, want true", tc.from, tc.to)
			}
		})
	}
}

func TestOutboxLifecycleRejectsProcessedTransitions(t *testing.T) {
	t.Parallel()

	for _, to := range []int64{OutboxTaskStatusPending, OutboxTaskStatusPublished, OutboxTaskStatusFailed} {
		if CanTransition(OutboxTaskStatusProcessed, to) {
			t.Fatalf("CanTransition(processed, %d) = true, want false", to)
		}
	}
}

func TestOutboxLifecycleHelpers(t *testing.T) {
	t.Parallel()

	if !IsTerminal(OutboxTaskStatusProcessed) {
		t.Fatalf("IsTerminal(processed) = false, want true")
	}
	for _, status := range []int64{OutboxTaskStatusPending, OutboxTaskStatusPublished, OutboxTaskStatusFailed} {
		if IsTerminal(status) {
			t.Fatalf("IsTerminal(%d) = true, want false", status)
		}
	}

	if ShouldRepublish(OutboxTaskStatusPending) {
		t.Fatalf("ShouldRepublish(pending) = true, want false")
	}
	if !ShouldRepublish(OutboxTaskStatusPublished) {
		t.Fatalf("ShouldRepublish(published) = false, want true")
	}
	if !ShouldRepublish(OutboxTaskStatusFailed) {
		t.Fatalf("ShouldRepublish(failed) = false, want true")
	}
	if ShouldRepublish(OutboxTaskStatusProcessed) {
		t.Fatalf("ShouldRepublish(processed) = true, want false")
	}
}
