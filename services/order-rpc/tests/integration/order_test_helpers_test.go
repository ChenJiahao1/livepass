package integration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"damai-go/pkg/xid"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/limitcache"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/mq"
	"damai-go/services/order-rpc/internal/repeatguard"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/repository"
	"damai-go/services/order-rpc/sharding"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

var testOrderMySQLDataSource = xmysql.WithLocalTime("root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true")

const testPurchaseLimitLedgerPrefix = "damai-go:test:order:purchase-limit"

const (
	testOrderStatusUnpaid    int64 = 1
	testOrderStatusCancelled int64 = 2
	testOrderStatusPaid      int64 = 3
	testOrderStatusRefunded  int64 = 4
)

type orderFixture struct {
	ID                      int64
	OrderNumber             int64
	ProgramID               int64
	ProgramTitle            string
	ProgramItemPicture      string
	ProgramPlace            string
	ProgramShowTime         string
	ProgramPermitChooseSeat int64
	UserID                  int64
	DistributionMode        string
	TakeTicketMode          string
	TicketCount             int64
	OrderPrice              int64
	OrderStatus             int64
	FreezeToken             string
	OrderExpireTime         string
	CreateOrderTime         string
	CancelOrderTime         string
	PayOrderTime            string
}

type orderTicketUserFixture struct {
	ID                 int64
	OrderNumber        int64
	UserID             int64
	TicketUserID       int64
	TicketUserName     string
	TicketUserIDNumber string
	TicketCategoryID   int64
	TicketCategoryName string
	TicketPrice        int64
	SeatID             int64
	SeatRow            int64
	SeatCol            int64
	SeatPrice          int64
	OrderStatus        int64
	CreateOrderTime    string
}

type userOrderIndexFixture struct {
	ID              int64
	OrderNumber     int64
	UserID          int64
	ProgramID       int64
	OrderStatus     int64
	TicketCount     int64
	OrderPrice      int64
	CreateOrderTime string
}

type legacyOrderRouteFixture struct {
	OrderNumber  int64
	UserID       int64
	LogicSlot    int64
	RouteVersion string
	CreateTime   string
}

type fakeOrderProgramRPC struct {
	getProgramPreorderResp            *programrpc.ProgramPreorderInfo
	getProgramPreorderRespByProgramID map[int64]*programrpc.ProgramPreorderInfo
	getProgramPreorderErr             error
	lastGetProgramPreorderReq         *programrpc.GetProgramDetailReq

	autoAssignAndFreezeSeatsFunc    func(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error)
	autoAssignAndFreezeSeatsResp    *programrpc.AutoAssignAndFreezeSeatsResp
	autoAssignAndFreezeSeatsErr     error
	lastAutoAssignAndFreezeSeatsReq *programrpc.AutoAssignAndFreezeSeatsReq

	releaseSeatFreezeResp    *programrpc.ReleaseSeatFreezeResp
	releaseSeatFreezeErr     error
	lastReleaseSeatFreezeReq *programrpc.ReleaseSeatFreezeReq
	releaseSeatFreezeCalls   int

	confirmSeatFreezeResp    *programrpc.ConfirmSeatFreezeResp
	confirmSeatFreezeErr     error
	lastConfirmSeatFreezeReq *programrpc.ConfirmSeatFreezeReq
	confirmSeatFreezeCalls   int

	evaluateRefundRuleResp    *programrpc.EvaluateRefundRuleResp
	evaluateRefundRuleErr     error
	lastEvaluateRefundRuleReq *programrpc.EvaluateRefundRuleReq

	releaseSoldSeatsResp    *programrpc.ReleaseSoldSeatsResp
	releaseSoldSeatsErr     error
	lastReleaseSoldSeatsReq *programrpc.ReleaseSoldSeatsReq
	releaseSoldSeatsCalls   int
}

type fakeOrderUserRPC struct {
	getUserAndTicketUserListResp         *userrpc.GetUserAndTicketUserListResp
	getUserAndTicketUserListRespByUserID map[int64]*userrpc.GetUserAndTicketUserListResp
	getUserAndTicketUserListErr          error
	lastGetUserAndTicketUserListReq      *userrpc.GetUserAndTicketUserListReq
}

type fakeOrderPayRPC struct {
	mockPayResp    *payrpc.MockPayResp
	mockPayErr     error
	lastMockPayReq *payrpc.MockPayReq
	mockPayCalls   int

	getPayBillResp    *payrpc.GetPayBillResp
	getPayBillErr     error
	lastGetPayBillReq *payrpc.GetPayBillReq
	getPayBillCalls   int

	refundResp    *payrpc.RefundResp
	refundErr     error
	lastRefundReq *payrpc.RefundReq
	refundCalls   int
}

