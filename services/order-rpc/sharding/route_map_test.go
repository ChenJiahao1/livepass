package sharding

import (
	"strings"
	"testing"
)

func TestRouteMapSelectsPhysicalTargetByVersionAndSlot(t *testing.T) {
	routeMap, err := NewRouteMap("v2", []RouteEntry{
		{
			Version:     "v1",
			LogicSlot:   9,
			DBKey:       "order-db-0",
			TableSuffix: "00",
			Status:      RouteStatusStable,
			WriteMode:   WriteModeShardPrimary,
		},
		{
			Version:     "v2",
			LogicSlot:   9,
			DBKey:       "order-db-1",
			TableSuffix: "07",
			Status:      RouteStatusStable,
			WriteMode:   WriteModeShardPrimary,
		},
	})
	if err != nil {
		t.Fatalf("NewRouteMap() error = %v", err)
	}

	route, err := routeMap.RouteByVersionAndSlot("v2", 9)
	if err != nil {
		t.Fatalf("RouteByVersionAndSlot() error = %v", err)
	}
	if route.DBKey != "order-db-1" {
		t.Fatalf("route db key = %s, want order-db-1", route.DBKey)
	}
	if route.TableSuffix != "07" {
		t.Fatalf("route table suffix = %s, want 07", route.TableSuffix)
	}
	if route.Version != "v2" {
		t.Fatalf("route version = %s, want v2", route.Version)
	}
}

func TestRouteMapRejectsNonStableShardPrimaryEntries(t *testing.T) {
	testCases := []struct {
		name  string
		entry RouteEntry
	}{
		{
			name: "non stable status rejected",
			entry: RouteEntry{
				Version:     "v1",
				LogicSlot:   10,
				DBKey:       "order-db-0",
				TableSuffix: "00",
				Status:      "shadow_write",
				WriteMode:   WriteModeShardPrimary,
			},
		},
		{
			name: "non shard primary rejected",
			entry: RouteEntry{
				Version:     "v1",
				LogicSlot:   11,
				DBKey:       "order-db-1",
				TableSuffix: "01",
				Status:      RouteStatusStable,
				WriteMode:   "primary_old",
			},
		},
		{
			name: "dual write rejected",
			entry: RouteEntry{
				Version:     "v1",
				LogicSlot:   12,
				DBKey:       "order-db-0",
				TableSuffix: "00",
				Status:      RouteStatusStable,
				WriteMode:   "fanout",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRouteMap("v1", []RouteEntry{tc.entry})
			if err == nil {
				t.Fatalf("NewRouteMap() error = nil, want shard-only route rejection")
			}
			if !strings.Contains(err.Error(), "stable") &&
				!strings.Contains(err.Error(), "shard_primary") &&
				!strings.Contains(err.Error(), "unsupported route status") &&
				!strings.Contains(err.Error(), "unsupported write mode") {
				t.Fatalf("NewRouteMap() error = %v, want shard-only validation message", err)
			}
		})
	}
}

func TestValidateRouteStatusTransitionAllowsStableOnly(t *testing.T) {
	if err := ValidateRouteStatusTransition(RouteStatusStable, RouteStatusStable); err != nil {
		t.Fatalf("ValidateRouteStatusTransition() error = %v, want nil", err)
	}
	if err := ValidateRouteStatusTransition(RouteStatusStable, "shadow_write"); err == nil {
		t.Fatalf("ValidateRouteStatusTransition() error = nil, want rejection")
	}
}
