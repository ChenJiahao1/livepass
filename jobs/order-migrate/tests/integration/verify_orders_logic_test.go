package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	logicpkg "damai-go/jobs/order-migrate/internal/logic"
	"damai-go/services/order-rpc/sharding"
)

func TestVerifyOrdersDetectsAggregateMismatch(t *testing.T) {
	resetOrderMigrateState(t)

	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", buildOrderMigrateRouteEntries())
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	userID := mustFindUserIDByLogicSlotForMigrate(t, 10)
	orderNumber := sharding.BuildOrderNumber(userID, mustParseMigrateTime(t, "2026-03-24 12:00:00"), 1, 1)
	route := routeForUser(userID)
	seedLegacyOrderFixtures(t, migrateOrderFixture{ID: 8101, OrderNumber: orderNumber, UserID: userID, OrderPrice: 299})
	seedLegacyOrderTicketFixtures(t, migrateOrderTicketFixture{ID: 8901, OrderNumber: orderNumber, UserID: userID, TicketUserID: 703, SeatID: 601, SeatRow: 1, SeatCol: 1})
	seedLegacyOrderOrderCopy := migrateOrderFixture{ID: 8101, OrderNumber: orderNumber, UserID: userID, OrderPrice: 399}
	seedLegacyOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, seedLegacyOrderOrderCopy)

	logic := logicpkg.NewVerifyOrdersLogic(context.Background(), svcCtx)
	_, err := logic.VerifyOrders()
	if err == nil || !strings.Contains(err.Error(), "sum mismatch") {
		t.Fatalf("VerifyOrders() error = %v, want sum mismatch", err)
	}
}

func TestVerifyOrdersDetectsTicketSnapshotMismatch(t *testing.T) {
	resetOrderMigrateState(t)

	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", buildOrderMigrateRouteEntries())
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	userID := mustFindUserIDByLogicSlotForMigrate(t, 10)
	orderNumber := sharding.BuildOrderNumber(userID, mustParseMigrateTime(t, "2026-03-24 12:10:00"), 1, 2)
	route := routeForUser(userID)
	orderFixture := migrateOrderFixture{ID: 8201, OrderNumber: orderNumber, UserID: userID, OrderPrice: 299}

	seedLegacyOrderFixtures(t, orderFixture)
	seedLegacyOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, orderFixture)
	seedLegacyOrderTicketFixtures(t, migrateOrderTicketFixture{ID: 9201, OrderNumber: orderNumber, UserID: userID, TicketUserID: 703, SeatID: 601, SeatRow: 1, SeatCol: 1})
	seedShardOrderTicketFixturesIntoTable(t, "d_order_ticket_user_"+route.TableSuffix,
		migrateOrderTicketFixture{ID: 9201, OrderNumber: orderNumber, UserID: userID, TicketUserID: 703, SeatID: 602, SeatRow: 1, SeatCol: 1},
	)
	seedLegacyUserOrderIndexFixtures(t, migrateUserOrderIndexFixture{ID: 10201, OrderNumber: orderNumber, UserID: userID, OrderPrice: 299})
	seedShardUserOrderIndexFixturesIntoTable(t, "d_user_order_index_"+route.TableSuffix,
		migrateUserOrderIndexFixture{ID: 10201, OrderNumber: orderNumber, UserID: userID, OrderPrice: 299},
	)

	logic := logicpkg.NewVerifyOrdersLogic(context.Background(), svcCtx)
	_, err := logic.VerifyOrders()
	if err == nil || !strings.Contains(err.Error(), "ticket snapshot mismatch") {
		t.Fatalf("VerifyOrders() error = %v, want ticket snapshot mismatch", err)
	}
}