type fakeOrderRepeatGuard struct {
	lastKey     string
	lockErr     error
	lockCalls   int
	unlockCalls int
}

type fakeOrderCreateProducer struct {
	sendErr    error
	lastKey    string
	lastBody   []byte
	sendCalls  int
	closeCalls int
}

type fakeOrderCreateConsumer struct {
	factory    *fakeOrderCreateConsumerFactory
	startErr   error
	startErrs  []error
	startCalls int
	closeCalls int
	handler    func(context.Context, []byte) error
	started    chan struct{}
}

type fakeOrderCreateConsumerFactory struct {
	seedConsumers []*fakeOrderCreateConsumer
	consumers     []*fakeOrderCreateConsumer
	createCalls   int
	closeCalls    int
}

func newOrderTestServiceContext(t *testing.T) (*svc.ServiceContext, *fakeOrderProgramRPC, *fakeOrderUserRPC, *fakeOrderPayRPC) {
	t.Helper()

	mustInitOrderTestXid(t)

	cfg := config.Config{
		RpcServerConf: zrpc.RpcServerConf{
			Etcd: discov.EtcdConf{
				Hosts: []string{"127.0.0.1:2379"},
			},
		},
		MySQL: xmysql.Config{
			DataSource: testOrderMySQLDataSource,
		},
		StoreRedis: xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		},
		Order: config.OrderConfig{
			CloseAfter: 15 * time.Minute,
		},
		RepeatGuard: config.RepeatGuardConfig{
			Prefix:             "/damai-go/tests/repeat-guard/order-create/",
			SessionTTL:         10,
			LockAcquireTimeout: 200 * time.Millisecond,
		},
		Kafka: config.KafkaConfig{
			TopicPartitions: 5,
			ConsumerWorkers: 1,
			RetryBackoff:    10 * time.Millisecond,
		},
		Sharding: buildOrderTestShardingConfig(),
	}

	programRPC := &fakeOrderProgramRPC{
		releaseSeatFreezeResp: &programrpc.ReleaseSeatFreezeResp{Success: true},
		confirmSeatFreezeResp: &programrpc.ConfirmSeatFreezeResp{Success: true},
		releaseSoldSeatsResp:  &programrpc.ReleaseSoldSeatsResp{Success: true},
	}
	userRPC := &fakeOrderUserRPC{}
	payRPC := &fakeOrderPayRPC{}
	orderCreateProducer := &fakeOrderCreateProducer{}
	orderCreateConsumerFactory := &fakeOrderCreateConsumerFactory{}
	cfg.MySQL.DataSource = xmysql.WithLocalTime(cfg.MySQL.DataSource)
	conn := sqlx.NewMysql(cfg.MySQL.DataSource)
	shardConn0 := sqlx.NewMysql(cfg.Sharding.Shards["order-db-0"].DataSource)
	shardConn1 := sqlx.NewMysql(cfg.Sharding.Shards["order-db-1"].DataSource)
	redisClient := xredis.MustNew(cfg.StoreRedis)
	legacyOrderModel := model.NewDOrderModel(conn)
	legacyOrderTicketUserModel := model.NewDOrderTicketUserModel(conn)
	legacyUserOrderIndexModel := model.NewDUserOrderIndexModel(conn)
	legacyRouteDirectoryModel := model.NewDOrderRouteLegacyModel(conn)
	routeMap, err := sharding.NewRouteMap(cfg.Sharding.RouteMap.Version, buildOrderTestRouteEntries())
	if err != nil {
		t.Fatalf("NewRouteMap error: %v", err)
	}
	orderRouter := sharding.NewStaticRouter(routeMap)
	orderRepository := repository.NewOrderRepository(repository.Dependencies{
		Mode:                       cfg.Sharding.Mode,
		LegacyConn:                 conn,
		LegacyOrderModel:           legacyOrderModel,
		LegacyOrderTicketUserModel: legacyOrderTicketUserModel,
		LegacyUserOrderIndexModel:  legacyUserOrderIndexModel,
		LegacyRouteDirectoryModel:  legacyRouteDirectoryModel,
		ShardConns: map[string]sqlx.SqlConn{
			"order-db-0": shardConn0,
			"order-db-1": shardConn1,
		},
		RouteMap: routeMap,
		Router:   orderRouter,
	})
	purchaseLimitStore := limitcache.NewPurchaseLimitStore(redisClient, orderRepository, limitcache.Config{
		Prefix:          testPurchaseLimitLedgerPrefix,
		LedgerTTL:       time.Hour,
		LoadingCooldown: 200 * time.Millisecond,
	})
	svcCtx := &svc.ServiceContext{
		Config:        cfg,
		SqlConn:       conn,
		LegacySqlConn: conn,
		ShardSqlConns: map[string]sqlx.SqlConn{
			"order-db-0": shardConn0,
			"order-db-1": shardConn1,
		},
		Redis:                      redisClient,
		PurchaseLimitStore:         purchaseLimitStore,
		DOrderModel:                legacyOrderModel,
		DOrderTicketUserModel:      legacyOrderTicketUserModel,
		DUserOrderIndexModel:       legacyUserOrderIndexModel,
		DOrderRouteLegacyModel:     legacyRouteDirectoryModel,
		OrderRouteMap:              routeMap,
		OrderRouter:                orderRouter,
		OrderRepository:            orderRepository,
		ShardingMode:               cfg.Sharding.Mode,
		ProgramRpc:                 programRPC,
		UserRpc:                    userRPC,
		PayRpc:                     payRPC,
		OrderCreateProducer:        orderCreateProducer,
		OrderCreateConsumerFactory: orderCreateConsumerFactory,
	}

	return svcCtx, programRPC, userRPC, payRPC
}

