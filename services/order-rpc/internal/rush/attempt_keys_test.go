package rush

import "testing"

func TestAttemptKeyHelpersUseShowTimeScopedHashes(t *testing.T) {
	const prefix = "livepass:test:rush"
	const showTimeID = int64(20001)
	const orderNumber = int64(90001)
	if got, want := attemptRecordKey(prefix, showTimeID, orderNumber), "livepass:test:rush:attempt:{st:20001}:90001"; got != want {
		t.Fatalf("attemptRecordKey() = %q, want %q", got, want)
	}
	if got, want := userInflightKey(prefix, showTimeID), "livepass:test:rush:user_inflight:{st:20001}"; got != want {
		t.Fatalf("userInflightKey() = %q, want %q", got, want)
	}
	if got, want := viewerInflightKey(prefix, showTimeID), "livepass:test:rush:viewer_inflight:{st:20001}"; got != want {
		t.Fatalf("viewerInflightKey() = %q, want %q", got, want)
	}
	if got, want := userActiveKey(prefix, showTimeID), "livepass:test:rush:user_active:{st:20001}"; got != want {
		t.Fatalf("userActiveKey() = %q, want %q", got, want)
	}
	if got, want := viewerActiveKey(prefix, showTimeID), "livepass:test:rush:viewer_active:{st:20001}"; got != want {
		t.Fatalf("viewerActiveKey() = %q, want %q", got, want)
	}
	if got, want := quotaAvailableKey(prefix, showTimeID), "livepass:test:rush:quota:{st:20001}"; got != want {
		t.Fatalf("quotaAvailableKey() = %q, want %q", got, want)
	}
}
