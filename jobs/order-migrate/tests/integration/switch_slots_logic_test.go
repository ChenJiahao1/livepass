package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	logicpkg "damai-go/jobs/order-migrate/internal/logic"
	"damai-go/services/order-rpc/sharding"
)

func TestSwitchSlotsPromotesVerifiedSlotToPrimaryNew(t *testing.T) {
	resetOrderMigrateState(t)

	entries := buildOrderMigrateRouteEntries()
	entries[0].Status = sharding.RouteStatusVerifying
	entries[0].WriteMode = sharding.WriteModeDualWrite
	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", entries)
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	logic := logicpkg.NewSwitchSlotsLogic(context.Background(), svcCtx)
	resp, err := logic.SwitchSlots()
	if err != nil {
		t.Fatalf("SwitchSlots() error = %v", err)
	}
	if resp.UpdatedSlots != 1 {
		t.Fatalf("SwitchSlots() updated slots = %d, want 1", resp.UpdatedSlots)
	}

	content, err := os.ReadFile(routeMapFile)
	if err != nil {
		t.Fatalf("ReadFile(routeMapFile) error = %v", err)
	}
	if !strings.Contains(string(content), "Status: primary_new") {
		t.Fatalf("expected route map file to promote slot to primary_new, content=%s", string(content))
	}
	if !strings.Contains(string(content), "WriteMode: shard_primary") {
		t.Fatalf("expected route map file to switch write mode to shard_primary, content=%s", string(content))
	}
}