func buildOrderTestShardingConfig() config.ShardingConfig {
	return config.ShardingConfig{
		Mode: sharding.MigrationModeLegacyOnly,
		LegacyMySQL: xmysql.Config{
			DataSource: testOrderMySQLDataSource,
		},
		Shards: map[string]xmysql.Config{
			"order-db-0": {DataSource: testOrderMySQLDataSource},
			"order-db-1": {DataSource: testOrderMySQLDataSource},
		},
		RouteMap: config.RouteMapConfig{
			Version: "v1",
			Entries: buildOrderTestRouteEntryConfigs(),
		},
	}
}

func buildOrderTestRouteEntryConfigs() []config.RouteEntryConfig {
	entries := buildOrderTestRouteEntries()
	configs := make([]config.RouteEntryConfig, 0, len(entries))
	for _, entry := range entries {
		configs = append(configs, config.RouteEntryConfig{
			Version:     entry.Version,
			LogicSlot:   entry.LogicSlot,
			DBKey:       entry.DBKey,
			TableSuffix: entry.TableSuffix,
			Status:      entry.Status,
			WriteMode:   entry.WriteMode,
		})
	}

	return configs
}

func buildOrderTestRouteEntries() []sharding.RouteEntry {
	entries := make([]sharding.RouteEntry, 0, 1024)
	for slot := 0; slot < 1024; slot++ {
		entry := sharding.RouteEntry{
			Version:     "v1",
			LogicSlot:   slot,
			DBKey:       "order-db-0",
			TableSuffix: "00",
			Status:      sharding.RouteStatusStable,
			WriteMode:   sharding.WriteModeLegacyPrimary,
		}
		if slot%2 == 1 {
			entry.DBKey = "order-db-1"
			entry.TableSuffix = "01"
		}
		entries = append(entries, entry)
	}

	return entries
}

func mustInitOrderTestXid(t *testing.T) {
	t.Helper()

	_ = xid.Close()
	if err := xid.InitEtcd(context.Background(), xid.Config{
		Hosts:   []string{"127.0.0.1:2379"},
		Prefix:  "/damai-go/tests/snowflake/order-rpc/",
		Service: "order-rpc-test",
	}); err != nil {
		t.Fatalf("init xid error: %v", err)
	}
	t.Cleanup(func() {
		_ = xid.Close()
	})
}

func clearPurchaseLimitLedger(t *testing.T, svcCtx *svc.ServiceContext, userID, programID int64) {
	t.Helper()

	if svcCtx.PurchaseLimitStore == nil {
		t.Fatalf("expected purchase limit store to be configured")
	}
	if err := svcCtx.PurchaseLimitStore.Clear(context.Background(), userID, programID); err != nil {
		t.Fatalf("clear purchase limit ledger error: %v", err)
	}
}

func seedPurchaseLimitLedger(t *testing.T, svcCtx *svc.ServiceContext, userID, programID, activeCount int64, reservations map[int64]int64) {
	t.Helper()

	if svcCtx.PurchaseLimitStore == nil {
		t.Fatalf("expected purchase limit store to be configured")
	}
	if err := svcCtx.PurchaseLimitStore.Seed(context.Background(), userID, programID, activeCount, reservations); err != nil {
		t.Fatalf("seed purchase limit ledger error: %v", err)
	}
}

