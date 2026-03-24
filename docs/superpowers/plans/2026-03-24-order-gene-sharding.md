# Order Gene Sharding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在订单侧落地基于基因法的分库分表，实现 `order_number` / `user_id` 双入口精准路由，并补齐在线迁移、切流和回滚能力。

**Architecture:** 以“稳定基因 + `logic_slot` + 版本化 `route_map`”为核心，把订单号生成、路由映射、分片仓储、双写迁移与槽位级切流拆成独立模块。订单域内主表、明细表和用户索引表始终同槽位绑定，状态流转继续保持单分片本地事务，迁移期通过双写、回填、校验、切读和回滚完成。

**Tech Stack:** Go, go-zero, gRPC, MySQL, Redis, Kafka, etcd, `goctl --style go_zero`, shell acceptance scripts, Go integration tests

---

## File Map

### 配置与启动

- Modify: `services/order-rpc/internal/config/config.go`
- Modify: `services/order-rpc/internal/svc/service_context.go`
- Modify: `services/order-rpc/etc/order-rpc.yaml`
- Modify: `services/order-rpc/etc/order-rpc.perf.yaml`
- Modify: `services/order-rpc/tests/config/config_test.go`
- Modify: `services/order-rpc/tests/integration/service_context_kafka_test.go`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go`

### 基因订单号与路由层

- Create: `services/order-rpc/sharding/order_number_codec.go`
- Create: `services/order-rpc/sharding/logic_slot.go`
- Create: `services/order-rpc/sharding/route_map.go`
- Create: `services/order-rpc/sharding/router.go`
- Create: `services/order-rpc/sharding/migration_mode.go`
- Create: `services/order-rpc/sharding/order_number_codec_test.go`
- Create: `services/order-rpc/sharding/route_map_test.go`

### SQL 与模型

- Create: `sql/order/d_user_order_index.sql`
- Create: `sql/order/d_order_route_legacy.sql`
- Create: `sql/order/sharding/d_order_shards.sql`
- Create: `sql/order/sharding/d_order_ticket_user_shards.sql`
- Create: `sql/order/sharding/d_user_order_index_shards.sql`
- Create: `services/order-rpc/internal/model/d_user_order_index_model_gen.go`
- Create: `services/order-rpc/internal/model/d_user_order_index_model.go`
- Create: `services/order-rpc/internal/model/d_order_route_legacy_model_gen.go`
- Create: `services/order-rpc/internal/model/d_order_route_legacy_model.go`
- Modify: `services/order-rpc/internal/model/d_order_model.go`
- Modify: `services/order-rpc/internal/model/d_order_ticket_user_model.go`

### 分片仓储

- Create: `services/order-rpc/repository/order_repository.go`
- Create: `services/order-rpc/repository/order_repository_legacy.go`
- Create: `services/order-rpc/repository/order_repository_sharded.go`
- Create: `services/order-rpc/repository/order_repository_dual_write.go`
- Create: `services/order-rpc/repository/order_transaction.go`
- Create: `services/order-rpc/repository/order_repository_test.go`

### 订单业务链路

- Modify: `services/order-rpc/internal/logic/create_order_logic.go`
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/order-rpc/internal/logic/get_order_logic.go`
- Modify: `services/order-rpc/internal/logic/get_order_service_view_logic.go`
- Modify: `services/order-rpc/internal/logic/list_orders_logic.go`
- Modify: `services/order-rpc/internal/logic/count_active_tickets_by_user_program_logic.go`
- Modify: `services/order-rpc/internal/logic/pay_order_logic.go`
- Modify: `services/order-rpc/internal/logic/cancel_order_logic.go`
- Modify: `services/order-rpc/internal/logic/pay_check_logic.go`
- Modify: `services/order-rpc/internal/logic/refund_order_logic.go`
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go`
- Modify: `services/order-rpc/internal/logic/close_expired_orders_logic.go`
- Modify: `services/order-rpc/internal/limitcache/purchase_limit_loader.go`
- Modify: `services/order-rpc/tests/integration/create_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_consumer_logic_test.go`
- Modify: `services/order-rpc/tests/integration/query_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/pay_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/cancel_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/pay_check_logic_test.go`
- Modify: `services/order-rpc/tests/integration/refund_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/close_expired_orders_logic_test.go`

### 迁移与回滚

- Create: `jobs/order-migrate/order_migrate.go`
- Create: `jobs/order-migrate/internal/config/config.go`
- Create: `jobs/order-migrate/internal/svc/service_context.go`
- Create: `jobs/order-migrate/internal/logic/backfill_orders_logic.go`
- Create: `jobs/order-migrate/internal/logic/verify_orders_logic.go`
- Create: `jobs/order-migrate/internal/logic/switch_slots_logic.go`
- Create: `jobs/order-migrate/internal/logic/rollback_slots_logic.go`
- Create: `jobs/order-migrate/etc/order-migrate.yaml`
- Create: `jobs/order-migrate/tests/integration/backfill_orders_logic_test.go`
- Create: `jobs/order-migrate/tests/integration/verify_orders_logic_test.go`
- Create: `jobs/order-migrate/tests/integration/switch_slots_logic_test.go`
- Create: `jobs/order-migrate/tests/integration/rollback_slots_logic_test.go`

### 文档与脚本

- Create: `docs/architecture/order-gene-sharding-runbook.md`
- Create: `docs/architecture/order-gene-sharding-rollback-runbook.md`
- Create: `scripts/acceptance/order_gene_sharding_smoke.sh`
- Create: `scripts/acceptance/order_gene_sharding_migration.sh`

### 参考设计

- Reference: `docs/superpowers/specs/2026-03-24-order-gene-sharding-design.md`

### 路径约束说明

`jobs/order-migrate` 需要复用订单号编解码和分片仓储，因此 `sharding/` 与 `repository/` 不能放在 `services/order-rpc/internal/` 下，否则根级 job 无法导入。它们应作为 `services/order-rpc/` 下的服务级共享包存在，供 `internal/logic/` 与 `jobs/order-migrate/` 同时使用。

### Task 1: 固化基因订单号与槽位路由测试

**Files:**
- Create: `services/order-rpc/sharding/order_number_codec_test.go`
- Create: `services/order-rpc/sharding/route_map_test.go`

- [ ] **Step 1: 写失败测试，固定编码与路由语义**

先写白盒测试，锁定以下行为：

```go
func TestLogicSlotByUserIDMatchesOrderNumber(t *testing.T) {}
func TestParseLegacyOrderNumberRequiresDirectoryLookup(t *testing.T) {}
func TestRouteMapSelectsPhysicalTargetByVersionAndSlot(t *testing.T) {}
func TestMigrationModeRejectsIllegalStateTransition(t *testing.T) {}
```

- [ ] **Step 2: 运行测试确认先失败**

Run: `go test ./services/order-rpc/sharding -count=1`
Expected: FAIL，提示 `sharding` 包或目标函数尚不存在。

### Task 2: 实现基因订单号编解码与 `route_map`

**Files:**
- Create: `services/order-rpc/sharding/order_number_codec.go`
- Create: `services/order-rpc/sharding/logic_slot.go`
- Create: `services/order-rpc/sharding/route_map.go`
- Create: `services/order-rpc/sharding/router.go`
- Create: `services/order-rpc/sharding/migration_mode.go`

- [ ] **Step 1: 写最小类型定义**

先把核心结构体和接口立住：

```go
type OrderNumberParts struct {
	TimePart   int64
	DBGene     uint8
	TableGene  uint8
	WorkerID   int64
	Sequence   int64
	Legacy     bool
}

