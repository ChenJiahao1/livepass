package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"livepass/pkg/xmysql"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/sharding"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var testRepositoryMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/livepass_order_repository?parseTime=true"
var testRepositoryMySQLShard1DataSource = "root:123456@tcp(127.0.0.1:3306)/livepass_order_repository_shard1?parseTime=true"

func TestOrderRepositoryWritesAndQueriesShardTables(t *testing.T) {
	deps := newTestOrderRepositoryDeps(t, buildFullRouteEntries("00", "01"))
	resetOrderRepositoryState(t)

	repo := NewOrderRepository(deps)
	userID := int64(3001)
	orderNumber := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 10, 0, 0, 0, time.UTC), 1, 1)

	err := repo.TransactByUserID(context.Background(), userID, func(ctx context.Context, tx OrderTx) error {
		if err := tx.InsertOrder(ctx, buildRepositoryOrder(orderNumber, userID)); err != nil {
			return err
		}
		if err := tx.InsertOrderTickets(ctx, buildRepositoryTickets(orderNumber, userID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("TransactByUserID() error = %v", err)
	}

	route, err := repo.RouteByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("RouteByUserID() error = %v", err)
	}
	if countTableRows(t, testRepositoryMySQLDataSource, "d_order_"+route.TableSuffix) != 1 {
		t.Fatalf("expected shard d_order_%s to contain 1 row", route.TableSuffix)
	}
	if countTableRows(t, testRepositoryMySQLDataSource, "d_order_ticket_user_"+route.TableSuffix) != 2 {
		t.Fatalf("expected shard d_order_ticket_user_%s to contain 2 rows", route.TableSuffix)
	}

	order, err := repo.FindOrderByNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("FindOrderByNumber() error = %v", err)
	}
	if order.OrderNumber != orderNumber {
		t.Fatalf("order number = %d, want %d", order.OrderNumber, orderNumber)
	}

	tickets, err := repo.FindOrderTicketsByNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("FindOrderTicketsByNumber() error = %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("ticket count = %d, want 2", len(tickets))
	}

	orders, total, err := repo.FindOrderPageByUser(context.Background(), userID, 1, 1, 10)
	if err != nil {
		t.Fatalf("FindOrderPageByUser() error = %v", err)
	}
	if total != 1 || len(orders) != 1 {
		t.Fatalf("page result total=%d len=%d, want 1/1", total, len(orders))
	}
}

func TestOrderRepositoryFindOrderPageByUserReadsCurrentShardTable(t *testing.T) {
	deps := newTestOrderRepositoryDeps(t, buildFullRouteEntries("00", "01"))
	resetOrderRepositoryState(t)

	repo := NewOrderRepository(deps)
	userID := int64(3002)
	orderNumber1 := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 10, 5, 0, 0, time.UTC), 1, 2)
	orderNumber2 := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 10, 6, 0, 0, time.UTC), 1, 3)

	route, err := repo.RouteByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("RouteByUserID() error = %v", err)
	}

	firstOrder := buildRepositoryOrder(orderNumber1, userID)
	firstOrder.CreateOrderTime = time.Date(2026, time.March, 24, 10, 5, 0, 0, time.UTC)
	secondOrder := buildRepositoryOrder(orderNumber2, userID)
	secondOrder.CreateOrderTime = time.Date(2026, time.March, 24, 10, 6, 0, 0, time.UTC)
	secondOrder.OrderStatus = 2

	seedRepositoryOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, firstOrder, secondOrder)

	orders, total, err := repo.FindOrderPageByUser(context.Background(), userID, 0, 1, 10)
	if err != nil {
		t.Fatalf("FindOrderPageByUser() error = %v", err)
	}
	if total != 2 || len(orders) != 2 {
		t.Fatalf("page result total=%d len=%d, want 2/2", total, len(orders))
	}
	if orders[0].OrderNumber != orderNumber2 || orders[1].OrderNumber != orderNumber1 {
		t.Fatalf("unexpected order list sequence: %+v", orders)
	}
}

