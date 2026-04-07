package integration_test

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

	"damai-go/pkg/xid"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/program-rpc/internal/config"
	"damai-go/services/program-rpc/internal/seatcache"
	"damai-go/services/program-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/cache"
	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
)

var testProgramMySQLDataSource = xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/damai_program?parseTime=true")

const testProgramSeatLedgerPrefix = "damai-go:test:program:seat-ledger"
const testProgramDateTimeLayout = "2006-01-02 15:04:05"

type ticketCategoryFixture struct {
	ID           int64
	ShowTimeID   int64
	Introduce    string
	Price        float64
	TotalNumber  int64
	RemainNumber int64
}

type seatFixture struct {
	ID               int64
	ProgramID        int64
	ShowTimeID       int64
	TicketCategoryID int64
	RowCode          int
	ColCode          int
	SeatType         int
	Price            float64
	SeatStatus       int
	FreezeToken      string
	FreezeExpireTime string
}

type seatFreezeFixture struct {
	ID               int64
	FreezeToken      string
	RequestNo        string
	ProgramID        int64
	TicketCategoryID int64
	OwnerOrderNumber int64
	OwnerEpoch       int64
	SeatCount        int
	FreezeStatus     int
	ExpireTime       string
	ReleaseReason    string
	ReleaseTime      string
}

type programFixture struct {
	ProgramID                 int64
	ProgramGroupID            int64
	ParentProgramCategoryID   int64
	ProgramCategoryID         int64
	AreaID                    int64
	Prime                     int64
	Title                     string
	Actor                     string
	Place                     string
	ItemPicture               string
	Detail                    string
	HighHeat                  int64
	IssueTime                 string
	ShowTimeID                int64
	ShowTime                  string
	ShowDayTime               string
	ShowWeekTime              string
	RushSaleOpenTime          string
	RushSaleEndTime           string
	ShowEndTime               string
	PermitRefund              int64
	RefundTicketRule          string
	RefundExplain             string
	RefundRuleJSON            string
	GroupAreaName             string
	ProgramSimpleInfoAreaName string
	TicketCategories          []ticketCategoryFixture
}

func newProgramTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	_ = xid.Close()
	if err := xid.InitEtcd(context.Background(), xid.Config{
		Hosts:   []string{"127.0.0.1:2379"},
		Prefix:  "/damai-go/tests/snowflake/program-rpc/",
		Service: "program-rpc-test",
	}); err != nil {
		t.Fatalf("init xid error: %v", err)
	}
	t.Cleanup(func() {
		_ = xid.Close()
	})

	svcCtx := svc.NewServiceContext(config.Config{
		MySQL: xmysql.Config{
			DataSource: testProgramMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		},
		Cache: cache.CacheConf{
			{
				RedisConf: gzredis.RedisConf{
					Host: "127.0.0.1:6379",
					Type: "node",
				},
				Weight: 100,
			},
		},
		LocalCache: config.LocalCacheConfig{
			DetailTTL:           20 * time.Second,
			DetailNotFoundTTL:   5 * time.Second,
			DetailCacheLimit:    5000,
			CategorySnapshotTTL: 5 * time.Minute,
		},
	})

	svcCtx.SeatStockStore = seatcache.NewSeatStockStore(svcCtx.Redis, svcCtx.DSeatModel, seatcache.Config{
		Prefix:          testProgramSeatLedgerPrefix,
		StockTTL:        time.Hour,
		SeatTTL:         time.Hour,
		LoadingCooldown: 200 * time.Millisecond,
	})

	return svcCtx
}

func clearProgramSeatLedger(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}
	if err := svcCtx.SeatStockStore.Clear(context.Background(), programID, ticketCategoryID); err != nil {
		t.Fatalf("clear program seat ledger error: %v", err)
	}
}

func primeProgramSeatLedgerFromDB(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}
	if err := svcCtx.SeatStockStore.PrimeFromDB(context.Background(), programID, ticketCategoryID); err != nil {
		t.Fatalf("prime program seat ledger from db error: %v", err)
	}
}

func requireProgramSeatLedgerSnapshot(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) *seatcache.SeatLedgerSnapshot {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}

	snapshot, err := svcCtx.SeatStockStore.Snapshot(context.Background(), programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("snapshot program seat ledger error: %v", err)
	}

	return snapshot
}

