package rush

import _ "embed"

var (
	//go:embed admit_attempt.lua
	admitAttemptScript string

	//go:embed prepare_attempt_for_consume.lua
	prepareAttemptForConsumeScript string

	//go:embed fail_before_processing.lua
	failBeforeProcessingScript string

	//go:embed refresh_processing_lease.lua
	refreshProcessingLeaseScript string

	//go:embed finalize_success.lua
	finalizeSuccessScript string

	//go:embed finalize_failure.lua
	finalizeFailureScript string

	//go:embed finalize_closed_order.lua
	finalizeClosedOrderScript string
)