type Route struct {
	LogicSlot   int
	DBKey       string
	TableSuffix string
	Version     string
	WriteMode   string
	Status      string
}
```

- [ ] **Step 2: 实现稳定 hash、订单号编码和解码**

实现：

- `LogicSlotByUserID(userID int64) int`
- `BuildOrderNumber(userID int64, now time.Time, workerID, seq int64) int64`
- `ParseOrderNumber(orderNumber int64) (OrderNumberParts, error)`
- `RouteByUserID` / `RouteByOrderNumber`

明确旧 `xid` 订单号的识别逻辑：返回 `Legacy=true`，交由目录表兜底。

- [ ] **Step 3: 实现 `route_map` 快照与状态机**

最小实现：

```go
type RouteEntry struct {
	Version     string
	LogicSlot   int
	DBKey       string
	TableSuffix string
	Status      string
	WriteMode   string
}
```

要求：

- 支持按 `logic_slot` 查当前路由
- 支持校验槽位状态跃迁
- 支持只读快照，不允许业务线程动态拼装

- [ ] **Step 4: 运行白盒测试**

Run: `go test ./services/order-rpc/sharding -count=1`
Expected: PASS

- [ ] **Step 5: 提交阶段性结果**

```bash
git add services/order-rpc/sharding
git commit -m "feat: add order gene sharding router core"
```

### Task 3: 扩展配置与 ServiceContext，支持多数据源和路由快照

**Files:**
- Modify: `services/order-rpc/internal/config/config.go`
- Modify: `services/order-rpc/internal/svc/service_context.go`
- Modify: `services/order-rpc/etc/order-rpc.yaml`
- Modify: `services/order-rpc/etc/order-rpc.perf.yaml`
- Modify: `services/order-rpc/tests/config/config_test.go`
- Modify: `services/order-rpc/tests/integration/service_context_kafka_test.go`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go`