func requirePurchaseLimitSnapshot(t *testing.T, svcCtx *svc.ServiceContext, userID, programID int64) *limitcache.PurchaseLimitSnapshot {
	t.Helper()

	if svcCtx.PurchaseLimitStore == nil {
		t.Fatalf("expected purchase limit store to be configured")
	}

	snapshot, err := svcCtx.PurchaseLimitStore.Snapshot(context.Background(), userID, programID)
	if err != nil {
		t.Fatalf("snapshot purchase limit ledger error: %v", err)
	}

	return snapshot
}

func waitPurchaseLimitLedgerReady(t *testing.T, svcCtx *svc.ServiceContext, userID, programID, expectedActiveCount int64) *limitcache.PurchaseLimitSnapshot {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := requirePurchaseLimitSnapshot(t, svcCtx, userID, programID)
		if snapshot.Ready && snapshot.ActiveCount == expectedActiveCount {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("purchase limit ledger was not ready before deadline, userID=%d programID=%d", userID, programID)
	return nil
}

func requirePurchaseLimitLedgerAbsentFor(t *testing.T, svcCtx *svc.ServiceContext, userID, programID int64, duration time.Duration) {
	t.Helper()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		snapshot := requirePurchaseLimitSnapshot(t, svcCtx, userID, programID)
		if snapshot.Ready || snapshot.Loading {
			t.Fatalf(
				"expected purchase limit ledger to stay absent, userID=%d programID=%d snapshot=%+v",
				userID,
				programID,
				snapshot,
			)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func primePurchaseLimitLedgerFromDB(t *testing.T, svcCtx *svc.ServiceContext, userID, programID int64) {
	t.Helper()

	activeCount, err := svcCtx.DOrderModel.CountActiveTicketsByUserProgram(context.Background(), userID, programID)
	if err != nil {
		t.Fatalf("count active tickets for purchase limit ledger error: %v", err)
	}
	reservations, err := svcCtx.DOrderModel.ListUnpaidReservationsByUserProgram(context.Background(), userID, programID)
	if err != nil {
		t.Fatalf("list unpaid reservations for purchase limit ledger error: %v", err)
	}

	seedPurchaseLimitLedger(t, svcCtx, userID, programID, activeCount, reservations)
}

func (f *fakeOrderCreateProducer) Send(_ context.Context, key string, value []byte) error {
	f.lastKey = key
	f.lastBody = append(f.lastBody[:0], value...)
	f.sendCalls++
	return f.sendErr
}

func (f *fakeOrderCreateProducer) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeOrderCreateConsumer) Start(ctx context.Context, handler func(context.Context, []byte) error) error {
	f.startCalls++
	f.handler = handler
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if len(f.startErrs) > 0 {
		err := f.startErrs[0]
		f.startErrs = f.startErrs[1:]
		if err != nil {
			return err
		}
	}
	if f.startErr != nil {
		return f.startErr
	}

	<-ctx.Done()
	return nil
}

func (f *fakeOrderCreateConsumer) Close() error {
	f.closeCalls++
	if f.factory != nil {
		f.factory.closeCalls++
	}
	return nil
}

func (f *fakeOrderCreateConsumerFactory) New(_ config.KafkaConfig) mq.OrderCreateConsumer {
	f.createCalls++

	var consumer *fakeOrderCreateConsumer
	if len(f.seedConsumers) > 0 {
		consumer = f.seedConsumers[0]
		f.seedConsumers = f.seedConsumers[1:]
	} else {
		consumer = &fakeOrderCreateConsumer{
			started: make(chan struct{}, 1),
		}
	}
	consumer.factory = f
	if consumer.started == nil {
		consumer.started = make(chan struct{}, 1)
	}
	f.consumers = append(f.consumers, consumer)

	return consumer
}

func (f *fakeOrderRepeatGuard) Lock(_ context.Context, key string) (repeatguard.UnlockFunc, error) {
	f.lastKey = key
	f.lockCalls++
	if f.lockErr != nil {
		return nil, f.lockErr
	}

	return func() {
		f.unlockCalls++
	}, nil
}

func resetOrderDomainState(t *testing.T) {
	t.Helper()

	db := openOrderTestDB(t, testOrderMySQLDataSource)
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
		execOrderSQLFile(t, db, relativePath)
	}
}

func setOrderTestRepositoryMode(t *testing.T, svcCtx *svc.ServiceContext, mode string) {
	t.Helper()

	svcCtx.Config.Sharding.Mode = mode
	svcCtx.ShardingMode = mode
	svcCtx.OrderRepository = repository.NewOrderRepository(repository.Dependencies{
		Mode:                       mode,
		LegacyConn:                 svcCtx.LegacySqlConn,
		LegacyOrderModel:           svcCtx.DOrderModel,
		LegacyOrderTicketUserModel: svcCtx.DOrderTicketUserModel,
		LegacyUserOrderIndexModel:  svcCtx.DUserOrderIndexModel,
		LegacyRouteDirectoryModel:  svcCtx.DOrderRouteLegacyModel,
		ShardConns:                 svcCtx.ShardSqlConns,
		RouteMap:                   svcCtx.OrderRouteMap,
		Router:                     svcCtx.OrderRouter,
	})
	svcCtx.PurchaseLimitStore = limitcache.NewPurchaseLimitStore(svcCtx.Redis, svcCtx.OrderRepository, limitcache.Config{
		Prefix:          testPurchaseLimitLedgerPrefix,
		LedgerTTL:       time.Hour,
		LoadingCooldown: 200 * time.Millisecond,
	})
}

func seedOrderFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...orderFixture) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withOrderFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO d_order (
				id, order_number, program_id, program_title, program_item_picture, program_place, program_show_time,
				program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
				order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.ProgramID,
			fixture.ProgramTitle,
			fixture.ProgramItemPicture,
			fixture.ProgramPlace,
			fixture.ProgramShowTime,
			fixture.ProgramPermitChooseSeat,
			fixture.UserID,
			fixture.DistributionMode,
			fixture.TakeTicketMode,
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.OrderStatus,
			fixture.FreezeToken,
			fixture.OrderExpireTime,
			fixture.CreateOrderTime,
			nullTimeIfEmpty(fixture.CancelOrderTime),
			nullTimeIfEmpty(fixture.PayOrderTime),
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		)
	}
}

func seedOrderTicketUserFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...orderTicketUserFixture) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withOrderTicketUserFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO d_order_ticket_user (
				id, order_number, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
				ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
				seat_price, order_status, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.TicketUserID,
			fixture.TicketUserName,
			fixture.TicketUserIDNumber,
			fixture.TicketCategoryID,
			fixture.TicketCategoryName,
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
		)
	}
}

func seedShardOrderFixtures(t *testing.T, svcCtx *svc.ServiceContext, route sharding.Route, fixtures ...orderFixture) {
	t.Helper()
	seedOrderFixturesIntoTable(t, svcCtx.Config.MySQL.DataSource, "d_order_"+route.TableSuffix, fixtures...)
}

func seedShardOrderTicketUserFixtures(t *testing.T, svcCtx *svc.ServiceContext, route sharding.Route, fixtures ...orderTicketUserFixture) {
	t.Helper()
	seedOrderTicketUserFixturesIntoTable(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user_"+route.TableSuffix, fixtures...)
}

func seedUserOrderIndexFixtures(t *testing.T, svcCtx *svc.ServiceContext, route sharding.Route, fixtures ...userOrderIndexFixture) {
	t.Helper()

	table := "d_user_order_index"
	if route.TableSuffix != "" {
		table = "d_user_order_index_" + route.TableSuffix
	}
	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withUserOrderIndexFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
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
		)
	}
}

func seedLegacyOrderRouteFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...legacyOrderRouteFixture) {
	t.Helper()

	db := openOrderTestDB(t, svcCtx.Config.MySQL.DataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withLegacyOrderRouteFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO d_order_route_legacy (
				order_number, user_id, logic_slot, route_version, status, create_time, edit_time
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.LogicSlot,
			fixture.RouteVersion,
			1,
			fixture.CreateTime,
			fixture.CreateTime,
		)
	}
}

func countRows(t *testing.T, dataSource, table string) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var count int64
	if err := db.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("QueryRow count error: %v", err)
	}

	return count
}

func findOrderStatus(t *testing.T, dataSource string, orderNumber int64) int64 {
	t.Helper()

	return findOrderStatusFromTable(t, dataSource, "d_order", orderNumber)
}

func findOrderStatusFromTable(t *testing.T, dataSource, table string, orderNumber int64) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	if err := db.QueryRow("SELECT order_status FROM "+table+" WHERE order_number = ?", orderNumber).Scan(&status); err != nil {
		t.Fatalf("QueryRow order status error: %v", err)
	}

	return status
}

func findOrderTicketStatus(t *testing.T, dataSource string, orderNumber int64) int64 {
	t.Helper()

	return findOrderTicketStatusFromTable(t, dataSource, "d_order_ticket_user", orderNumber)
}

