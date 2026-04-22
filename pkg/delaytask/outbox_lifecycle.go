package delaytask

const (
	OutboxTaskStatusPending   int64 = 0
	OutboxTaskStatusPublished int64 = 1
	OutboxTaskStatusProcessed int64 = 3
	OutboxTaskStatusFailed    int64 = 4
)

var allowedOutboxTransitions = map[int64]map[int64]struct{}{
	OutboxTaskStatusPending: {
		OutboxTaskStatusPublished: {},
		OutboxTaskStatusFailed:    {},
	},
	OutboxTaskStatusPublished: {
		OutboxTaskStatusPublished: {},
		OutboxTaskStatusProcessed: {},
		OutboxTaskStatusFailed:    {},
	},
	OutboxTaskStatusFailed: {
		OutboxTaskStatusPublished: {},
		OutboxTaskStatusProcessed: {},
	},
}

func CanTransition(from, to int64) bool {
	allowed, ok := allowedOutboxTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

func IsTerminal(status int64) bool {
	return status == OutboxTaskStatusProcessed
}

func ShouldRepublish(status int64) bool {
	return status == OutboxTaskStatusPublished || status == OutboxTaskStatusFailed
}