- [ ] **Step 1: 写失败测试，固定新配置结构**

补配置测试，要求配置至少支持：

```yaml
Sharding:
  Mode: legacy_only
  LegacyMySQL: ...
  Shards:
    order-db-0: ...
    order-db-1: ...
  RouteMap:
    Version: v1
    Entries: ...
```

- [ ] **Step 2: 扩展 `Config` 与 `ServiceContext`**

新增：

- 旧表连接
- 分片连接池 map
- 路由快照加载器
- 分片模式开关

不要在业务逻辑里直接管理连接选择，所有连接都通过 `ServiceContext` 暴露给仓储层。

- [ ] **Step 3: 改造测试 helper，支持多库建表和清理**

让 `order_test_helpers_test.go` 能：

- 初始化 legacy 表
- 初始化分片表
- 初始化目录表
- 支持按槽位清理数据

- [ ] **Step 4: 运行配置与启动相关测试**

Run: `go test ./services/order-rpc/tests/config ./services/order-rpc/tests/integration -run 'TestServiceContext|TestOrderRPCStartup' -count=1`
Expected: PASS

### Task 4: 落 SQL、模型和动态表名支持

**Files:**
- Create: `sql/order/d_user_order_index.sql`
- Create: `sql/order/d_order_route_legacy.sql`
- Create: `sql/order/sharding/d_order_shards.sql`
- Create: `sql/order/sharding/d_order_ticket_user_shards.sql`
- Create: `sql/order/sharding/d_user_order_index_shards.sql`
- Create: `services/order-rpc/internal/model/d_user_order_index_model_gen.go`
- Create: `services/order-rpc/internal/model/d_user_order_index_model.go`
- Create: `services/order-rpc/internal/model/d_order_route_legacy_model_gen.go`
- Create: `services/order-rpc/internal/model/d_order_route_legacy_model.go`
- Modify: `services/order-rpc/internal/model/d_order_model.go`
- Modify: `services/order-rpc/internal/model/d_order_ticket_user_model.go`

- [ ] **Step 1: 先写 DDL**

新增：

- 用户订单索引表 DDL
- 历史路由目录表 DDL
- 分片表模板 DDL

要求每张表都保留与当前订单状态流转兼容的关键索引。

- [ ] **Step 2: 用 `goctl --style go_zero` 生成新增模型**

Run:

```bash
goctl model mysql ddl --src sql/order/d_user_order_index.sql --dir services/order-rpc/internal/model --style go_zero
goctl model mysql ddl --src sql/order/d_order_route_legacy.sql --dir services/order-rpc/internal/model --style go_zero
```

Expected: 生成 `*_model_gen.go` 文件，文件名保持下划线风格。

- [ ] **Step 3: 给现有模型补动态表名构造器**

为订单主表、订单明细表、新索引表、目录表补 `New...ModelWithTable(conn, table string)` 或等价构造器，避免为每个物理后缀生成一套重复模型。

- [ ] **Step 4: 运行模型相关测试或最小编译验证**