func findOrderTicketStatusFromTable(t *testing.T, dataSource, table string, orderNumber int64) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	if err := db.QueryRow("SELECT order_status FROM "+table+" WHERE order_number = ? ORDER BY id ASC LIMIT 1", orderNumber).Scan(&status); err != nil {
		t.Fatalf("QueryRow order ticket status error: %v", err)
	}

	return status
}

func findUserOrderIndexStatusFromTable(t *testing.T, dataSource, table string, orderNumber int64) int64 {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	if err := db.QueryRow("SELECT order_status FROM "+table+" WHERE order_number = ?", orderNumber).Scan(&status); err != nil {
		t.Fatalf("QueryRow user order index status error: %v", err)
	}

	return status
}

func (f *fakeOrderProgramRPC) ConfirmSeatFreeze(ctx context.Context, in *programrpc.ConfirmSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ConfirmSeatFreezeResp, error) {
	f.lastConfirmSeatFreezeReq = in
	f.confirmSeatFreezeCalls++
	return f.confirmSeatFreezeResp, f.confirmSeatFreezeErr
}

func (f *fakeOrderProgramRPC) EvaluateRefundRule(ctx context.Context, in *programrpc.EvaluateRefundRuleReq, opts ...grpc.CallOption) (*programrpc.EvaluateRefundRuleResp, error) {
	f.lastEvaluateRefundRuleReq = in
	return f.evaluateRefundRuleResp, f.evaluateRefundRuleErr
}

func (f *fakeOrderProgramRPC) ReleaseSoldSeats(ctx context.Context, in *programrpc.ReleaseSoldSeatsReq, opts ...grpc.CallOption) (*programrpc.ReleaseSoldSeatsResp, error) {
	f.lastReleaseSoldSeatsReq = in
	f.releaseSoldSeatsCalls++
	return f.releaseSoldSeatsResp, f.releaseSoldSeatsErr
}

func (f *fakeOrderPayRPC) MockPay(ctx context.Context, in *payrpc.MockPayReq, opts ...grpc.CallOption) (*payrpc.MockPayResp, error) {
	f.lastMockPayReq = in
	f.mockPayCalls++
	return f.mockPayResp, f.mockPayErr
}

func (f *fakeOrderPayRPC) GetPayBill(ctx context.Context, in *payrpc.GetPayBillReq, opts ...grpc.CallOption) (*payrpc.GetPayBillResp, error) {
	f.lastGetPayBillReq = in
	f.getPayBillCalls++
	return f.getPayBillResp, f.getPayBillErr
}

func (f *fakeOrderPayRPC) Refund(ctx context.Context, in *payrpc.RefundReq, opts ...grpc.CallOption) (*payrpc.RefundResp, error) {
	f.lastRefundReq = in
	f.refundCalls++
	return f.refundResp, f.refundErr
}

func openOrderTestDB(t *testing.T, dataSource string) *sql.DB {
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

func execOrderSQLFile(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(orderProjectRoot(t), relativePath))
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

func mustExecOrderSQL(t *testing.T, db *sql.DB, stmt string, args ...interface{}) {
	t.Helper()

	if _, err := db.Exec(stmt, args...); err != nil {
		t.Fatalf("db.Exec error: %v\nstatement: %s", err, stmt)
	}
}

func withOrderFixtureDefaults(fixture orderFixture) orderFixture {
	if fixture.ProgramTitle == "" {
		fixture.ProgramTitle = "订单测试演出"
	}
	if fixture.ProgramItemPicture == "" {
		fixture.ProgramItemPicture = "https://example.com/order-program.jpg"
	}
	if fixture.ProgramPlace == "" {
		fixture.ProgramPlace = "测试场馆"
	}
	if fixture.ProgramShowTime == "" {
		fixture.ProgramShowTime = "2026-12-31 19:30:00"
	}
	if fixture.ProgramPermitChooseSeat == 0 {
		fixture.ProgramPermitChooseSeat = 0
	}
	if fixture.DistributionMode == "" {
		fixture.DistributionMode = "express"
	}
	if fixture.TakeTicketMode == "" {
		fixture.TakeTicketMode = "paper"
	}
	if fixture.TicketCount == 0 {
		fixture.TicketCount = 1
	}
	if fixture.OrderPrice == 0 {
		fixture.OrderPrice = 299
	}
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = testOrderStatusUnpaid
	}
	if fixture.FreezeToken == "" {
		fixture.FreezeToken = "freeze-seed"
	}
	if fixture.OrderExpireTime == "" {
		fixture.OrderExpireTime = "2026-12-31 20:00:00"
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}

	return fixture
}