func TestOrderRepositoryFindOrderByNumberReadsCurrentShardTable(t *testing.T) {
	deps := newTestOrderRepositoryDeps(t, buildFullRouteEntries("00", "01"))
	resetOrderRepositoryState(t)

	repo := NewOrderRepository(deps)
	userID := mustFindUserIDByLogicSlot(t, 10)
	orderNumber := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 11, 30, 0, 0, time.UTC), 1, 20)
	route, err := repo.RouteByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("RouteByUserID() error = %v", err)
	}

	orderFixture := buildRepositoryOrder(orderNumber, userID)
	orderFixture.OrderPrice = 399
	seedRepositoryOrderFixturesIntoTable(t, "d_order_"+route.TableSuffix, orderFixture)

	order, err := repo.FindOrderByNumber(context.Background(), orderNumber)
	if err != nil {
		t.Fatalf("FindOrderByNumber() error = %v", err)
	}
	if order.OrderPrice != 399 {
		t.Fatalf("order price = %v, want shard value 399", order.OrderPrice)
	}
}

func TestOrderRepositoryFindExpiredUnpaidBySlotReadsOnlyTargetShard(t *testing.T) {
	deps := newTestOrderRepositoryDeps(t, buildFullRouteEntries("00", "01"))
	resetOrderRepositoryState(t)

	repo := NewOrderRepository(deps)

	slot0UserID := mustFindUserIDByLogicSlot(t, 10)
	slot1UserID := mustFindUserIDByLogicSlot(t, 11)
	slot0OrderNumber := sharding.BuildOrderNumber(slot0UserID, time.Date(2026, time.March, 24, 11, 0, 0, 0, time.UTC), 1, 10)
	slot1OrderNumber := sharding.BuildOrderNumber(slot1UserID, time.Date(2026, time.March, 24, 11, 1, 0, 0, time.UTC), 1, 11)

	seedRepositoryOrderFixturesIntoTable(
		t,
		"d_order_00",
		buildRepositoryOrderWithExpiry(slot0OrderNumber, slot0UserID, time.Date(2026, time.March, 24, 10, 0, 0, 0, time.Local)),
	)
	seedRepositoryOrderFixturesIntoTable(
		t,
		"d_order_01",
		buildRepositoryOrderWithExpiry(slot1OrderNumber, slot1UserID, time.Date(2026, time.March, 24, 10, 0, 0, 0, time.Local)),
	)

	orders, err := repo.FindExpiredUnpaidBySlot(context.Background(), 10, time.Date(2026, time.March, 24, 12, 0, 0, 0, time.Local), 10)
	if err != nil {
		t.Fatalf("FindExpiredUnpaidBySlot() error = %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expired order count = %d, want 1", len(orders))
	}
	if orders[0].OrderNumber != slot0OrderNumber {
		t.Fatalf("expired order number = %d, want %d", orders[0].OrderNumber, slot0OrderNumber)
	}
}