Run: `go test ./services/order-rpc/internal/model ./services/order-rpc/tests/integration -run 'TestCreateOrder|TestListOrders' -count=1`
Expected: 至少完成编译，集成测试此时可因仓储未接入而失败。

### Task 5: 引入订单分片仓储与双写抽象

**Files:**
- Create: `services/order-rpc/repository/order_repository.go`
- Create: `services/order-rpc/repository/order_repository_legacy.go`
- Create: `services/order-rpc/repository/order_repository_sharded.go`
- Create: `services/order-rpc/repository/order_repository_dual_write.go`
- Create: `services/order-rpc/repository/order_transaction.go`
- Create: `services/order-rpc/repository/order_repository_test.go`
- Modify: `services/order-rpc/internal/svc/service_context.go`

- [ ] **Step 1: 写失败测试，固定仓储接口**

先写接口级测试，要求仓储至少暴露：

```go
type OrderRepository interface {
	TransactByOrderNumber(ctx context.Context, orderNumber int64, fn func(context.Context, OrderTx) error) error
	TransactByUserID(ctx context.Context, userID int64, fn func(context.Context, OrderTx) error) error
	FindOrderByNumber(ctx context.Context, orderNumber int64) (*model.DOrder, error)
	FindOrderTicketsByNumber(ctx context.Context, orderNumber int64) ([]*model.DOrderTicketUser, error)
	FindOrderPageByUser(ctx context.Context, userID, orderStatus, pageNumber, pageSize int64) ([]*model.DOrder, int64, error)
}
```

- [ ] **Step 2: 先实现 legacy 仓储**

保持当前行为不变，让业务逻辑先通过仓储接口跑在旧单表上。

- [ ] **Step 3: 实现 sharded 与 dual-write 仓储**

要求：

- 新分片仓储按 `Route` 命中唯一库表
- dual-write 仓储明确“主写/影子写”边界
- 影子写失败必须返回结构化错误，方便补偿

- [ ] **Step 4: 在 `ServiceContext` 注入仓储**

禁止逻辑层继续直接依赖裸 `DOrderModel` + `SqlConn` 做跨模式切换。

- [ ] **Step 5: 运行仓储层测试**

Run: `go test ./services/order-rpc/repository -count=1`
Expected: PASS

- [ ] **Step 6: 提交阶段性结果**

```bash
git add services/order-rpc/repository services/order-rpc/internal/model services/order-rpc/internal/svc
git commit -m "feat: add order sharding repository layer"
```

### Task 6: 改造下单写链路和历史路由目录写入

**Files:**
- Modify: `services/order-rpc/internal/logic/create_order_logic.go`
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/order-rpc/internal/logic/order_create_event_builder.go`
- Modify: `services/order-rpc/internal/logic/order_create_event_mapper.go`
- Modify: `services/order-rpc/tests/integration/create_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_consumer_logic_test.go`

- [ ] **Step 1: 写失败测试，固定新订单号与三表写入**

补测试证明：

- `CreateOrder` 返回的新订单号可解析出基因槽位
- consumer 落库后同时写入 `d_order`、`d_order_ticket_user`、`d_user_order_index`
- `legacy_only` 与 `dual_write_shadow` 模式都可工作

- [ ] **Step 2: 用新 codec 替换 `xid.New()` 订单号生成**

只替换订单号，不替换其他 `id` 字段的雪花 ID 生成。

- [ ] **Step 3: 让 consumer 通过仓储写入三表**

把当前直接 `SqlConn.TransactCtx` + 两张表写入改成仓储事务，新增用户索引表写入。

- [ ] **Step 4: 对旧格式订单保留目录表写入能力**

为后续回填与兼容准备 `d_order_route_legacy` 写入路径，但不要让新格式订单多做无意义目录写入。

- [ ] **Step 5: 运行下单相关集成测试**

Run: `go test ./services/order-rpc/tests/integration -run 'TestCreateOrder|TestCreateOrderConsumer' -count=1`
Expected: PASS

### Task 7: 改造详情、列表与状态流转链路

**Files:**
- Modify: `services/order-rpc/internal/logic/get_order_logic.go`
- Modify: `services/order-rpc/internal/logic/get_order_service_view_logic.go`
- Modify: `services/order-rpc/internal/logic/list_orders_logic.go`
- Modify: `services/order-rpc/internal/logic/count_active_tickets_by_user_program_logic.go`
- Modify: `services/order-rpc/internal/logic/pay_order_logic.go`
- Modify: `services/order-rpc/internal/logic/cancel_order_logic.go`
- Modify: `services/order-rpc/internal/logic/pay_check_logic.go`
- Modify: `services/order-rpc/internal/logic/refund_order_logic.go`
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go`
- Modify: `services/order-rpc/internal/limitcache/purchase_limit_loader.go`
- Modify: `services/order-rpc/tests/integration/query_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/pay_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/cancel_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/pay_check_logic_test.go`
- Modify: `services/order-rpc/tests/integration/refund_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/purchase_limit_cache_test.go`