func withOrderTicketUserFixtureDefaults(fixture orderTicketUserFixture) orderTicketUserFixture {
	if fixture.TicketUserName == "" {
		fixture.TicketUserName = "张三"
	}
	if fixture.TicketUserIDNumber == "" {
		fixture.TicketUserIDNumber = "110101199001011234"
	}
	if fixture.TicketCategoryName == "" {
		fixture.TicketCategoryName = "普通票"
	}
	if fixture.TicketPrice == 0 {
		fixture.TicketPrice = 299
	}
	if fixture.SeatPrice == 0 {
		fixture.SeatPrice = 299
	}
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = testOrderStatusUnpaid
	}
	if fixture.CreateOrderTime == "" {
		fixture.CreateOrderTime = "2026-01-01 00:00:00"
	}

	return fixture
}

func withUserOrderIndexFixtureDefaults(fixture userOrderIndexFixture) userOrderIndexFixture {
	if fixture.OrderStatus == 0 {
		fixture.OrderStatus = testOrderStatusUnpaid
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

func withLegacyOrderRouteFixtureDefaults(fixture legacyOrderRouteFixture) legacyOrderRouteFixture {
	if fixture.RouteVersion == "" {
		fixture.RouteVersion = "v1"
	}
	if fixture.CreateTime == "" {
		fixture.CreateTime = "2026-01-01 00:00:00"
	}

	return fixture
}

func orderRouteForUser(t *testing.T, svcCtx *svc.ServiceContext, userID int64) sharding.Route {
	t.Helper()

	route, err := svcCtx.OrderRepository.RouteByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("RouteByUserID error: %v", err)
	}

	return route
}

func mustFindOrderTestUserIDByLogicSlot(t *testing.T, targetSlot int) int64 {
	t.Helper()

	for userID := int64(1); userID < 1_000_000; userID++ {
		if sharding.LogicSlotByUserID(userID) == targetSlot {
			return userID
		}
	}

	t.Fatalf("failed to find user id for logic slot %d", targetSlot)
	return 0
}

func seedOrderFixturesIntoTable(t *testing.T, dataSource, table string, fixtures ...orderFixture) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withOrderFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO `+table+` (
				id, order_number, program_id, program_title, program_item_picture, program_place, program_show_time,
				program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
				order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.ProgramID,
			fixture.ProgramTitle,
			fixture.ProgramItemPicture,
			fixture.ProgramPlace,
			fixture.ProgramShowTime,
			fixture.ProgramPermitChooseSeat,
			fixture.UserID,
			fixture.DistributionMode,
			fixture.TakeTicketMode,
			fixture.TicketCount,
			fixture.OrderPrice,
			fixture.OrderStatus,
			fixture.FreezeToken,
			fixture.OrderExpireTime,
			fixture.CreateOrderTime,
			nullTimeIfEmpty(fixture.CancelOrderTime),
			nullTimeIfEmpty(fixture.PayOrderTime),
			fixture.CreateOrderTime,
			fixture.CreateOrderTime,
			1,
		)
	}
}

func seedOrderTicketUserFixturesIntoTable(t *testing.T, dataSource, table string, fixtures ...orderTicketUserFixture) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	for _, fixture := range fixtures {
		fixture = withOrderTicketUserFixtureDefaults(fixture)
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO `+table+` (
				id, order_number, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
				ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
				seat_price, order_status, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.UserID,
			fixture.TicketUserID,
			fixture.TicketUserName,
			fixture.TicketUserIDNumber,
			fixture.TicketCategoryID,
			fixture.TicketCategoryName,
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
		)
	}
}

func nullTimeIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}

	return value
}

func orderProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func buildTestProgramPreorder(programID, ticketCategoryID, perOrderLimit, perAccountLimit, ticketPrice int64) *programrpc.ProgramPreorderInfo {
	return &programrpc.ProgramPreorderInfo{
		Id:                           programID,
		ProgramGroupId:               programID + 1000,
		Title:                        "订单测试演出",
		Place:                        "测试场馆",
		ItemPicture:                  "https://example.com/order-program.jpg",
		ShowTime:                     "2026-12-31 19:30:00",
		ShowDayTime:                  "2026-12-31 00:00:00",
		ShowWeekTime:                 "周四",
		PerOrderLimitPurchaseCount:   perOrderLimit,
		PerAccountLimitPurchaseCount: perAccountLimit,
		PermitChooseSeat:             0,
		TicketCategoryVoList: []*programrpc.ProgramPreorderTicketCategoryInfo{
			{
				Id:           ticketCategoryID,
				Introduce:    "普通票",
				Price:        ticketPrice,
				TotalNumber:  100,
				RemainNumber: 100,
			},
		},
	}
}

