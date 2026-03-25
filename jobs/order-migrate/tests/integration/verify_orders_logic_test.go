package integration_test

import (
	"context"
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