- [ ] **Step 1: 写失败测试，先锁定双入口精准查询**

补测试证明：

- `GetOrder(orderNumber)` 不读扩散
- `ListOrders(userId)` 命中 `d_user_order_index`
- 旧格式订单号能通过目录表查询到详情

- [ ] **Step 2: 先改查询链路**

让详情查询统一走：

```go
route, err := repo.RouteByOrderNumber(...)
order, err := repo.FindOrderByNumber(...)
```

让列表查询统一走：

```go
orders, total, err := repo.FindOrderPageByUser(...)
```

- [ ] **Step 3: 再改支付、取消、退款与对账**

把当前直接依赖 `SqlConn.TransactCtx` 的逻辑全部迁到仓储事务上，确保同槽位下主表与明细表仍是单分片本地事务。

- [ ] **Step 4: 修正限购账本加载入口**

`purchase_limit_loader` 不能再直接假设单表，需要通过仓储或查询接口读取用户维度聚合。

- [ ] **Step 5: 运行查询与状态流转测试**

Run: `go test ./services/order-rpc/tests/integration -run 'TestListOrders|TestGetOrder|TestPayOrder|TestCancelOrder|TestPayCheck|TestRefundOrder|TestPurchaseLimit' -count=1`
Expected: PASS

- [ ] **Step 6: 提交阶段性结果**

```bash
git add services/order-rpc/internal/logic services/order-rpc/internal/limitcache services/order-rpc/tests/integration
git commit -m "feat: route order query and state flows by gene shards"
```

### Task 8: 改造超时关单为槽位扫描

**Files:**
- Modify: `services/order-rpc/internal/logic/close_expired_orders_logic.go`
- Modify: `services/order-rpc/repository/order_repository.go`
- Modify: `jobs/order-close/internal/config/config.go`
- Modify: `jobs/order-close/internal/logic/closeexpiredorderslogic.go`
- Modify: `jobs/order-close/etc/order-close.yaml`
- Modify: `services/order-rpc/tests/integration/close_expired_orders_logic_test.go`
- Modify: `jobs/order-close/tests/integration/closeexpiredorderslogic_test.go`

- [ ] **Step 1: 写失败测试，固定槽位扫描行为**

补测试证明：

- `CloseExpiredOrders` 会按槽位遍历
- 每次扫描只命中单分片
- 结果仍返回总关闭数

- [ ] **Step 2: 给仓储补“按槽位扫描过期未支付订单”接口**

例如：

```go
FindExpiredUnpaidBySlot(ctx context.Context, logicSlot int, before time.Time, limit int64) ([]*model.DOrder, error)
```

- [ ] **Step 3: 改造 order-close job 配置**

支持：

- 扫描槽位范围
- 每轮槽位批次大小
- 上次扫描位置 checkpoint

- [ ] **Step 4: 运行关单相关测试**

Run: `go test ./services/order-rpc/tests/integration ./jobs/order-close/tests/integration -run 'TestCloseExpiredOrders|TestRunOnce' -count=1`
Expected: PASS

### Task 9: 落迁移作业，补回填、校验、切流与回滚

