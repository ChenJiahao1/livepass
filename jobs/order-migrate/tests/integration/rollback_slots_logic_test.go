package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	logicpkg "damai-go/jobs/order-migrate/internal/logic"
	"damai-go/services/order-rpc/sharding"
)

func TestRollbackSlotsFallsBackToLegacyPrimary(t *testing.T) {
	resetOrderMigrateState(t)

	entries := buildOrderMigrateRouteEntries()
	entries[0].Status = sharding.RouteStatusPrimaryNew
	entries[0].WriteMode = sharding.WriteModeShardPrimary
	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", entries)
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	logic := logicpkg.NewRollbackSlotsLogic(context.Background(), svcCtx)
	resp, err := logic.RollbackSlots()
	if err != nil {
		t.Fatalf("RollbackSlots() error = %v", err)
	}
	if resp.UpdatedSlots != 1 {
		t.Fatalf("RollbackSlots() updated slots = %d, want 1", resp.UpdatedSlots)
	}

	content, err := os.ReadFile(routeMapFile)
	if err != nil {
		t.Fatalf("ReadFile(routeMapFile) error = %v", err)
	}
	if !strings.Contains(string(content), "Status: rollback") {
		t.Fatalf("expected route map file to mark slot rollback, content=%s", string(content))
	}
	if !strings.Contains(string(content), "WriteMode: legacy_primary") {
		t.Fatalf("expected route map file to restore legacy_primary write mode, content=%s", string(content))
	}
}
