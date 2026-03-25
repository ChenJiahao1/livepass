package sharding

import "testing"

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