**Files:**
- Create: `jobs/order-migrate/order_migrate.go`
- Create: `jobs/order-migrate/internal/config/config.go`
- Create: `jobs/order-migrate/internal/svc/service_context.go`
- Create: `jobs/order-migrate/internal/logic/backfill_orders_logic.go`
- Create: `jobs/order-migrate/internal/logic/verify_orders_logic.go`
- Create: `jobs/order-migrate/internal/logic/switch_slots_logic.go`
- Create: `jobs/order-migrate/internal/logic/rollback_slots_logic.go`
- Create: `jobs/order-migrate/etc/order-migrate.yaml`
- Create: `jobs/order-migrate/tests/integration/backfill_orders_logic_test.go`
- Create: `jobs/order-migrate/tests/integration/verify_orders_logic_test.go`
- Create: `jobs/order-migrate/tests/integration/switch_slots_logic_test.go`
- Create: `jobs/order-migrate/tests/integration/rollback_slots_logic_test.go`

- [ ] **Step 1: 写失败测试，固定迁移状态机**

至少覆盖：

- 回填断点续跑
- 校验发现行数或聚合不一致
- 槽位切新读
- 槽位回滚到旧读

- [ ] **Step 2: 实现 `backfill`**

要求：

- 从旧表按游标扫描
- 幂等写入新分片三表
- 对旧格式订单额外写目录表
- 持久化 checkpoint

- [ ] **Step 3: 实现 `verify`**

最少比较：

- 行数
- 金额总和
- 状态分布
- 随机抽样详情
- 随机抽样列表

- [ ] **Step 4: 实现 `switch` 与 `rollback`**

让作业只改槽位级 `route_map` 状态，不直接改业务代码。

- [ ] **Step 5: 运行迁移 job 测试**

Run: `go test ./jobs/order-migrate/tests/integration -count=1`
Expected: PASS

- [ ] **Step 6: 提交阶段性结果**

```bash
git add jobs/order-migrate
git commit -m "feat: add order shard migration and rollback jobs"
```

### Task 10: 补 runbook、验收脚本与全链路验证

**Files:**
- Create: `docs/architecture/order-gene-sharding-runbook.md`
- Create: `docs/architecture/order-gene-sharding-rollback-runbook.md`
- Create: `scripts/acceptance/order_gene_sharding_smoke.sh`
- Create: `scripts/acceptance/order_gene_sharding_migration.sh`
- Modify: `docs/api/order-checkout-acceptance.md`
- Modify: `docs/api/order-checkout-failure-acceptance.md`

- [ ] **Step 1: 写最小验收脚本**

脚本至少验证：

- 新订单号可解析基因位
- 详情查询命中单槽位
- 列表查询命中单槽位
- 迁移切新后读写一致
- 回滚后旧读恢复

- [ ] **Step 2: 写 runbook**

runbook 需要写清：

- 初始化分片库表
- 开启双写
- 执行回填
- 执行校验
- 按槽位切流
- 按槽位回滚

- [ ] **Step 3: 跑最小全链路验证**

Run:

```bash
go test ./services/order-rpc/tests/integration ./jobs/order-close/tests/integration ./jobs/order-migrate/tests/integration -count=1
bash scripts/acceptance/order_gene_sharding_smoke.sh
```

Expected: PASS

- [ ] **Step 4: 最终提交**

```bash
git add docs scripts
git commit -m "docs: add order gene sharding rollout runbooks"
```

### Task 11: 合并验证与发布前检查

**Files:**
- No code changes

- [ ] **Step 1: 跑聚合验证**

Run:

```bash
go test ./services/order-rpc/sharding ./services/order-rpc/repository ./services/order-rpc/tests/integration ./jobs/order-close/tests/integration ./jobs/order-migrate/tests/integration -count=1
```

Expected: PASS

- [ ] **Step 2: 做一次手工发布前检查**

确认：

- 新旧订单号兼容路径存在
- `legacy_only` / `dual_write_shadow` / `dual_write_new_read_old` / `dual_write_new_read_new` / `shard_only` 均可配置
- 旧格式订单必须依赖 `d_order_route_legacy` 精准定位
- 没有业务逻辑直接绕过仓储访问单表

- [ ] **Step 3: 记录验收结论**

最终说明里要明确：

- 哪些是已真实落地的工程能力
- 哪些仍是预留扩展位
- 默认发布模式是否仍为 `legacy_only`
