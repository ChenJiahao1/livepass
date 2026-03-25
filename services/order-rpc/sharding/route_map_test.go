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
			WriteMode:   WriteModeLegacyPrimary,
		},
		{
			Version:     "v2",
			LogicSlot:   9,
			DBKey:       "order-db-1",
			TableSuffix: "07",
			Status:      RouteStatusPrimaryNew,
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

func TestMigrationModeRejectsIllegalStateTransition(t *testing.T) {
	if err := ValidateMigrationModeTransition(MigrationModeLegacyOnly, MigrationModeShardOnly); err == nil {
		t.Fatalf("ValidateMigrationModeTransition() error = nil, want rejection")
	}
	if err := ValidateMigrationModeTransition(MigrationModeLegacyOnly, MigrationModeDualWriteShadow); err != nil {
		t.Fatalf("ValidateMigrationModeTransition() error = %v, want nil", err)
	}
}

func TestRouteMapRejectsInvalidStatusWriteModeCombination(t *testing.T) {
	testCases := []struct {
		name  string
		entry RouteEntry
	}{
		{
			name: "primary new requires shard primary",
			entry: RouteEntry{
				Version:     "v1",
				LogicSlot:   10,
				DBKey:       "order-db-0",
				TableSuffix: "00",
				Status:      RouteStatusPrimaryNew,
				WriteMode:   WriteModeDualWrite,
			},
		},
		{
			name: "rollback requires legacy primary",
			entry: RouteEntry{
				Version:     "v1",
				LogicSlot:   11,
				DBKey:       "order-db-1",
				TableSuffix: "01",
				Status:      RouteStatusRollback,
				WriteMode:   WriteModeDualWrite,
			},
		},
		{
			name: "backfilling requires dual write",
			entry: RouteEntry{
				Version:     "v1",
				LogicSlot:   12,
				DBKey:       "order-db-0",
				TableSuffix: "00",
				Status:      RouteStatusBackfilling,
				WriteMode:   WriteModeLegacyPrimary,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRouteMap("v1", []RouteEntry{tc.entry})
			if err == nil {
				t.Fatalf("NewRouteMap() error = nil, want invalid status/write mode rejection")
			}
			if !strings.Contains(err.Error(), "write mode") {
				t.Fatalf("NewRouteMap() error = %v, want write mode validation message", err)
			}
		})
	}
}
