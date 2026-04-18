package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"livepass/pkg/xid"
	"livepass/pkg/xmysql"
	"livepass/pkg/xredis"
	"livepass/services/program-rpc/internal/config"
	"livepass/services/program-rpc/internal/model"
	"livepass/services/program-rpc/internal/programcache"
	"livepass/services/program-rpc/internal/seatcache"
	"livepass/services/program-rpc/internal/svc"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/cache"
	gzredis "github.com/zeromicro/go-zero/core/stores/redis"
)

var testProgramIsolationNamespace = newProgramTestIsolationNamespace()
var testProgramMySQLDataSource = newProgramTestMySQLDataSource()
var testProgramSeatLedgerPrefix = testProgramIsolationNamespace + ":seat-ledger"

const testProgramDateTimeLayout = "2006-01-02 15:04:05"

var (
	programDomainStateMu      sync.Mutex
	programDomainStateOwners  = make(map[string]struct{})
	programDomainStateOwnersM sync.Mutex
)

func init() {
	model.SetCacheKeyNamespace(testProgramIsolationNamespace)
}

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
	InventoryPreheatStatus    int64
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
	acquireProgramDomainState(t)
	ensureProgramTestDatabase(t, testProgramMySQLDataSource)

	_ = xid.Close()
	if err := xid.Init(xid.Config{
		Provider: xid.ProviderStatic,
		NodeID:   901,
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
	svcCtx.RushInventoryPreheatClient = nil

	svcCtx.SeatStockStore = seatcache.NewSeatStockStore(svcCtx.Redis, svcCtx.DSeatModel, seatcache.Config{
		Prefix:          testProgramSeatLedgerPrefix,
		StockTTL:        time.Hour,
		SeatTTL:         time.Hour,
		LoadingCooldown: 200 * time.Millisecond,
	})

	return svcCtx
}

func newProgramTestServiceContextWithRushInventoryPreheat(t *testing.T) *svc.ServiceContext {
	t.Helper()
	acquireProgramDomainState(t)
	ensureProgramTestDatabase(t, testProgramMySQLDataSource)

	_ = xid.Close()
	if err := xid.Init(xid.Config{
		Provider: xid.ProviderStatic,
		NodeID:   901,
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
		RushInventoryPreheat: config.RushInventoryPreheatConfig{
			Enable:    true,
			LeadTime:  5 * time.Minute,
			Queue:     "rush_inventory_preheat",
			MaxRetry:  8,
			UniqueTTL: 30 * time.Minute,
			Redis: xredis.Config{
				Host: "127.0.0.1:6379",
				Type: "node",
			},
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

func startProgramCacheSubscriber(t *testing.T, svcCtx *svc.ServiceContext, channel string, expectedSubs int64) func() {
	t.Helper()

	if svcCtx == nil || svcCtx.Redis == nil || svcCtx.ProgramCacheRegistry == nil {
		t.Fatalf("service context cache invalidation dependencies not ready")
	}

	conf := svcCtx.Config.CacheInvalidation.Normalize()
	if channel == "" {
		channel = conf.Channel
	}

	subscriber := programcache.NewPubSubSubscriber(
		svcCtx.Redis,
		channel,
		svcCtx.ProgramCacheRegistry,
		conf.PublishTimeout,
		conf.ReconnectBackoff,
	)

	ctx, cancel := context.WithCancel(context.Background())
	go subscriber.Start(ctx)
	stop := func() {
		cancel()
		subscriber.Close()
	}
	t.Cleanup(stop)
	waitForProgramCacheSubscribers(t, svcCtx, channel, expectedSubs)

	return stop
}

func configureProgramCachePublisher(t *testing.T, svcCtx *svc.ServiceContext, channel string) {
	t.Helper()

	if svcCtx == nil || svcCtx.Redis == nil || svcCtx.ProgramCacheInvalidator == nil {
		t.Fatalf("service context cache invalidation dependencies not ready")
	}

	conf := svcCtx.Config.CacheInvalidation.Normalize()
	if channel == "" {
		channel = conf.Channel
	}

	publisher, err := programcache.NewRedisPubSubPublisher(svcCtx.Redis, channel, conf.PublishTimeout)
	if err != nil {
		t.Fatalf("new redis pubsub publisher error: %v", err)
	}

	svcCtx.ProgramCacheInvalidator.SetPublisher(publisher)
}

func splitProgramRedisAddrs(t *testing.T, svcCtx *svc.ServiceContext) []string {
	t.Helper()

	if svcCtx == nil || svcCtx.Redis == nil {
		t.Fatalf("service context redis not ready")
	}
	raw := strings.Split(svcCtx.Redis.Addr, ",")
	addrs := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		addrs = append(addrs, item)
	}
	if len(addrs) == 0 {
		t.Fatalf("redis addr is empty")
	}

	return addrs
}

func waitForProgramCacheSubscribers(t *testing.T, svcCtx *svc.ServiceContext, channel string, expectedSubs int64) {
	t.Helper()

	if expectedSubs <= 0 {
		expectedSubs = 1
	}

	addrs := splitProgramRedisAddrs(t, svcCtx)
	opts := &redis.UniversalOptions{
		Addrs:    addrs,
		Username: svcCtx.Redis.User,
		Password: svcCtx.Redis.Pass,
	}
	if strings.EqualFold(svcCtx.Redis.Type, "cluster") {
		opts.IsClusterMode = true
	}

	client := redis.NewUniversalClient(opts)
	t.Cleanup(func() {
		_ = client.Close()
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := client.PubSubNumSub(context.Background(), channel).Result()
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if counts[channel] >= expectedSubs {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected pubsub subscribers >= %d on channel %q before deadline", expectedSubs, channel)
}

func clearProgramSeatLedger(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}
	showTimeID := resolveSeatLedgerShowTimeID(t, svcCtx, programID, ticketCategoryID)
	if err := svcCtx.SeatStockStore.Clear(context.Background(), showTimeID, ticketCategoryID); err != nil {
		t.Fatalf("clear program seat ledger error: %v", err)
	}
}

func primeProgramSeatLedgerFromDB(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}
	showTimeID := resolveSeatLedgerShowTimeID(t, svcCtx, programID, ticketCategoryID)
	if err := svcCtx.SeatStockStore.PrimeFromDB(context.Background(), showTimeID, ticketCategoryID); err != nil {
		t.Fatalf("prime program seat ledger from db error: %v", err)
	}
}

func requireProgramSeatLedgerSnapshot(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) *seatcache.SeatLedgerSnapshot {
	t.Helper()

	if svcCtx.SeatStockStore == nil {
		t.Fatalf("expected seat stock store to be configured")
	}

	showTimeID := resolveSeatLedgerShowTimeID(t, svcCtx, programID, ticketCategoryID)
	snapshot, err := svcCtx.SeatStockStore.Snapshot(context.Background(), showTimeID, ticketCategoryID)
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

func resolveSeatLedgerShowTimeID(t *testing.T, svcCtx *svc.ServiceContext, programID, ticketCategoryID int64) int64 {
	t.Helper()

	if ticketCategoryID > 0 && svcCtx != nil && svcCtx.DTicketCategoryModel != nil {
		ticketCategory, err := svcCtx.DTicketCategoryModel.FindOne(context.Background(), ticketCategoryID)
		if err == nil && ticketCategory != nil && ticketCategory.ShowTimeId > 0 {
			return ticketCategory.ShowTimeId
		}
	}
	if programID > 0 && svcCtx != nil && svcCtx.DProgramShowTimeModel != nil {
		showTime, err := svcCtx.DProgramShowTimeModel.FindFirstByProgramId(context.Background(), programID)
		if err == nil && showTime != nil && showTime.Id > 0 {
			return showTime.Id
		}
	}

	t.Fatalf("resolve seat ledger show time id failed, programID=%d ticketCategoryID=%d", programID, ticketCategoryID)
	return 0
}

func resetProgramDomainState(t *testing.T) {
	t.Helper()
	acquireProgramDomainState(t)

	db := openProgramTestDB(t, testProgramMySQLDataSource)
	defer db.Close()

	for _, relativePath := range []string{
		"sql/program/d_program_category.sql",
		"sql/program/d_program_group.sql",
		"sql/program/d_program.sql",
		"sql/program/d_program_show_time.sql",
		"sql/program/d_delay_task_outbox.sql",
		"sql/program/d_seat.sql",
		"sql/program/d_ticket_category.sql",
		"sql/program/dev_seed.sql",
	} {
		execProgramSQLFile(t, db, relativePath)
	}

	clearProgramRedisState(t)
}

func acquireProgramDomainState(t *testing.T) {
	t.Helper()

	rootName := t.Name()
	if idx := strings.IndexByte(rootName, '/'); idx >= 0 {
		rootName = rootName[:idx]
	}

	programDomainStateOwnersM.Lock()
	if _, ok := programDomainStateOwners[rootName]; ok {
		programDomainStateOwnersM.Unlock()
		return
	}
	programDomainStateOwners[rootName] = struct{}{}
	programDomainStateOwnersM.Unlock()

	programDomainStateMu.Lock()
	t.Cleanup(func() {
		programDomainStateOwnersM.Lock()
		delete(programDomainStateOwners, rootName)
		programDomainStateOwnersM.Unlock()
		programDomainStateMu.Unlock()
	})
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
		model.ProgramCacheKeyPrefix() + "*",
		model.ProgramGroupCacheKeyPrefix() + "*",
		model.ProgramShowTimeCacheKeyPrefix() + "*",
		model.ProgramFirstShowTimeCacheKeyPrefix() + "*",
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

func openProgramTestDB(t *testing.T, dataSource string) *sql.DB {
	t.Helper()

	ensureProgramTestDatabase(t, dataSource)

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

func requireProgramDelayTaskOutbox(t *testing.T, dataSource, taskType, taskKey, executeAt string) {
	t.Helper()

	db := openProgramTestDB(t, dataSource)
	defer db.Close()

	var actualTaskType string
	var actualTaskKey string
	var actualExecuteAt string
	err := db.QueryRow(
		`SELECT task_type, task_key, DATE_FORMAT(execute_at, '%Y-%m-%d %H:%i:%s')
		FROM d_delay_task_outbox
		WHERE task_key = ?
		LIMIT 1`,
		taskKey,
	).Scan(&actualTaskType, &actualTaskKey, &actualExecuteAt)
	if err == sql.ErrNoRows {
		t.Fatalf("delay task outbox row not found for task_key=%s", taskKey)
	}
	if err != nil {
		t.Fatalf("query delay task outbox error: %v", err)
	}

	if actualTaskType != taskType {
		t.Fatalf("expected task_type %s, got %s", taskType, actualTaskType)
	}
	if actualTaskKey != taskKey {
		t.Fatalf("expected task_key %s, got %s", taskKey, actualTaskKey)
	}
	if actualExecuteAt != executeAt {
		t.Fatalf("expected execute_at %s, got %s", executeAt, actualExecuteAt)
	}
}

func ensureProgramTestDatabase(t *testing.T, dataSource string) {
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
		t.Fatalf("sql.Open admin db error: %v", err)
	}
	defer adminDB.Close()

	if err := adminDB.Ping(); err != nil {
		t.Fatalf("admin db ping error: %v", err)
	}

	stmt := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci",
		strings.ReplaceAll(dbName, "`", "``"),
	)
	if _, err := adminDB.Exec(stmt); err != nil {
		t.Fatalf("create test database error: %v", err)
	}
}

func newProgramTestMySQLDataSource() string {
	baseDataSource := xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/livepass_program?parseTime=true")
	cfg, err := mysqlDriver.ParseDSN(baseDataSource)
	if err != nil {
		return baseDataSource
	}

	cfg.DBName = fmt.Sprintf("livepass_program_test_%d_%d", os.Getpid(), time.Now().UnixNano())

	return cfg.FormatDSN()
}

func newProgramTestIsolationNamespace() string {
	return fmt.Sprintf("livepass:test:program:%d:%d", os.Getpid(), time.Now().UnixNano())
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
	if fixture.ShowDayTime == "" {
		if fixture.ShowTime != "" {
			fixture.ShowDayTime = toProgramShowDayTime(fixture.ShowTime)
		} else {
			fixture.ShowDayTime = "2026-06-01 00:00:00"
		}
	}
	if fixture.ShowTimeID == 0 {
		fixture.ShowTimeID = fixture.ProgramID
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

func toProgramShowDayTime(showTime string) string {
	parts := strings.SplitN(showTime, " ", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "2026-06-01 00:00:00"
	}
	return fmt.Sprintf("%s 00:00:00", parts[0])
}

func withSeatFixtureDefaults(fixture seatFixture) seatFixture {
	if fixture.SeatType == 0 {
		fixture.SeatType = 1
	}
	if fixture.ShowTimeID == 0 {
		fixture.ShowTimeID = fixture.ProgramID
	}
	if fixture.Price == 0 {
		fixture.Price = 299
	}
	if fixture.SeatStatus == 0 {
		fixture.SeatStatus = 1
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
			high_heat, program_status, issue_time, rush_sale_open_time, rush_sale_end_time, inventory_preheat_status,
			create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		nullIfEmpty(fixture.RushSaleOpenTime),
		nullIfEmpty(fixture.RushSaleEndTime),
		fixture.InventoryPreheatStatus,
		"2026-01-01 00:00:00",
		"2026-01-01 00:00:00",
		1,
	)

	mustExecProgramSQL(
		t,
		db,
		`INSERT INTO d_program_show_time (
			id, program_id, show_time, show_day_time, show_week_time, show_end_time, create_time, edit_time, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		showTimeID,
		fixture.ProgramID,
		fixture.ShowTime,
		fixture.ShowDayTime,
		fixture.ShowWeekTime,
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