func TestVerifyOrdersDetectsUserOrderIndexMismatch(t *testing.T) {
	resetOrderMigrateState(t)

	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", buildOrderMigrateRouteEntries())
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	userID := mustFindUserIDByLogicSlotForMigrate(t, 10)
	orderNumber := sharding.BuildOrderNumber(userID, mustParseMigrateTime(t, "2026-03-24 12:20:00"), 1, 3)
	route := routeForUser(userID)
	orderFixture := migrateOrderFixture{ID: 8301, OrderNumber: orderNumber, UserID: userID, OrderPrice: 299}

	seedLegacyOrderFixtures(t, orderFixture)
	seedLegacyOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, orderFixture)
	seedLegacyOrderTicketFixtures(t, migrateOrderTicketFixture{ID: 9301, OrderNumber: orderNumber, UserID: userID, TicketUserID: 704, SeatID: 611, SeatRow: 1, SeatCol: 1})
	seedShardOrderTicketFixturesIntoTable(t, "d_order_ticket_user_"+route.TableSuffix,
		migrateOrderTicketFixture{ID: 9301, OrderNumber: orderNumber, UserID: userID, TicketUserID: 704, SeatID: 611, SeatRow: 1, SeatCol: 1},
	)
	seedLegacyUserOrderIndexFixtures(t, migrateUserOrderIndexFixture{ID: 10301, OrderNumber: orderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 299})
	seedShardUserOrderIndexFixturesIntoTable(t, "d_user_order_index_"+route.TableSuffix,
		migrateUserOrderIndexFixture{ID: 10301, OrderNumber: orderNumber, UserID: userID, OrderStatus: 2, OrderPrice: 299},
	)

	logic := logicpkg.NewVerifyOrdersLogic(context.Background(), svcCtx)
	_, err := logic.VerifyOrders()
	if err == nil || !strings.Contains(err.Error(), "user order index mismatch") {
		t.Fatalf("VerifyOrders() error = %v, want user order index mismatch", err)
	}
}

func TestVerifyOrdersPromotesBackfillingSlotToVerifying(t *testing.T) {
	resetOrderMigrateState(t)

	entries := buildOrderMigrateRouteEntries()
	entries[0].Status = sharding.RouteStatusBackfilling
	entries[1].Status = sharding.RouteStatusBackfilling
	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", entries)
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	userID := mustFindUserIDByLogicSlotForMigrate(t, 10)
	orderNumber := sharding.BuildOrderNumber(userID, mustParseMigrateTime(t, "2026-03-24 12:30:00"), 1, 4)
	route := routeForUser(userID)
	orderFixture := migrateOrderFixture{ID: 8401, OrderNumber: orderNumber, UserID: userID, OrderPrice: 299}

	seedLegacyOrderFixtures(t, orderFixture)
	seedLegacyOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, orderFixture)
	seedLegacyOrderTicketFixtures(t, migrateOrderTicketFixture{ID: 9401, OrderNumber: orderNumber, UserID: userID, TicketUserID: 704, SeatID: 621, SeatRow: 1, SeatCol: 1})
	seedShardOrderTicketFixturesIntoTable(t, "d_order_ticket_user_"+route.TableSuffix,
		migrateOrderTicketFixture{ID: 9401, OrderNumber: orderNumber, UserID: userID, TicketUserID: 704, SeatID: 621, SeatRow: 1, SeatCol: 1},
	)
	seedLegacyUserOrderIndexFixtures(t, migrateUserOrderIndexFixture{ID: 10401, OrderNumber: orderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 299})
	seedShardUserOrderIndexFixturesIntoTable(t, "d_user_order_index_"+route.TableSuffix,
		migrateUserOrderIndexFixture{ID: 10401, OrderNumber: orderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 299},
	)

	logic := logicpkg.NewVerifyOrdersLogic(context.Background(), svcCtx)
	resp, err := logic.VerifyOrders()
	if err != nil {
		t.Fatalf("VerifyOrders() error = %v", err)
	}
	if resp.VerifiedSlots != 1 {
		t.Fatalf("VerifyOrders() verified slots = %d, want 1", resp.VerifiedSlots)
	}

	content, readErr := os.ReadFile(routeMapFile)
	if readErr != nil {
		t.Fatalf("ReadFile(routeMapFile) error = %v", readErr)
	}
	if !strings.Contains(string(content), "Status: verifying") {
		t.Fatalf("expected verify action to promote slot to verifying, content=%s", string(content))
	}
}