func TestWalkActiveUserGuardsByShowTimeAcrossShards(t *testing.T) {
	deps := newTestOrderRepositoryDepsWithDataSources(t, buildFullRouteEntries("00", "01"), map[string]string{
		"order-db-0": testRepositoryMySQLDataSource,
		"order-db-1": testRepositoryMySQLShard1DataSource,
	})
	resetOrderRepositoryStateForDataSources(t, testRepositoryMySQLDataSource, testRepositoryMySQLShard1DataSource)

	repo := NewOrderRepository(deps)
	showTimeID := int64(88001)
	now := time.Date(2026, time.April, 13, 10, 0, 0, 0, time.UTC)
	seedRepositoryUserGuardFixturesIntoDataSource(t, testRepositoryMySQLDataSource, buildRepositoryUserGuard(70001, 91001, showTimeID, 3001, now))
	seedRepositoryUserGuardFixturesIntoDataSource(t, testRepositoryMySQLShard1DataSource, buildRepositoryUserGuard(70002, 91002, showTimeID, 3002, now.Add(time.Second)))

	var got []int64
	err := repo.WalkActiveUserGuardsByShowTime(context.Background(), showTimeID, 1, func(rows []*model.DOrderUserGuard) error {
		for _, row := range rows {
			if row == nil {
				continue
			}
			got = append(got, row.UserId)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkActiveUserGuardsByShowTime() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 active user guards, got %v", got)
	}
}

func TestWalkActiveViewerGuardsByShowTimeAcrossShards(t *testing.T) {
	deps := newTestOrderRepositoryDepsWithDataSources(t, buildFullRouteEntries("00", "01"), map[string]string{
		"order-db-0": testRepositoryMySQLDataSource,
		"order-db-1": testRepositoryMySQLShard1DataSource,
	})
	resetOrderRepositoryStateForDataSources(t, testRepositoryMySQLDataSource, testRepositoryMySQLShard1DataSource)

	repo := NewOrderRepository(deps)
	showTimeID := int64(88002)
	now := time.Date(2026, time.April, 13, 10, 5, 0, 0, time.UTC)
	seedRepositoryViewerGuardFixturesIntoDataSource(t, testRepositoryMySQLDataSource, buildRepositoryViewerGuard(71001, 92001, showTimeID, 4001, now))
	seedRepositoryViewerGuardFixturesIntoDataSource(t, testRepositoryMySQLShard1DataSource, buildRepositoryViewerGuard(71002, 92002, showTimeID, 4002, now.Add(time.Second)))

	var got []int64
	err := repo.WalkActiveViewerGuardsByShowTime(context.Background(), showTimeID, 1, func(rows []*model.DOrderViewerGuard) error {
		for _, row := range rows {
			if row == nil {
				continue
			}
			got = append(got, row.ViewerId)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkActiveViewerGuardsByShowTime() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 active viewer guards, got %v", got)
	}
}

func newTestOrderRepositoryDeps(t *testing.T, entries []sharding.RouteEntry) Dependencies {
	t.Helper()

	return newTestOrderRepositoryDepsWithDataSources(t, entries, map[string]string{
		"order-db-0": testRepositoryMySQLDataSource,
		"order-db-1": testRepositoryMySQLDataSource,
	})
}

func newTestOrderRepositoryDepsWithDataSources(t *testing.T, entries []sharding.RouteEntry, dataSources map[string]string) Dependencies {
	t.Helper()

	routeMap, err := sharding.NewRouteMap("v1", entries)
	if err != nil {
		t.Fatalf("NewRouteMap() error = %v", err)
	}

	shardConns := make(map[string]sqlx.SqlConn, len(dataSources))
	for dbKey, dataSource := range dataSources {
		shardConns[dbKey] = sqlx.NewMysql(xmysql.WithLocalTime(dataSource))
	}

	return Dependencies{
		ShardConns: shardConns,
		RouteMap:   routeMap,
		Router:     sharding.NewStaticRouter(routeMap),
	}
}

func buildFullRouteEntries(suffix0, suffix1 string) []sharding.RouteEntry {
	entries := make([]sharding.RouteEntry, 0, 1024)
	for slot := 0; slot < 1024; slot++ {
		entry := sharding.RouteEntry{
			Version:     "v1",
			LogicSlot:   slot,
			Status:      sharding.RouteStatusStable,
			WriteMode:   sharding.WriteModeShardPrimary,
			DBKey:       "order-db-0",
			TableSuffix: suffix0,
		}
		if slot%2 == 1 {
			entry.DBKey = "order-db-1"
			entry.TableSuffix = suffix1
		}
		entries = append(entries, entry)
	}

	return entries
}

func buildRepositoryOrder(orderNumber, userID int64) *model.DOrder {
	now := time.Date(2026, time.March, 24, 10, 0, 0, 0, time.UTC)
	return &model.DOrder{
		Id:                      orderNumber + 1000,
		OrderNumber:             orderNumber,
		ProgramId:               10001,
		ProgramTitle:            "订单分片测试演出",
		ProgramItemPicture:      "https://example.com/program.jpg",
		ProgramPlace:            "测试场馆",
		ProgramShowTime:         now.Add(24 * time.Hour),
		ProgramPermitChooseSeat: 0,
		UserId:                  userID,
		DistributionMode:        "express",
		TakeTicketMode:          "paper",
		TicketCount:             2,
		OrderPrice:              598,
		OrderStatus:             1,
		FreezeToken:             "freeze-token",
		OrderExpireTime:         now.Add(15 * time.Minute),
		CreateOrderTime:         now,
		CreateTime:              now,
		EditTime:                now,
		Status:                  1,
	}
}

func buildRepositoryOrderWithExpiry(orderNumber, userID int64, expireAt time.Time) *model.DOrder {
	order := buildRepositoryOrder(orderNumber, userID)
	order.OrderExpireTime = expireAt
	order.CreateOrderTime = expireAt.Add(-15 * time.Minute)
	order.CreateTime = order.CreateOrderTime
	order.EditTime = order.CreateOrderTime
	return order
}

func buildRepositoryTickets(orderNumber, userID int64) []*model.DOrderTicketUser {
	now := time.Date(2026, time.March, 24, 10, 0, 0, 0, time.UTC)
	return []*model.DOrderTicketUser{
		{
			Id:                 orderNumber + 2001,
			OrderNumber:        orderNumber,
			UserId:             userID,
			TicketUserId:       701,
			TicketUserName:     "张三",
			TicketUserIdNumber: "110101199001011234",
			TicketCategoryId:   40001,
			TicketCategoryName: "普通票",
			TicketPrice:        299,
			SeatId:             5001,
			SeatRow:            1,
			SeatCol:            1,
			SeatPrice:          299,
			OrderStatus:        1,
			CreateOrderTime:    now,
			CreateTime:         now,
			EditTime:           now,
			Status:             1,
		},
		{
			Id:                 orderNumber + 2002,
			OrderNumber:        orderNumber,
			UserId:             userID,
			TicketUserId:       702,
			TicketUserName:     "李四",
			TicketUserIdNumber: "110101199001011235",
			TicketCategoryId:   40001,
			TicketCategoryName: "普通票",
			TicketPrice:        299,
			SeatId:             5002,
			SeatRow:            1,
			SeatCol:            2,
			SeatPrice:          299,
			OrderStatus:        1,
			CreateOrderTime:    now,
			CreateTime:         now,
			EditTime:           now,
			Status:             1,
		},
	}
}

func buildRepositoryUserGuard(id, orderNumber, showTimeID, userID int64, now time.Time) *model.DOrderUserGuard {
	return &model.DOrderUserGuard{
		Id:          id,
		OrderNumber: orderNumber,
		ProgramId:   10001,
		ShowTimeId:  showTimeID,
		UserId:      userID,
		CreateTime:  now,
		EditTime:    now,
		Status:      1,
	}
}

func buildRepositoryViewerGuard(id, orderNumber, showTimeID, viewerID int64, now time.Time) *model.DOrderViewerGuard {
	return &model.DOrderViewerGuard{
		Id:          id,
		OrderNumber: orderNumber,
		ProgramId:   10001,
		ShowTimeId:  showTimeID,
		ViewerId:    viewerID,
		CreateTime:  now,
		EditTime:    now,
		Status:      1,
	}
}

func resetOrderRepositoryState(t *testing.T) {
	t.Helper()

	resetOrderRepositoryStateForDataSources(t, testRepositoryMySQLDataSource)
}

func resetOrderRepositoryStateForDataSources(t *testing.T, dataSources ...string) {
	t.Helper()

	for _, dataSource := range dataSources {
		db := openRepositoryTestDB(t, dataSource)
		for _, relativePath := range []string{
			"sql/order/sharding/d_order_shards.sql",
			"sql/order/sharding/d_order_ticket_user_shards.sql",
			"sql/order/sharding/d_order_user_guard.sql",
			"sql/order/sharding/d_order_viewer_guard.sql",
			"sql/order/sharding/d_order_seat_guard.sql",
		} {
			execRepositorySQLFile(t, db, relativePath)
		}
		_ = db.Close()
	}
}

func openRepositoryTestDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	ensureRepositoryTestDatabase(t, dataSource)

	db, err := sql.Open("mysql", xmysql.WithLocalTime(dataSource))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("db.Ping() error = %v", err)
	}

	return db
}

func ensureRepositoryTestDatabase(t *testing.T, dataSource string) {
	t.Helper()

	cfg, err := mysqlDriver.ParseDSN(dataSource)
	if err != nil {
		t.Fatalf("mysql parse dsn error: %v", err)
	}
	if cfg.DBName == "" {
		t.Fatalf("expected mysql dsn to include database name: %s", dataSource)
	}

	dbName := cfg.DBName
	cfg.DBName = ""

	adminDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("sql.Open admin db error = %v", err)
	}
	defer adminDB.Close()

	if err := adminDB.Ping(); err != nil {
		t.Fatalf("admin db ping error = %v", err)
	}

	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci",
		strings.ReplaceAll(dbName, "`", "``"),
	)
	if _, err := adminDB.Exec(stmt); err != nil {
		t.Fatalf("create test database error = %v", err)
	}
}

