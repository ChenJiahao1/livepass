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
	"sync/atomic"
	"testing"
	"time"

	"damai-go/pkg/xid"
	"damai-go/pkg/xmysql"
	"damai-go/pkg/xredis"
	"damai-go/services/order-rpc/internal/config"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/mq"
	"damai-go/services/order-rpc/internal/repeatguard"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/repository"
	"damai-go/services/order-rpc/sharding"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"
	userrpc "damai-go/services/user-rpc/userrpc"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

var testOrderMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true"

var (
	orderDomainStateMu      sync.Mutex
	orderDomainStateOwners  = make(map[string]struct{})
	orderDomainStateOwnersM sync.Mutex
)

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
	ShowTimeID              int64
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
	ShowTimeID         int64
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

type fakeOrderProgramRPC struct {
	getProgramPreorderResp            *programrpc.ProgramPreorderInfo
	getProgramPreorderRespByProgramID map[int64]*programrpc.ProgramPreorderInfo
	getProgramPreorderErr             error
	lastGetProgramPreorderReq         *programrpc.GetProgramPreorderReq

	listProgramShowTimesForRushResp    *programrpc.ListProgramShowTimesForRushResp
	listProgramShowTimesForRushErr     error
	lastListProgramShowTimesForRushReq *programrpc.ListProgramShowTimesForRushReq

	autoAssignAndFreezeSeatsFunc    func(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq) (*programrpc.AutoAssignAndFreezeSeatsResp, error)
	autoAssignAndFreezeSeatsResp    *programrpc.AutoAssignAndFreezeSeatsResp
	autoAssignAndFreezeSeatsErr     error
	lastAutoAssignAndFreezeSeatsReq *programrpc.AutoAssignAndFreezeSeatsReq
	autoAssignAndFreezeSeatsCalls   int

	releaseSeatFreezeResp    *programrpc.ReleaseSeatFreezeResp
	releaseSeatFreezeErr     error
	lastReleaseSeatFreezeReq *programrpc.ReleaseSeatFreezeReq
	releaseSeatFreezeCalls   int
	onReleaseSeatFreeze      func(*programrpc.ReleaseSeatFreezeReq)

	confirmSeatFreezeResp    *programrpc.ConfirmSeatFreezeResp
	confirmSeatFreezeErr     error
	lastConfirmSeatFreezeReq *programrpc.ConfirmSeatFreezeReq
	confirmSeatFreezeCalls   int
	onConfirmSeatFreeze      func(*programrpc.ConfirmSeatFreezeReq)

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
	onMockPay      func(*payrpc.MockPayReq)

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
	sendHook   func()
	sendFunc   func(context.Context, string, []byte) error
	lastKey    string
	lastBody   []byte
	sendCalls  int32
	closeCalls int32
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

type tracingOrderRepository struct {
	base                       repository.OrderRepository
	transactByOrderNumberCalls int
	events                     []string
	failUpdateCancelStatusErr  error
	failUpdateCancelStatusN    int
	failUpdatePayStatusErr     error
	failUpdatePayStatusN       int
}

type tracingOrderTx struct {
	repository.OrderTx
	repo    *tracingOrderRepository
	txLabel string
}

func newOrderTestServiceContext(t *testing.T) (*svc.ServiceContext, *fakeOrderProgramRPC, *fakeOrderUserRPC, *fakeOrderPayRPC) {
	t.Helper()
	acquireOrderDomainState(t)

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
		RushOrder: config.RushOrderConfig{
			Enabled:       true,
			TokenSecret:   "order-rpc-test-secret",
			TokenTTL:      2 * time.Minute,
			InFlightTTL:   30 * time.Second,
			FinalStateTTL: 30 * time.Minute,
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
	shardConn0 := sqlx.NewMysql(xmysql.WithLocalTime(cfg.Sharding.Shards["order-db-0"].DataSource))
	shardConn1 := sqlx.NewMysql(xmysql.WithLocalTime(cfg.Sharding.Shards["order-db-1"].DataSource))
	redisClient := xredis.MustNew(cfg.StoreRedis)
	routeMap, err := sharding.NewRouteMap(cfg.Sharding.RouteMap.Version, buildOrderTestRouteEntries())
	if err != nil {
		t.Fatalf("NewRouteMap error: %v", err)
	}
	orderRouter := sharding.NewStaticRouter(routeMap)
	orderRepository := repository.NewOrderRepository(repository.Dependencies{
		ShardConns: map[string]sqlx.SqlConn{
			"order-db-0": shardConn0,
			"order-db-1": shardConn1,
		},
		RouteMap: routeMap,
		Router:   orderRouter,
	})
	attemptStore := rush.NewAttemptStore(redisClient, rush.AttemptStoreConfig{
		InFlightTTL:   cfg.RushOrder.InFlightTTL,
		FinalStateTTL: cfg.RushOrder.FinalStateTTL,
	})
	purchaseTokenCodec := rush.MustNewPurchaseTokenCodec(cfg.RushOrder.TokenSecret, cfg.RushOrder.TokenTTL)
	svcCtx := &svc.ServiceContext{
		Config:  cfg,
		SqlConn: conn,
		ShardSqlConns: map[string]sqlx.SqlConn{
			"order-db-0": shardConn0,
			"order-db-1": shardConn1,
		},
		Redis:                      redisClient,
		AttemptStore:               attemptStore,
		OrderRouteMap:              routeMap,
		OrderRouter:                orderRouter,
		OrderRepository:            orderRepository,
		PurchaseTokenCodec:         purchaseTokenCodec,
		ProgramRpc:                 programRPC,
		UserRpc:                    userRPC,
		PayRpc:                     payRPC,
		OrderCreateProducer:        orderCreateProducer,
		OrderCreateConsumerFactory: orderCreateConsumerFactory,
	}

	return svcCtx, programRPC, userRPC, payRPC
}

func rebindOrderTestAttemptStore(t *testing.T, svcCtx *svc.ServiceContext) *rush.AttemptStore {
	t.Helper()
	acquireOrderDomainState(t)

	if svcCtx == nil || svcCtx.Redis == nil {
		t.Fatalf("expected redis-backed service context")
	}

	prefix := fmt.Sprintf(
		"damai-go:test:order:rush:%s:%d",
		strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-"),
		time.Now().UnixNano(),
	)
	store := rush.NewAttemptStore(svcCtx.Redis, rush.AttemptStoreConfig{
		Prefix:        prefix,
		InFlightTTL:   svcCtx.Config.RushOrder.InFlightTTL,
		FinalStateTTL: svcCtx.Config.RushOrder.FinalStateTTL,
	})
	svcCtx.AttemptStore = store

	return store
}

func buildOrderTestShardingConfig() config.ShardingConfig {
	return config.ShardingConfig{
		Mode: "shard_only",
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
			WriteMode:   sharding.WriteModeShardPrimary,
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
	if err := xid.Init(xid.Config{
		Provider: xid.ProviderStatic,
		NodeID:   902,
	}); err != nil {
		t.Fatalf("init xid error: %v", err)
	}
	t.Cleanup(func() {
		_ = xid.Close()
	})
}

func (f *fakeOrderCreateProducer) Send(ctx context.Context, key string, value []byte) error {
	f.lastKey = key
	f.lastBody = append(f.lastBody[:0], value...)
	atomic.AddInt32(&f.sendCalls, 1)
	if f.sendHook != nil {
		f.sendHook()
	}
	if f.sendFunc != nil {
		return f.sendFunc(ctx, key, append([]byte(nil), value...))
	}
	return f.sendErr
}

func (f *fakeOrderCreateProducer) Close() error {
	atomic.AddInt32(&f.closeCalls, 1)
	return nil
}

func (f *fakeOrderCreateProducer) SendCalls() int {
	return int(atomic.LoadInt32(&f.sendCalls))
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
	acquireOrderDomainState(t)

	db := openOrderTestDB(t, testOrderMySQLDataSource)
	defer db.Close()

	for _, relativePath := range []string{
		"sql/order/sharding/d_order_shards.sql",
		"sql/order/sharding/d_order_ticket_user_shards.sql",
		"sql/order/sharding/d_order_user_guard.sql",
		"sql/order/sharding/d_order_viewer_guard.sql",
		"sql/order/sharding/d_order_seat_guard.sql",
		"sql/order/sharding/d_delay_task_outbox.sql",
	} {
		execOrderSQLFile(t, db, relativePath)
	}
}

func acquireOrderDomainState(t *testing.T) {
	t.Helper()

	rootName := t.Name()
	if idx := strings.IndexByte(rootName, '/'); idx >= 0 {
		rootName = rootName[:idx]
	}

	orderDomainStateOwnersM.Lock()
	if _, ok := orderDomainStateOwners[rootName]; ok {
		orderDomainStateOwnersM.Unlock()
		return
	}
	orderDomainStateOwners[rootName] = struct{}{}
	orderDomainStateOwnersM.Unlock()

	orderDomainStateMu.Lock()
	t.Cleanup(func() {
		orderDomainStateOwnersM.Lock()
		delete(orderDomainStateOwners, rootName)
		orderDomainStateOwnersM.Unlock()
		orderDomainStateMu.Unlock()
	})
}

func seedOrderFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...orderFixture) {
	t.Helper()

	for _, fixture := range fixtures {
		fixture = withOrderFixtureDefaults(fixture)
		route := orderRouteForUser(t, svcCtx, fixture.UserID)
		seedOrderFixturesIntoTable(t, svcCtx.Config.MySQL.DataSource, "d_order_"+route.TableSuffix, fixture)
	}
}

func seedOrderTicketUserFixtures(t *testing.T, svcCtx *svc.ServiceContext, fixtures ...orderTicketUserFixture) {
	t.Helper()

	for _, fixture := range fixtures {
		fixture = withOrderTicketUserFixtureDefaults(fixture)
		route := orderRouteForUser(t, svcCtx, fixture.UserID)
		seedOrderTicketUserFixturesIntoTable(t, svcCtx.Config.MySQL.DataSource, "d_order_ticket_user_"+route.TableSuffix, fixture)
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

func countShardOrderRows(t *testing.T, dataSource string) int64 {
	t.Helper()

	var count int64
	for _, table := range []string{"d_order_00", "d_order_01"} {
		count += countRows(t, dataSource, table)
	}

	return count
}

func countShardOrderTicketRows(t *testing.T, dataSource string) int64 {
	t.Helper()

	var count int64
	for _, table := range []string{"d_order_ticket_user_00", "d_order_ticket_user_01"} {
		count += countRows(t, dataSource, table)
	}

	return count
}

func findOrderStatus(t *testing.T, dataSource string, orderNumber int64) int64 {
	t.Helper()

	for _, table := range []string{"d_order_00", "d_order_01"} {
		status, found := findOrderStatusFromTableIfExists(t, dataSource, table, orderNumber)
		if found {
			return status
		}
	}

	t.Fatalf("order status not found for orderNumber=%d", orderNumber)
	return 0
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

func findOrderStatusFromTableIfExists(t *testing.T, dataSource, table string, orderNumber int64) (int64, bool) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	err := db.QueryRow("SELECT order_status FROM "+table+" WHERE order_number = ?", orderNumber).Scan(&status)
	if err == nil {
		return status, true
	}
	if err == sql.ErrNoRows {
		return 0, false
	}

	t.Fatalf("QueryRow order status error: %v", err)
	return 0, false
}

func findOrderTicketStatus(t *testing.T, dataSource string, orderNumber int64) int64 {
	t.Helper()

	for _, table := range []string{"d_order_ticket_user_00", "d_order_ticket_user_01"} {
		status, found := findOrderTicketStatusFromTableIfExists(t, dataSource, table, orderNumber)
		if found {
			return status
		}
	}

	t.Fatalf("order ticket status not found for orderNumber=%d", orderNumber)
	return 0
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

func findOrderTicketStatusFromTableIfExists(t *testing.T, dataSource, table string, orderNumber int64) (int64, bool) {
	t.Helper()

	db := openOrderTestDB(t, dataSource)
	defer db.Close()

	var status int64
	err := db.QueryRow("SELECT order_status FROM "+table+" WHERE order_number = ? ORDER BY id ASC LIMIT 1", orderNumber).Scan(&status)
	if err == nil {
		return status, true
	}
	if err == sql.ErrNoRows {
		return 0, false
	}

	t.Fatalf("QueryRow order ticket status error: %v", err)
	return 0, false
}

func (f *fakeOrderProgramRPC) ConfirmSeatFreeze(ctx context.Context, in *programrpc.ConfirmSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ConfirmSeatFreezeResp, error) {
	f.lastConfirmSeatFreezeReq = in
	f.confirmSeatFreezeCalls++
	if f.onConfirmSeatFreeze != nil {
		f.onConfirmSeatFreeze(in)
	}
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
	if f.onMockPay != nil {
		f.onMockPay(in)
	}
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

	ensureOrderTestDatabase(t, dataSource)

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

func ensureOrderTestDatabase(t *testing.T, dataSource string) {
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

	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci",
		strings.ReplaceAll(dbName, "`", "``"),
	)
	if _, err := adminDB.Exec(stmt); err != nil {
		t.Fatalf("create test database error: %v", err)
	}
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
	if fixture.ShowTimeID == 0 {
		fixture.ShowTimeID = fixture.ProgramID
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
				id, order_number, program_id, show_time_id, program_title, program_item_picture, program_place, program_show_time,
				program_permit_choose_seat, user_id, distribution_mode, take_ticket_mode, ticket_count, order_price,
				order_status, freeze_token, order_expire_time, create_order_time, cancel_order_time, pay_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.ProgramID,
			fixture.ShowTimeID,
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
		if fixture.ShowTimeID == 0 {
			fixture.ShowTimeID = lookupOrderShowTimeID(t, db, strings.Replace(table, "d_order_ticket_user_", "d_order_", 1), fixture.OrderNumber)
		}
		mustExecOrderSQL(
			t,
			db,
			`INSERT INTO `+table+` (
				id, order_number, show_time_id, user_id, ticket_user_id, ticket_user_name, ticket_user_id_number,
				ticket_category_id, ticket_category_name, ticket_price, seat_id, seat_row, seat_col,
				seat_price, order_status, create_order_time, create_time, edit_time, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fixture.ID,
			fixture.OrderNumber,
			fixture.ShowTimeID,
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

func lookupOrderShowTimeID(t *testing.T, db *sql.DB, table string, orderNumber int64) int64 {
	t.Helper()

	var showTimeID int64
	if err := db.QueryRow("SELECT show_time_id FROM "+table+" WHERE order_number = ? LIMIT 1", orderNumber).Scan(&showTimeID); err != nil {
		t.Fatalf("lookup order show_time_id error: %v", err)
	}

	return showTimeID
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
		ProgramId:                    programID,
		ShowTimeId:                   programID,
		ProgramGroupId:               programID + 1000,
		Title:                        "订单测试演出",
		Place:                        "测试场馆",
		ItemPicture:                  "https://example.com/order-program.jpg",
		ShowTime:                     "2026-12-31 19:30:00",
		ShowDayTime:                  "2026-12-31 00:00:00",
		ShowWeekTime:                 "周四",
		RushSaleOpenTime:             "2026-12-31 18:00:00",
		RushSaleEndTime:              "2026-12-31 19:00:00",
		PerOrderLimitPurchaseCount:   perOrderLimit,
		PerAccountLimitPurchaseCount: perAccountLimit,
		PermitChooseSeat:             0,
		TicketCategoryVoList: []*programrpc.ProgramPreorderTicketCategoryInfo{
			{
				Id:             ticketCategoryID,
				Introduce:      "普通票",
				Price:          ticketPrice,
				TotalNumber:    100,
				RemainNumber:   100,
				AdmissionQuota: 100,
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

func (f *fakeOrderProgramRPC) InvalidProgram(ctx context.Context, in *programrpc.ProgramInvalidReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ResetProgram(ctx context.Context, in *programrpc.ProgramResetReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListProgramCategories(ctx context.Context, in *programrpc.Empty, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListProgramCategoriesByType(ctx context.Context, in *programrpc.ProgramCategoryTypeReq, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListProgramCategoriesByParent(ctx context.Context, in *programrpc.ParentProgramCategoryReq, opts ...grpc.CallOption) (*programrpc.ProgramCategoryListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) BatchCreateProgramCategories(ctx context.Context, in *programrpc.ProgramCategoryBatchSaveReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
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

func (f *fakeOrderProgramRPC) GetProgramPreorder(ctx context.Context, in *programrpc.GetProgramPreorderReq, opts ...grpc.CallOption) (*programrpc.ProgramPreorderInfo, error) {
	f.lastGetProgramPreorderReq = in
	if resp, ok := f.getProgramPreorderRespByProgramID[in.GetShowTimeId()]; ok {
		return resp, f.getProgramPreorderErr
	}
	return f.getProgramPreorderResp, f.getProgramPreorderErr
}

func (f *fakeOrderProgramRPC) ListProgramShowTimesForRush(ctx context.Context, in *programrpc.ListProgramShowTimesForRushReq, opts ...grpc.CallOption) (*programrpc.ListProgramShowTimesForRushResp, error) {
	f.lastListProgramShowTimesForRushReq = in
	return f.listProgramShowTimesForRushResp, f.listProgramShowTimesForRushErr
}

func (f *fakeOrderProgramRPC) CreateProgramShowTime(ctx context.Context, in *programrpc.ProgramShowTimeAddReq, opts ...grpc.CallOption) (*programrpc.IdResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) UpdateProgramShowTime(ctx context.Context, in *programrpc.UpdateProgramShowTimeReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) PrimeSeatLedger(ctx context.Context, in *programrpc.PrimeSeatLedgerReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) CreateTicketCategory(ctx context.Context, in *programrpc.TicketCategoryAddReq, opts ...grpc.CallOption) (*programrpc.IdResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) GetTicketCategoryDetail(ctx context.Context, in *programrpc.TicketCategoryReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailInfo, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) ListTicketCategoriesByProgram(ctx context.Context, in *programrpc.ListTicketCategoriesByProgramReq, opts ...grpc.CallOption) (*programrpc.TicketCategoryDetailListResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) CreateSeat(ctx context.Context, in *programrpc.SeatAddReq, opts ...grpc.CallOption) (*programrpc.IdResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) BatchCreateSeats(ctx context.Context, in *programrpc.SeatBatchAddReq, opts ...grpc.CallOption) (*programrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) GetSeatRelateInfo(ctx context.Context, in *programrpc.SeatListReq, opts ...grpc.CallOption) (*programrpc.SeatRelateInfo, error) {
	return nil, nil
}

func (f *fakeOrderProgramRPC) AutoAssignAndFreezeSeats(ctx context.Context, in *programrpc.AutoAssignAndFreezeSeatsReq, opts ...grpc.CallOption) (*programrpc.AutoAssignAndFreezeSeatsResp, error) {
	f.lastAutoAssignAndFreezeSeatsReq = in
	f.autoAssignAndFreezeSeatsCalls++
	if f.autoAssignAndFreezeSeatsFunc != nil {
		return f.autoAssignAndFreezeSeatsFunc(ctx, in)
	}
	return f.autoAssignAndFreezeSeatsResp, f.autoAssignAndFreezeSeatsErr
}

func (f *fakeOrderProgramRPC) ReleaseSeatFreeze(ctx context.Context, in *programrpc.ReleaseSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ReleaseSeatFreezeResp, error) {
	f.lastReleaseSeatFreezeReq = in
	f.releaseSeatFreezeCalls++
	if f.onReleaseSeatFreeze != nil {
		f.onReleaseSeatFreeze(in)
	}
	return f.releaseSeatFreezeResp, f.releaseSeatFreezeErr
}

func newTracingOrderRepository(base repository.OrderRepository) *tracingOrderRepository {
	return &tracingOrderRepository{base: base}
}

func (r *tracingOrderRepository) record(event string) {
	r.events = append(r.events, event)
}

func (r *tracingOrderRepository) TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, repository.OrderTx) error) error {
	r.transactByOrderNumberCalls++
	txLabel := fmt.Sprintf("tx%d", r.transactByOrderNumberCalls)

	return r.base.TransactByOrderNumber(ctx, orderNumber, func(txCtx context.Context, tx repository.OrderTx) error {
		return fn(txCtx, &tracingOrderTx{
			OrderTx: tx,
			repo:    r,
			txLabel: txLabel,
		})
	})
}

func (r *tracingOrderRepository) TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, repository.OrderTx) error) error {
	return r.base.TransactByUserID(ctx, userID, fn)
}

func (r *tracingOrderRepository) FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	return r.base.FindOrderByNumber(ctx, orderNumber)
}

func (r *tracingOrderRepository) FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error) {
	return r.base.FindOrderTicketsByNumber(ctx, orderNumber)
}

func (r *tracingOrderRepository) FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error) {
	return r.base.FindOrderPageByUser(ctx, userID, orderStatus, pageNumber, pageSize)
}

func (r *tracingOrderRepository) FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error) {
	return r.base.FindExpiredUnpaidBySlot(ctx, logicSlot, before, limit)
}

func (r *tracingOrderRepository) CountActiveTicketsByUserShowTime(ctx context.Context, userID, showTimeID int64) (int64, error) {
	return r.base.CountActiveTicketsByUserShowTime(ctx, userID, showTimeID)
}

func (r *tracingOrderRepository) ListUnpaidReservationsByUserShowTime(ctx context.Context, userID, showTimeID int64) (map[int64]int64, error) {
	return r.base.ListUnpaidReservationsByUserShowTime(ctx, userID, showTimeID)
}

func (r *tracingOrderRepository) WalkActiveUserGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderUserGuard) error) error {
	return r.base.WalkActiveUserGuardsByShowTime(ctx, showTimeID, batchSize, fn)
}

func (r *tracingOrderRepository) WalkActiveViewerGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderViewerGuard) error) error {
	return r.base.WalkActiveViewerGuardsByShowTime(ctx, showTimeID, batchSize, fn)
}

func (r *tracingOrderRepository) RouteByUserID(ctx context.Context, userID int64) (sharding.Route, error) {
	return r.base.RouteByUserID(ctx, userID)
}

func (r *tracingOrderRepository) RouteByOrderNumber(ctx context.Context, orderNumber int64) (sharding.Route, error) {
	return r.base.RouteByOrderNumber(ctx, orderNumber)
}

func (t *tracingOrderTx) FindOrderByNumberForUpdate(ctx context.Context, orderNumber int64) (*model.DOrder, error) {
	t.repo.record(t.txLabel + ":find")
	return t.OrderTx.FindOrderByNumberForUpdate(ctx, orderNumber)
}

func (t *tracingOrderTx) UpdateCancelStatus(ctx context.Context, orderNumber int64, cancelTime time.Time) error {
	if t.repo.failUpdateCancelStatusN > 0 {
		t.repo.failUpdateCancelStatusN--
		t.repo.record(t.txLabel + ":update_cancel_fail")
		return t.repo.failUpdateCancelStatusErr
	}
	t.repo.record(t.txLabel + ":update_cancel")
	return t.OrderTx.UpdateCancelStatus(ctx, orderNumber, cancelTime)
}

func (t *tracingOrderTx) UpdatePayStatus(ctx context.Context, orderNumber int64, payTime time.Time) error {
	if t.repo.failUpdatePayStatusN > 0 {
		t.repo.failUpdatePayStatusN--
		t.repo.record(t.txLabel + ":update_pay_fail")
		return t.repo.failUpdatePayStatusErr
	}
	t.repo.record(t.txLabel + ":update_pay")
	return t.OrderTx.UpdatePayStatus(ctx, orderNumber, payTime)
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
