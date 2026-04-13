# Rush Inventory Preheat Outbox Refactor Design

**日期**: 2026-04-13

## 背景

当前 `jobs/rush-inventory-preheat-worker` 仍是单一 worker 目录形态，`services/program-rpc` 直接将预热任务投递到 asynq。该结构与已经完成重构的 `jobs/order-close` 不一致，导致延迟任务的生产、派发、消费职责分散，后续同类任务无法复用同一套目录与运行模式。

## 目标

- 将 `jobs/rush-inventory-preheat-worker` 重构为 `jobs/rush-inventory-preheat`
- 对齐 `jobs/order-close` 的目录布局，拆分 `dispatcher` 与 `worker`
- 将 `program-rpc` 的预热任务生产方式从“直接投递 asynq”改为“写入 delay task outbox”
- 将任务定义收拢到 job 自身，避免消息契约散落在服务目录

## 非目标

- 不做双写兼容
- 不保留旧目录并行运行
- 不扩展新的业务规则，仅整理结构并切换投递链路

## 目标结构

### Job 目录

新增 `jobs/rush-inventory-preheat/`，结构如下：

- `cmd/dispatcher/main.go`：周期扫描 outbox，发布待执行任务
- `cmd/worker/main.go`：启动 asynq worker，消费预热任务
- `etc/rush-inventory-preheat-dispatcher.yaml`：dispatcher 配置
- `etc/rush-inventory-preheat-worker.yaml`：worker 配置
- `internal/config/config.go`：统一配置模型
- `internal/dispatch/`：outbox store 与调度逻辑
- `internal/svc/dispatcher_service_context.go`：dispatcher 依赖装配
- `internal/svc/worker_service_context.go`：worker 依赖装配
- `internal/worker/`：serve mux 与任务处理逻辑
- `taskdef/rush_inventory_preheat_task.go`：任务类型、payload、task key、message 构造与解析
- `tests/integration/`：dispatcher 与 worker 集成测试

### Program RPC

`services/program-rpc` 继续负责判定“何时需要库存预热”，但不再直接向 asynq 入队，而是写入 `d_delay_task_outbox`。新的链路为：

1. `program-rpc` 在创建或更新场次时决定需要预热
2. `program-rpc` 构造 `rush inventory preheat` 的延迟任务消息并写入 outbox
3. `jobs/rush-inventory-preheat/cmd/dispatcher` 扫描并发布任务
4. `jobs/rush-inventory-preheat/cmd/worker` 消费任务并执行预热

## 数据流

### 生产端

- 入口仍为 `scheduleRushInventoryPreheat(...)`
- `RushInventoryPreheatClient.Enqueue(...)` 保留业务语义，但底层改为写 outbox
- 写入记录包含 `task_type`、`task_key`、`payload`、`execute_at`

### 派发端

- 仅扫描 `task_type = program.rush_inventory_preheat` 的待发布记录
- 发布成功或任务重复时，标记为已发布
- 发布失败时，累计失败次数并记录最后错误

### 消费端

- 从 payload 中解析 `show_time_id` 和 `expected_open_time`
- 查询场次记录并校验当前开售时间是否仍匹配
- 调用 `order-rpc.PrimeAdmissionQuota`
- 调用 `program-rpc.PrimeSeatLedger`
- 成功后更新 `inventory_preheat_status`

## 任务契约

任务契约从 `services/program-rpc/preheatqueue` 迁移到 job 自身的 `taskdef/`。这样消息格式、任务类型、task key 与 worker 处理逻辑由同一模块维护，`program-rpc` 仅依赖这个稳定契约生成 outbox 消息。

## 配置

配置风格对齐 `jobs/order-close`：

- `Interval`：dispatcher 轮询间隔
- `BatchSize`：单次扫描批量
- `Shards`：用于写入或扫描 outbox 的 MySQL 分片配置
- `Asynq`：queue、retry、unique TTL、redis、worker 并发等
- `OrderRpc`、`ProgramRpc`：worker 执行预热时的 RPC 依赖

## 脚本与引用更新

- 更新 `scripts/deploy/start_backend.sh`
- 清理旧的 `jobs/rush-inventory-preheat-worker/...` 路径引用
- 测试导入路径统一切换到新 job 目录

## 测试策略

- `taskdef` 单测：覆盖 payload 编解码、task key、message 构造
- `dispatcher` 集成测试：覆盖发布成功、重复投递视为成功、发布失败回写
- `worker` 单测与集成测试：覆盖任务路由、payload 非法跳过、开售时间不匹配跳过、成功调用双 RPC 并更新状态
- `program-rpc` 相关测试：覆盖创建/更新场次时写 outbox，而不是直接 enqueue

## 风险与控制

- 路径迁移会影响启动脚本与导入路径，必须一并更新
- 若遗漏 `preheatqueue` 旧引用，会出现编译失败或双契约并存
- 若 outbox 配置与现有 program 分片不一致，会导致 dispatcher 扫描不到记录，需要测试覆盖
