package logic

import "testing"

func TestShouldLogCreateOrderPerf(t *testing.T) {
	tests := []struct {
		name             string
		enableValue      string
		sampleEveryValue string
		orderNumber      int64
		want             bool
	}{
		{
			name:             "disabled by default",
			enableValue:      "",
			sampleEveryValue: "",
			orderNumber:      101,
			want:             false,
		},
		{
			name:             "enabled without sampling",
			enableValue:      "1",
			sampleEveryValue: "",
			orderNumber:      101,
			want:             true,
		},
		{
			name:             "enabled with sample hit",
			enableValue:      "1",
			sampleEveryValue: "10",
			orderNumber:      120,
			want:             true,
		},
		{
			name:             "enabled with sample miss",
			enableValue:      "1",
			sampleEveryValue: "10",
			orderNumber:      121,
			want:             false,
		},
		{
			name:             "invalid sample falls back to always",
			enableValue:      "1",
			sampleEveryValue: "bad",
			orderNumber:      121,
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldLogCreateOrderPerf(tt.enableValue, tt.sampleEveryValue, tt.orderNumber); got != tt.want {
				t.Fatalf("ShouldLogCreateOrderPerf(%q, %q, %d) = %v, want %v", tt.enableValue, tt.sampleEveryValue, tt.orderNumber, got, tt.want)
			}
		})
	}
}
