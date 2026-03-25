package integration_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	jobconfig "damai-go/jobs/order-migrate/internal/config"
	jobsvc "damai-go/jobs/order-migrate/internal/svc"
	"damai-go/pkg/xmysql"
	"damai-go/services/order-rpc/sharding"

	_ "github.com/go-sql-driver/mysql"
)

var testOrderMigrateMySQLDataSource = xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true")

type migrateOrderFixture struct {
	ID              int64
	OrderNumber     int64
	ProgramID       int64
	UserID          int64
	OrderStatus     int64
	FreezeToken     string
	OrderExpireTime string
	CreateOrderTime string
	OrderPrice      int64
	TicketCount     int64
}

type migrateOrderTicketFixture struct {
	ID               int64
	OrderNumber      int64
	UserID           int64
	TicketUserID     int64
	TicketCategoryID int64
	SeatID           int64
	SeatRow          int64
	SeatCol          int64
	OrderStatus      int64
	CreateOrderTime  string
	TicketPrice      int64
	SeatPrice        int64
}

type migrateUserOrderIndexFixture struct {
	ID              int64
	OrderNumber     int64
	UserID          int64
	ProgramID       int64
	OrderStatus     int64
	TicketCount     int64
	OrderPrice      int64
	CreateOrderTime string
}

func newOrderMigrateTestServiceContext(t *testing.T, cfg jobconfig.Config) *jobsvc.ServiceContext {
	t.Helper()

	svcCtx, err := jobsvc.NewServiceContext(cfg)
	if err != nil {
		t.Fatalf("NewServiceContext() error = %v", err)
	}
	return svcCtx
}

func newOrderMigrateTestConfig(t *testing.T, routeMapFile, checkpointFile string) jobconfig.Config {
	t.Helper()

	return jobconfig.Config{
		LegacyMySQL: jobconfig.MySQLConfig{
			DataSource: testOrderMigrateMySQLDataSource,
		},
		Shards: map[string]jobconfig.MySQLConfig{
			"order-db-0": {DataSource: testOrderMigrateMySQLDataSource},
			"order-db-1": {DataSource: testOrderMigrateMySQLDataSource},
		},
		RouteMap: jobconfig.RouteMapConfig{
			File: routeMapFile,
		},
		Backfill: jobconfig.BackfillConfig{
			BatchSize:      1,
			CheckpointFile: checkpointFile,
			Slots:          []int{10, 11},
		},
		Verify: jobconfig.VerifyConfig{
			SampleSize:     1,
			BatchSize:      100,
			CheckpointFile: t.TempDir() + "/verify.checkpoint.json",
			Slots:          []int{10},
		},
		Switch: jobconfig.SwitchConfig{
			Slots: []int{10},
		},
		Rollback: jobconfig.RollbackConfig{
			Slots: []int{10},
		},
	}
}

func writeOrderMigrateRouteMapFile(t *testing.T, version string, entries []jobconfig.RouteEntryConfig) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "route_map.yaml")
	content := &strings.Builder{}
	content.WriteString("Version: " + version + "\n")
	content.WriteString("Entries:\n")
	for _, entry := range entries {
		content.WriteString("  - Version: " + entry.Version + "\n")
		content.WriteString("    LogicSlot: " + itoa(entry.LogicSlot) + "\n")
		content.WriteString("    DBKey: " + entry.DBKey + "\n")
		content.WriteString("    TableSuffix: " + entry.TableSuffix + "\n")
		content.WriteString("    Status: " + entry.Status + "\n")
		content.WriteString("    WriteMode: " + entry.WriteMode + "\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
	return path
}

func buildOrderMigrateRouteEntries() []jobconfig.RouteEntryConfig {
	return []jobconfig.RouteEntryConfig{
		{
			Version:     "v1",
			LogicSlot:   10,
			DBKey:       "order-db-0",
			TableSuffix: "00",
			Status:      sharding.RouteStatusVerifying,
			WriteMode:   sharding.WriteModeDualWrite,
		},
		{
			Version:     "v1",
			LogicSlot:   11,
			DBKey:       "order-db-1",
			TableSuffix: "01",
			Status:      sharding.RouteStatusVerifying,
			WriteMode:   sharding.WriteModeDualWrite,
		},
	}
}

func resetOrderMigrateState(t *testing.T) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	for _, relativePath := range []string{
		"sql/order/d_order.sql",
		"sql/order/d_order_ticket_user.sql",
		"sql/order/d_user_order_index.sql",
		"sql/order/d_order_route_legacy.sql",
		"sql/order/sharding/d_order_shards.sql",
		"sql/order/sharding/d_order_ticket_user_shards.sql",
		"sql/order/sharding/d_user_order_index_shards.sql",
	} {
		execOrderMigrateSQLFile(t, db, relativePath)
	}
}

