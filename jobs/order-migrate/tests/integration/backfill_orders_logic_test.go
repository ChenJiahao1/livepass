package integration_test

import (
	"context"
	"strings"
	"testing"

	logicpkg "damai-go/jobs/order-migrate/internal/logic"
)

func TestBackfillOrdersResumesFromCheckpoint(t *testing.T) {
	resetOrderMigrateState(t)

	entries := buildOrderMigrateRouteEntries()
	entries[0].Status = "shadow_write"
	entries[1].Status = "shadow_write"
	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", entries)
	checkpointFile := t.TempDir() + "/backfill.checkpoint.json"
	cfg := newOrderMigrateTestConfig(t, routeMapFile, checkpointFile)
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	userIDSlot10 := mustFindUserIDByLogicSlotForMigrate(t, 10)
	userIDSlot11 := mustFindUserIDByLogicSlotForMigrate(t, 11)
	legacyOrderNumber1 := int64(36000000000000001)
	legacyOrderNumber2 := int64(36000000000000002)
	seedLegacyOrderFixtures(
		t,
		migrateOrderFixture{ID: 8001, OrderNumber: legacyOrderNumber1, UserID: userIDSlot10},
		migrateOrderFixture{ID: 8002, OrderNumber: legacyOrderNumber2, UserID: userIDSlot11},
	)
	seedLegacyOrderTicketFixtures(
		t,
		migrateOrderTicketFixture{ID: 8801, OrderNumber: legacyOrderNumber1, UserID: userIDSlot10, TicketUserID: 701, SeatID: 501, SeatRow: 1, SeatCol: 1},
		migrateOrderTicketFixture{ID: 8802, OrderNumber: legacyOrderNumber2, UserID: userIDSlot11, TicketUserID: 702, SeatID: 502, SeatRow: 1, SeatCol: 2},
	)

	logic := logicpkg.NewBackfillOrdersLogic(context.Background(), svcCtx)
	firstResp, err := logic.BackfillOrders()
	if err != nil {
		t.Fatalf("first BackfillOrders() error = %v", err)
	}
	if firstResp.ProcessedCount != 1 || firstResp.LastOrderID != 8001 {
		t.Fatalf("first response = %+v, want processed=1 lastOrderID=8001", firstResp)
	}
	firstRoute := routeForUser(userIDSlot10)
	if countTableRowsByOrderNumber(t, "d_order_"+firstRoute.TableSuffix, legacyOrderNumber1) != 1 {
		t.Fatalf("expected first order backfilled into shard table")
	}
	if countTableRowsByOrderNumber(t, "d_user_order_index_"+firstRoute.TableSuffix, legacyOrderNumber1) != 1 {
		t.Fatalf("expected first order index backfilled into shard table")
	}
	if countTableRowsByOrderNumber(t, "d_order_route_legacy", legacyOrderNumber1) != 1 {
		t.Fatalf("expected legacy route directory row")
	}
	if !strings.Contains(readCheckpointFile(t, checkpointFile), "\"last_order_id\":8001") {
		t.Fatalf("expected checkpoint to record first legacy order id")
	}
	routeMapContent := readCheckpointFile(t, routeMapFile)
	if !strings.Contains(routeMapContent, "Status: backfilling") {
		t.Fatalf("expected backfill action to promote slots to backfilling, content=%s", routeMapContent)
	}

	secondResp, err := logic.BackfillOrders()
	if err != nil {
		t.Fatalf("second BackfillOrders() error = %v", err)
	}
	if secondResp.ProcessedCount != 1 || secondResp.LastOrderID != 8002 {
		t.Fatalf("second response = %+v, want processed=1 lastOrderID=8002", secondResp)
	}
	secondRoute := routeForUser(userIDSlot11)
	if countTableRowsByOrderNumber(t, "d_order_"+secondRoute.TableSuffix, legacyOrderNumber2) != 1 {
		t.Fatalf("expected second order backfilled into shard table")
	}
	if !strings.Contains(readCheckpointFile(t, checkpointFile), "\"last_order_id\":8002") {
		t.Fatalf("expected checkpoint to advance to second legacy order id")
	}
}
