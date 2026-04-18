package tests

import (
	"testing"

	"livepass/pkg/seatfreeze"
)

func TestFreezeTokenRoundTrip(t *testing.T) {
	token := seatfreeze.FormatToken(20001, 30001, 91001, 4)
	if token != "freeze-st20001-tc30001-o91001-e4" {
		t.Fatalf("unexpected token %q", token)
	}

	parsed, err := seatfreeze.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if parsed.ShowTimeID != 20001 || parsed.TicketCategoryID != 30001 || parsed.OrderNumber != 91001 || parsed.ProcessingEpoch != 4 {
		t.Fatalf("unexpected parsed token %+v", parsed)
	}
}

func TestParseTokenRejectsInvalidValue(t *testing.T) {
	cases := []string{
		"",
		"freeze-st0-tc30001-o91001-e4",
		"freeze-st20001-tc0-o91001-e4",
		"freeze-st20001-tc30001-o0-e4",
		"freeze-st20001-tc30001-o91001-e0",
		"freeze-20001-30001-91001-4",
	}

	for _, token := range cases {
		if _, err := seatfreeze.ParseToken(token); err == nil {
			t.Fatalf("expected invalid token error for %q", token)
		}
	}
}
