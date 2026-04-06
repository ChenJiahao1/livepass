package rush

import _ "embed"

var (
	//go:embed admit_attempt.lua
	admitAttemptScript string

	//go:embed mark_attempt_queued.lua
	markAttemptQueuedScript string

	//go:embed mark_attempt_verifying.lua
	markAttemptVerifyingScript string

	//go:embed claim_processing.lua
	claimProcessingScript string

	//go:embed release_attempt.lua
	releaseAttemptScript string

	//go:embed commit_attempt_projection.lua
	commitAttemptProjectionScript string

	//go:embed release_closed_order_projection.lua
	releaseClosedOrderProjectionScript string
)