func TestVerifyOrdersPersistsCheckpointBeforePromotingSlots(t *testing.T) {
	resetOrderMigrateState(t)

	entries := buildOrderMigrateRouteEntries()
	entries[0].Status = sharding.RouteStatusBackfilling
	routeMapFile := writeOrderMigrateRouteMapFile(t, "v1", entries)
	cfg := newOrderMigrateTestConfig(t, routeMapFile, t.TempDir()+"/checkpoint.json")
	verifyCheckpointFile := t.TempDir() + "/verify.checkpoint.json"
	cfg.Verify.BatchSize = 1
	cfg.Verify.CheckpointFile = verifyCheckpointFile
	svcCtx := newOrderMigrateTestServiceContext(t, cfg)

	userID := mustFindUserIDByLogicSlotForMigrate(t, 10)
	route := routeForUser(userID)
	firstOrderNumber := sharding.BuildOrderNumber(userID, mustParseMigrateTime(t, "2026-03-24 12:40:00"), 1, 5)
	secondOrderNumber := sharding.BuildOrderNumber(userID, mustParseMigrateTime(t, "2026-03-24 12:41:00"), 1, 6)

	seedLegacyOrderFixtures(
		t,
		migrateOrderFixture{ID: 8501, OrderNumber: firstOrderNumber, UserID: userID, OrderPrice: 299},
		migrateOrderFixture{ID: 8502, OrderNumber: secondOrderNumber, UserID: userID, OrderPrice: 399},
	)
	seedLegacyOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, migrateOrderFixture{ID: 8501, OrderNumber: firstOrderNumber, UserID: userID, OrderPrice: 299})
	seedLegacyOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, migrateOrderFixture{ID: 8502, OrderNumber: secondOrderNumber, UserID: userID, OrderPrice: 399})
	seedLegacyOrderTicketFixtures(
		t,
		migrateOrderTicketFixture{ID: 9501, OrderNumber: firstOrderNumber, UserID: userID, TicketUserID: 705, SeatID: 631, SeatRow: 1, SeatCol: 1},
		migrateOrderTicketFixture{ID: 9502, OrderNumber: secondOrderNumber, UserID: userID, TicketUserID: 706, SeatID: 632, SeatRow: 1, SeatCol: 2},
	)
	seedShardOrderTicketFixturesIntoTable(
		t,
		"d_order_ticket_user_"+route.TableSuffix,
		migrateOrderTicketFixture{ID: 9501, OrderNumber: firstOrderNumber, UserID: userID, TicketUserID: 705, SeatID: 631, SeatRow: 1, SeatCol: 1},
		migrateOrderTicketFixture{ID: 9502, OrderNumber: secondOrderNumber, UserID: userID, TicketUserID: 706, SeatID: 632, SeatRow: 1, SeatCol: 2},
	)
	seedLegacyUserOrderIndexFixtures(
		t,
		migrateUserOrderIndexFixture{ID: 10501, OrderNumber: firstOrderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 299},
		migrateUserOrderIndexFixture{ID: 10502, OrderNumber: secondOrderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 399},
	)
	seedShardUserOrderIndexFixturesIntoTable(
		t,
		"d_user_order_index_"+route.TableSuffix,
		migrateUserOrderIndexFixture{ID: 10501, OrderNumber: firstOrderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 299},
		migrateUserOrderIndexFixture{ID: 10502, OrderNumber: secondOrderNumber, UserID: userID, OrderStatus: 1, OrderPrice: 399},
	)

	logic := logicpkg.NewVerifyOrdersLogic(context.Background(), svcCtx)
	resp, err := logic.VerifyOrders()
	if err != nil {
		t.Fatalf("VerifyOrders() error = %v", err)
	}
	if resp.VerifiedSlots != 0 {
		t.Fatalf("VerifyOrders() verified slots = %d, want 0 before incremental verify reaches EOF", resp.VerifiedSlots)
	}

	content, readErr := os.ReadFile(routeMapFile)
	if readErr != nil {
		t.Fatalf("ReadFile(routeMapFile) error = %v", readErr)
	}
	if !strings.Contains(string(content), "LogicSlot: 10\n    DBKey: order-db-0\n    TableSuffix: 00\n    Status: backfilling") {
		t.Fatalf("expected slot 10 to remain backfilling before verify scan completes, content=%s", string(content))
	}
	if !strings.Contains(readCheckpointFile(t, verifyCheckpointFile), "\"last_order_id\":8501") {
		t.Fatalf("expected verify checkpoint to record first verified order id")
	}
}