func seedRepositoryOrderFixturesIntoTable(t *testing.T, table string, fixtures ...*model.DOrder) {
	t.Helper()

	db := openRepositoryTestDB(t, testRepositoryMySQLDataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		if fixture == nil {
			continue
		}
		if _, err := db.Exec(
			`INSERT INTO `+table+` (
				id, order_number, program_id, show_time_id, program_title, program_item_picture, program_place, program_show_time,
				program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
				order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.Id,
			fixture.OrderNumber,
			fixture.ProgramId,
			fixture.ShowTimeId,
			fixture.ProgramTitle,
			fixture.ProgramItemPicture,
			fixture.ProgramPlace,
			fixture.ProgramShowTime,
			fixture.ProgramPermitChooseSeat,
			fixture.UserId,
			fixture.DistributionMode,
			fixture.TakeTicketMode,
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.OrderStatus,
			fixture.FreezeToken,
			fixture.OrderExpireTime,
			fixture.CreateOrderTime,
			fixture.CancelOrderTime,
			fixture.PayOrderTime,
			fixture.CreateTime,
			fixture.EditTime,
			fixture.Status,
		); err != nil {
			t.Fatalf("insert fixture into %s error: %v", table, err)
		}
	}
}

func seedRepositoryUserGuardFixturesIntoDataSource(t *testing.T, dataSource string, fixtures ...*model.DOrderUserGuard) {
	t.Helper()

	db := openRepositoryTestDB(t, dataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		if fixture == nil {
			continue
		}
		if _, err := db.Exec(
			`INSERT INTO d_order_user_guard (id, order_number, program_id, show_time_id, user_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.Id,
			fixture.OrderNumber,
			fixture.ProgramId,
			fixture.ShowTimeId,
			fixture.UserId,
			fixture.CreateTime,
			fixture.EditTime,
			fixture.Status,
		); err != nil {
			t.Fatalf("insert user guard fixture error: %v", err)
		}
	}
}

func seedRepositoryViewerGuardFixturesIntoDataSource(t *testing.T, dataSource string, fixtures ...*model.DOrderViewerGuard) {
	t.Helper()

	db := openRepositoryTestDB(t, dataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		if fixture == nil {
			continue
		}
		if _, err := db.Exec(
			`INSERT INTO d_order_viewer_guard (id, order_number, program_id, show_time_id, viewer_id, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.Id,
			fixture.OrderNumber,
			fixture.ProgramId,
			fixture.ShowTimeId,
			fixture.ViewerId,
			fixture.CreateTime,
			fixture.EditTime,
			fixture.Status,
		); err != nil {
			t.Fatalf("insert viewer guard fixture error: %v", err)
		}
	}
}

func mustFindUserIDByLogicSlot(t *testing.T, targetSlot int) int64 {
	t.Helper()

	for userID := int64(1); userID < 1_000_000; userID++ {
		if sharding.LogicSlotByUserID(userID) == targetSlot {
			return userID
		}
	}

	t.Fatalf("failed to find user id for logic slot %d", targetSlot)
	return 0
}

func execRepositorySQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repositoryProjectRoot(t), relativePath))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", relativePath, err)
	}

	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Exec(%s) error = %v\nstatement: %s", relativePath, err, stmt)
		}
	}
}

func countTableRows(t *testing.T, dataSource, table string) int64 {
	t.Helper()

	db := openRepositoryTestDB(t, dataSource)
	defer db.Close()

	var count int64
	if err := db.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s) error = %v", table, err)
	}

	return count
}

func repositoryProjectRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}