func seedLegacyOrderFixtures(t *testing.T, fixtures ...migrateOrderFixture) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withMigrateOrderDefaults(fixture)
		if _, err := db.Exec(
			`INSERT INTO d_order (
				id, order_number, program_id, program_title, program_item_picture, program_place, program_show_time,
				program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
				order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.ProgramID,
			"迁移测试演出",
			"https://example.com/migrate.jpg",
			"测试场馆",
			"2026-12-31 19:30:00",
			0,
			fixture.UserID,
			"express",
			"paper",
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.OrderStatus,
			fixture.FreezeToken,
			fixture.OrderExpireTime,
			fixture.CreateOrderTime,
			nil,
			nil,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		); err != nil {
			t.Fatalf("seed legacy order error: %v", err)
		}
	}
}

func seedLegacyOrderFixturesIntoTable(t *testing.T, table string, fixture migrateOrderFixture) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	fixture = withMigrateOrderDefaults(fixture)
	if _, err := db.Exec(
		`INSERT INTO `+table+` (
			id, order_number, program_id, program_title, program_item_picture, program_place, program_show_time,
			program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
			order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.ID,
		fixture.OrderNumber,
		fixture.ProgramID,
		"迁移测试演出",
		"https://example.com/migrate.jpg",
		"测试场馆",
		"2026-12-31 19:30:00",
		0,
		fixture.UserID,
		"express",
		"paper",
		fixture.TicketCount,
		fixture.OrderPrice,
		fixture.OrderStatus,
		fixture.FreezeToken,
		fixture.OrderExpireTime,
		fixture.CreateOrderTime,
		nil,
		nil,
		fixture.CreateOrderTime,
		fixture.CreateOrderTime,
		1,
	); err != nil {
		t.Fatalf("seed order into %s error: %v", table, err)
	}
}

func seedLegacyOrderTicketFixtures(t *testing.T, fixtures ...migrateOrderTicketFixture) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withMigrateOrderTicketDefaults(fixture)
		if _, err := db.Exec(
			`INSERT INTO d_order_ticket_user (
				id, order_number, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
				ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
				seat_price, order_status, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.TicketUserID,
			"张三",
			"110101199001011234",
			fixture.TicketCategoryID,
			"普通票",
			fixture.TicketPrice,
			fixture.SeatID,
			fixture.SeatRow,
			fixture.SeatCol,
			fixture.SeatPrice,
			fixture.OrderStatus,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		); err != nil {
			t.Fatalf("seed legacy ticket error: %v", err)
		}
	}
}

func seedShardOrderTicketFixturesIntoTable(t *testing.T, table string, fixtures ...migrateOrderTicketFixture) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withMigrateOrderTicketDefaults(fixture)
		if _, err := db.Exec(
			`INSERT INTO `+table+` (
				id, order_number, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
				ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
				seat_price, order_status, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.TicketUserID,
			"张三",
			"110101199001011234",
			fixture.TicketCategoryID,
			"普通票",
			fixture.TicketPrice,
			fixture.SeatID,
			fixture.SeatRow,
			fixture.SeatCol,
			fixture.SeatPrice,
			fixture.OrderStatus,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		); err != nil {
			t.Fatalf("seed shard ticket into %s error: %v", table, err)
		}
	}
}