func waitProgramSeatLedgerReady(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID, expectedAvailableCount int64) *seatcache.SeatLedgerSnapshot {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := requireProgramSeatLedgerSnapshot(t, svcCtx, programID, ticketCategoryID)
		if snapshot.Ready && snapshot.AvailableCount == expectedAvailableCount {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("program seat ledger was not ready before deadline, programID=%d ticketCategoryID=%d", programID, ticketCategoryID)
	return nil
}

func resetProgramDomainState(t *testing.T) {
	t.Helper()

	db := openProgramTestDB(t, testProgramMySQLDataSource)
	defer db.Close()

	for _, relativePath := range []string{
		"sql/program/d_program_category.sql",
		"sql/program/d_program_group.sql",
		"sql/program/d_program.sql",
		"sql/program/d_program_show_time.sql",
		"sql/program/d_seat.sql",
		"sql/program/d_ticket_category.sql",
		"sql/program/dev_seed.sql",
	} {
		execProgramSQLFile(t, db, relativePath)
	}

	clearProgramRedisState(t)
}

func clearProgramRedisState(t *testing.T) {
	t.Helper()

	redis, err := xredis.New(xredis.Config{
		Host: "127.0.0.1:6379",
		Type: "node",
	})
	if err != nil {
		t.Fatalf("new program test redis client error: %v", err)
	}

	for _, pattern := range []string{
		"cache:dProgram:id:*",
		"cache:dProgramGroup:id:*",
		"cache:dProgramShowTime:first:programId:*",
		testProgramSeatLedgerPrefix + ":*",
	} {
		keys, err := redis.KeysCtx(context.Background(), pattern)
		if err != nil {
			t.Fatalf("list program redis keys by pattern %q error: %v", pattern, err)
		}
		if len(keys) == 0 {
			continue
		}
		if _, err := redis.DelCtx(context.Background(), keys...); err != nil {
			t.Fatalf("delete program redis keys by pattern %q error: %v", pattern, err)
		}
	}
}

func seedProgramFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...programFixture) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		insertProgramFixture(t, db, withProgramFixtureDefaults(fixture))
	}
}

func seedSeatFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...seatFixture) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withSeatFixtureDefaults(fixture)
		mustExecProgramSQL(
			t,
			db,
			`INSERT INTO d_seat (
				id, program_id, show_time_id, ticket_category_id, row_code, col_code, seat_type, price, seat_status,
				freeze_token, freeze_expire_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.ProgramID,
			fixture.ShowTimeID,
			fixture.TicketCategoryID,
			fixture.RowCode,
			fixture.ColCode,
			fixture.SeatType,
			fixture.Price,
			fixture.SeatStatus,
			nullIfEmpty(fixture.FreezeToken),
			nullIfEmpty(fixture.FreezeExpireTime),
			"2026-01-01 00:00:00",
			"2026-01-01 00:00:00",
			1,
		)
	}
}

func seedRedisSeatFreezeFixture(t *testing.T, svcCtx *svc.ServiceContext, fixture seatFreezeFixture) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}

	fixture = withSeatFreezeFixtureDefaults(fixture)
	expireTime, err := time.ParseInLocation(testProgramDateTimeLayout, fixture.ExpireTime, time.Local)
	if err != nil {
		t.Fatalf("parse seat freeze expire time error: %v", err)
	}

	if _, err := svcCtx.SeatStockStore.FreezeAutoAssignedSeats(
		context.Background(),
		fixture.ProgramID,
		fixture.TicketCategoryID,
		fixture.FreezeToken,
		int64(fixture.SeatCount),
	); err != nil {
		t.Fatalf("freeze seats in redis error: %v", err)
	}

	meta := &seatcache.SeatFreezeMetadata{
		FreezeToken:      fixture.FreezeToken,
		RequestNo:        fixture.RequestNo,
		ProgramID:        fixture.ProgramID,
		TicketCategoryID: fixture.TicketCategoryID,
		OwnerOrderNumber: fixture.OwnerOrderNumber,
		OwnerEpoch:       fixture.OwnerEpoch,
		SeatCount:        int64(fixture.SeatCount),
		FreezeStatus:     int64(fixture.FreezeStatus),
		ExpireAt:         expireTime.Unix(),
		UpdatedAt:        expireTime.Unix(),
	}
	if err := svcCtx.SeatStockStore.SaveFreezeMetadata(context.Background(), meta); err != nil {
		t.Fatalf("save redis seat freeze metadata error: %v", err)
	}
}

func seedSeatInventoryProgram(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) {
	t.Helper()

	seedProgramFixtures(t, svcCtx, programFixture{
		ProgramID:               programID,
		ProgramGroupID:          programID + 1000,
		ParentProgramCategoryID: 1,
		ProgramCategoryID:       11,
		AreaID:                  1,
		Title:                   "座位库存测试演出",
		ShowTime:                "2026-12-31 19:30:00",
		ShowDayTime:             "2026-12-31 00:00:00",
		ShowWeekTime:            "周四",
		TicketCategories: []ticketCategoryFixture{
			{
				ID:           ticketCategoryID,
				Introduce:    "普通票",
				Price:        299,
				TotalNumber:  100,
				RemainNumber: 100,
			},
		},
	})
}

func clearSeatInventoryByProgram(t *testing.T, svcCtx *svc.ServiceContext, programID int64) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(t, db, "DELETE FROM d_seat WHERE program_id = ?", programID)
}

func updateTicketCategoryRemainNumber(t *testing.T, svcCtx *svc.ServiceContext, ticketCategoryID int64, remainNumber int64) {
	t.Helper()

	db := openProgramTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	mustExecProgramSQL(
		t,
		db,
		"UPDATE d_ticket_category SET remain_number = ?, edit_time = ? WHERE id = ?",
		remainNumber,
		time.Now().Format(testProgramDateTimeLayout),
		ticketCategoryID,
	)
}

func requireSeatFreezeMetadataByRequestNo(t *testing.T, svcCtx *svc.ServiceContext, requestNo string) *seatcache.SeatFreezeMetadata {
	t.Helper()

	meta, ok := findSeatFreezeMetadataByRequestNo(t, svcCtx, requestNo)
	if !ok {
		t.Fatalf("seat freeze metadata not found by requestNo=%s", requestNo)
	}

	return meta
}

func requireSeatFreezeMetadataByToken(t *testing.T, svcCtx *svc.ServiceContext, freezeToken string) *seatcache.SeatFreezeMetadata {
	t.Helper()

	meta, ok := findSeatFreezeMetadataByToken(t, svcCtx, freezeToken)
	if !ok {
		t.Fatalf("seat freeze metadata not found by freezeToken=%s", freezeToken)
	}

	return meta
}

func findSeatFreezeMetadataByRequestNo(t *testing.T, svcCtx *svc.ServiceContext, requestNo string) (*seatcache.SeatFreezeMetadata, bool) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}

	meta, err := svcCtx.SeatStockStore.GetFreezeMetadataByRequestNo(context.Background(), requestNo)
	if err != nil {
		t.Fatalf("get seat freeze metadata by request no error: %v", err)
	}

	return meta, meta != nil
}

func findSeatFreezeMetadataByToken(t *testing.T, svcCtx *svc.ServiceContext, freezeToken string) (*seatcache.SeatFreezeMetadata, bool) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}

	meta, err := svcCtx.SeatStockStore.GetFreezeMetadataByToken(context.Background(), freezeToken)
	if err != nil {
		t.Fatalf("get seat freeze metadata by token error: %v", err)
	}

	return meta, meta != nil
}

func openProgramTestDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	db, err := sql.Open("mysql", xmysql.WithLocalTime(dataSource))
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("db.Ping error: %v", err)
	}

	return db
}

func withProgramFixtureDefaults(fixture programFixture) programFixture {
	if fixture.Prime == 0 {
		fixture.Prime = 1
	}
	if fixture.Title == "" {
		fixture.Title = fmt.Sprintf("Program-%d", fixture.ProgramID)
	}
	if fixture.Actor == "" {
		fixture.Actor = "测试艺人"
	}
	if fixture.Place == "" {
		fixture.Place = "测试场馆"
	}
	if fixture.ItemPicture == "" {
		fixture.ItemPicture = fmt.Sprintf("https://example.com/program-%d.jpg", fixture.ProgramID)
	}
	if fixture.Detail == "" {
		fixture.Detail = fmt.Sprintf("<p>fixture detail %d</p>", fixture.ProgramID)
	}
	if fixture.IssueTime == "" {
		fixture.IssueTime = "2026-06-01 09:00:00"
	}
	if fixture.ShowTimeID == 0 {
		fixture.ShowTimeID = fixture.ProgramID + 20000
	}
	if fixture.ShowWeekTime == "" {
		fixture.ShowWeekTime = "周六"
	}
	if fixture.GroupAreaName == "" {
		fixture.GroupAreaName = "测试城市"
	}
	if fixture.ProgramSimpleInfoAreaName == "" {
		fixture.ProgramSimpleInfoAreaName = fixture.GroupAreaName
	}
	if len(fixture.TicketCategories) == 0 {
		fixture.TicketCategories = []ticketCategoryFixture{
			{
				ID:           fixture.ProgramID + 30000,
				Introduce:    "普通票",
				Price:        199,
				TotalNumber:  100,
				RemainNumber: 80,
			},
			{
				ID:           fixture.ProgramID + 30001,
				Introduce:    "VIP票",
				Price:        399,
				TotalNumber:  50,
				RemainNumber: 40,
			},
		}
	}

	return fixture
}

func withSeatFixtureDefaults(fixture seatFixture) seatFixture {
	if fixture.SeatType == 0 {
		fixture.SeatType = 1
	}
	if fixture.ShowTimeID == 0 {
		fixture.ShowTimeID = fixture.ProgramID + 20000
	}
	if fixture.Price == 0 {
		fixture.Price = 299
	}
	if fixture.SeatStatus == 0 {
		fixture.SeatStatus = 1
	}

	return fixture
}

func withSeatFreezeFixtureDefaults(fixture seatFreezeFixture) seatFreezeFixture {
	if fixture.SeatCount == 0 {
		fixture.SeatCount = 1
	}
	if fixture.FreezeStatus == 0 {
		fixture.FreezeStatus = 1
	}
	if fixture.ExpireTime == "" {
		fixture.ExpireTime = "2026-12-31 20:00:00"
	}

	return fixture
}

func insertProgramFixture(t *testing.T, db *sql.DB, fixture programFixture) {
	t.Helper()

	programJSON := fmt.Sprintf(
		`[{"programId":%d,"areaId":%d,"areaIdName":"%s"}]`,
		fixture.ProgramID,
		fixture.AreaID,
		fixture.ProgramSimpleInfoAreaName,
	)
	showTimeID := fixture.ShowTimeID

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_group (id, program_json, recent_show_time, create_time, edit_time, status) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.ProgramGroupID,
		programJSON,
		fixture.ShowTime,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program (
			id, program_group_id, prime, area_id, program_category_id, parent_program_category_id,
			title, actor, place, item_picture, detail, permit_refund, refund_ticket_rule, refund_explain, refund_rule_json,
			high_heat, program_status, issue_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.ProgramID,
		fixture.ProgramGroupID,
		fixture.Prime,
		fixture.AreaID,
		fixture.ProgramCategoryID,
		fixture.ParentProgramCategoryID,
		fixture.Title,
		fixture.Actor,
		fixture.Place,
		fixture.ItemPicture,
		fixture.Detail,
		fixture.PermitRefund,
		nullIfEmpty(fixture.RefundTicketRule),
		nullIfEmpty(fixture.RefundExplain),
		nullIfEmpty(fixture.RefundRuleJSON),
		fixture.HighHeat,
		1,
		fixture.IssueTime,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_show_time (
			id, program_id, show_time, show_day_time, show_week_time,
			rush_sale_open_time, rush_sale_end_time, show_end_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		showTimeID,
		fixture.ProgramID,
		fixture.ShowTime,
		fixture.ShowDayTime,
		fixture.ShowWeekTime,
		nullIfEmpty(fixture.RushSaleOpenTime),
		nullIfEmpty(fixture.RushSaleEndTime),
		nullIfEmpty(fixture.ShowEndTime),
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	for _, ticketCategory := range fixture.TicketCategories {
		ticketShowTimeID := ticketCategory.ShowTimeID
		if ticketShowTimeID == 0 {
			ticketShowTimeID = showTimeID
		}
		mustExecProgramSQL(
			t,
			db,
			`INSERT INTO d_ticket_category (
				id, program_id, show_time_id, introduce, price, total_number, remain_number, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ticketCategory.ID,
			fixture.ProgramID,
			ticketShowTimeID,
			ticketCategory.Introduce,
			ticketCategory.Price,
			ticketCategory.TotalNumber,
			ticketCategory.RemainNumber,
			"2026-01-01 00:00:00",
			"2026-01-01 00:00:00",
			1,
		)
	}
}

func mustExecProgramSQL(t *testing.T, db *sql.DB, stmt string, args ...interface{}) {
	t.Helper()

	if _, err := db.Exec(stmt, args...); err != nil {
		t.Fatalf("db.Exec error: %v\nstatement: %s", err, stmt)
	}
}

func execProgramSQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(programProjectRoot(t), relativePath))
	if err != nil {
		t.Fatalf("ReadFile %s error: %v", relativePath, err)
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

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}

	return s
}

func programProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