func buildTestUserAndTicketUsers(userID int64, ticketUsers ...*userrpc.TicketUserInfo) *userrpc.GetUserAndTicketUserListResp {
	return &userrpc.GetUserAndTicketUserListResp{
		UserVo:           &userrpc.UserInfo{Id: userID, Mobile: "13800000000"},
		TicketUserVoList: ticketUsers,
	}
}

func (f *fakeOrderProgramRPC) CreateProgram(ctx context.Context, in *programrpc.CreateProgramReq, opts ...grpc.CallOption) (*programrpc.CreateProgramResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) UpdateProgram(ctx context.Context, in *programrpc.UpdateProgramReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListProgramCategories(ctx context.Context, in *programrpc.Empty, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListHomePrograms(ctx context.Context, in *programrpc.ListHomeProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramHomeListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) PagePrograms(ctx context.Context, in *programrpc.PageProgramsReq, opts ...grpc.CallOption) (*programrpc.ProgramPageResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) GetProgramDetail(ctx context.Context, in *programrpc.GetProgramDetailReq, opts ...grpc.CallOption) (*programrpc.ProgramDetailInfo, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) GetProgramPreorder(ctx context.Context, in *programrpc.GetProgramDetailReq, opts ...grpc.CallOption) (*programrpc.ProgramPreorderInfo, error) {
	f.lastGetProgramPreorderReq = in
	if resp, ok := f.getProgramPreorderRespByProgramID[in.GetId()]; ok {
		return resp, f.getProgramPreorderErr
	}
	return f.getProgramPreorderResp, f.getProgramPreorderErr
}

func (f *fakeOrderProgramRPC) ListTicketCategoriesByProgram(ctx context.Context, in *programrpc.ListTicketCategoriesByProgramReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) AutoAssignAndFreezeSeats(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq, opts ...grpc.CallOption) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
	f.lastAutoAssignAndFreezeSeatsReq = in
	if f.autoAssignAndFreezeSeatsFunc != nil {
		return f.autoAssignAndFreezeSeatsFunc(ctx, in)
	}
	return f.autoAssignAndFreezeSeatsResp, f.autoAssignAndFreezeSeatsErr
}

func (f *fakeOrderProgramRPC) ReleaseSeatFreeze(ctx context.Context, in *programrpc.ReleaseSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ReleaseSeatFreezeResp, error) {
	f.lastReleaseSeatFreezeReq = in
	f.releaseSeatFreezeCalls++
	return f.releaseSeatFreezeResp, f.releaseSeatFreezeErr
}

func (f *fakeOrderUserRPC) Register(ctx context.Context, in *userrpc.RegisterReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Exist(ctx context.Context, in *userrpc.ExistReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Login(ctx context.Context, in *userrpc.LoginReq, opts ...grpc.CallOption) (*userrpc.LoginResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) GetUserById(ctx context.Context, in *userrpc.GetUserByIdReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) GetUserByMobile(ctx context.Context, in *userrpc.GetUserByMobileReq, opts ...grpc.CallOption) (*userrpc.UserInfo, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Logout(ctx context.Context, in *userrpc.LogoutReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdateUser(ctx context.Context, in *userrpc.UpdateUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdatePassword(ctx context.Context, in *userrpc.UpdatePasswordReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdateEmail(ctx context.Context, in *userrpc.UpdateEmailReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) UpdateMobile(ctx context.Context, in *userrpc.UpdateMobileReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) Authentication(ctx context.Context, in *userrpc.AuthenticationReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) ListTicketUsers(ctx context.Context, in *userrpc.ListTicketUsersReq, opts ...grpc.CallOption) (*userrpc.ListTicketUsersResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) AddTicketUser(ctx context.Context, in *userrpc.AddTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) DeleteTicketUser(ctx context.Context, in *userrpc.DeleteTicketUserReq, opts ...grpc.CallOption) (*userrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderUserRPC) GetUserAndTicketUserList(ctx context.Context, in *userrpc.GetUserAndTicketUserListReq, opts ...grpc.CallOption) (*userrpc.GetUserAndTicketUserListResp, error) {
	f.lastGetUserAndTicketUserListReq = in
	if resp, ok := f.getUserAndTicketUserListRespByUserID[in.GetUserId()]; ok {
		return resp, f.getUserAndTicketUserListErr
	}
	return f.getUserAndTicketUserListResp, f.getUserAndTicketUserListErr
}

var _ programrpc.ProgramRpc = (*fakeOrderProgramRPC)(nil)
var _ userrpc.UserRpc = (*fakeOrderUserRPC)(nil)
var _ = pb.BoolResp{}