func seedLegacyUserOrderIndexFixtures(t *testing.T, fixtures ...migrateUserOrderIndexFixture) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withMigrateUserOrderIndexDefaults(fixture)
		if _, err := db.Exec(
			`INSERT INTO d_user_order_index (
				id, order_number, user_id, program_id, order_status, ticket_count, order_price, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.ProgramID,
			fixture.OrderStatus,
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		); err != nil {
			t.Fatalf("seed legacy user order index error: %v", err)
		}
	}
}

func seedShardUserOrderIndexFixturesIntoTable(t *testing.T, table string, fixtures ...migrateUserOrderIndexFixture) {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withMigrateUserOrderIndexDefaults(fixture)
		if _, err := db.Exec(
			`INSERT INTO `+table+` (
				id, order_number, user_id, program_id, order_status, ticket_count, order_price, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.ProgramID,
			fixture.OrderStatus,
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		); err != nil {
			t.Fatalf("seed shard user order index into %s error: %v", table, err)
		}
	}
}

func countTableRowsByOrderNumber(t *testing.T, table string, orderNumber int64) int64 {
	t.Helper()

	db := openOrderMigrateTestDB(t)
	defer db.Close()

	var count int64
	if err := db.QueryRow("SELECT COUNT(1) FROM "+table+" WHERE order_number = ?", orderNumber).Scan(&count); err != nil {
		t.Fatalf("QueryRow count error: %v", err)
	}
	return count
}

func readCheckpointFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func routeForUser(userID int64) sharding.Route {
	entries := buildOrderMigrateRouteEntries()
	slot := sharding.LogicSlotByUserID(userID)
	for _, entry := range entries {
		if entry.LogicSlot == slot {
			return sharding.Route{
				LogicSlot:   entry.LogicSlot,
				DBKey:       entry.DBKey,
				TableSuffix: entry.TableSuffix,
				Version:     entry.Version,
				Status:      entry.Status,
				WriteMode:   entry.WriteMode,
			}
		}
	}
	return sharding.Route{}
}

func mustFindUserIDByLogicSlotForMigrate(t *testing.T, targetSlot int) int64 {
	t.Helper()

	for userID := int64(1); userID < 1_000_000; userID++ {
		if sharding.LogicSlotByUserID(userID) == targetSlot {
			return userID
		}
	}
	t.Fatalf("failed to find user id for logic slot %d", targetSlot)
	return 0
}

func openOrderMigrateTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", xmysql.WithLocalTime(testOrderMigrateMySQLDataSource))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("db.Ping() error = %v", err)
	}
	return db
}

func execOrderMigrateSQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(orderMigrateProjectRoot(t), relativePath))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", relativePath, err)
	}
	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %s error: %v\nstatement: %s", relativePath, err, stmt)
		}
	}
}

func orderMigrateProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func withMigrateOrderDefaults(fixture migrateOrderFixture) migrateOrderFixture {
	if fixture.ProgramID == 0 {
		fixture.ProgramID = 10001
	}
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = 1
	}
	if fixture.FreezeToken == "" {
		fixture.FreezeToken = "freeze-migrate"
	}
	if fixture.OrderExpireTime == "" {
		fixture.OrderExpireTime = "2026-01-01 00:15:00"
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}
	if fixture.OrderPrice == 0 {
		fixture.OrderPrice = 299
	}
	if fixture.TicketCount == 0 {
		fixture.TicketCount = 1
	}
	return fixture
}

func withMigrateOrderTicketDefaults(fixture migrateOrderTicketFixture) migrateOrderTicketFixture {
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = 1
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}
	if fixture.TicketPrice == 0 {
		fixture.TicketPrice = 299
	}
	if fixture.SeatPrice == 0 {
		fixture.SeatPrice = 299
	}
	if fixture.TicketCategoryID == 0 {
		fixture.TicketCategoryID = 40001
	}
	return fixture
}

func withMigrateUserOrderIndexDefaults(fixture migrateUserOrderIndexFixture) migrateUserOrderIndexFixture {
	if fixture.ProgramID == 0 {
		fixture.ProgramID = 10001
	}
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = 1
	}
	if fixture.TicketCount == 0 {
		fixture.TicketCount = 1
	}
	if fixture.OrderPrice == 0 {
		fixture.OrderPrice = 299
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}
	return fixture
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func mustParseMigrateTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		t.Fatalf("time.ParseInLocation(%q) error = %v", value, err)
	}
	return parsed
}
