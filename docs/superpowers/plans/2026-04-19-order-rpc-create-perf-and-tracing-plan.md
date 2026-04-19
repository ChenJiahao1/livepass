# Order RPC Create 压测与链路追踪计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 隔离并验证 `CreateOrder` 热路径的真实瓶颈，拿到 `gateway/http` 与 `order-rpc/PerfCreateOrder gRPC` 两组可对比结果，并补齐可落地的链路追踪手段。

**Architecture:** 先保持现有 2k 数据模型不变，只替换压测入口与运行模式，避免一次改多个变量。先做“只读式实验”确认瓶颈位置，再决定是否需要扩 `order-rpc` 实例、扩 Kafka partitions，或优化 Redis admission/Lua。观测层分为两档：第一档用现有日志和 Prometheus 指标快速定位；第二档补 OpenTelemetry/阶段耗时埋点，把 `CreateOrder` 内部拆到 Redis admit、token verify、async send 三段。

**Tech Stack:** Go, go-zero, zrpc, k6, gRPC, Redis Lua, Kafka, Prometheus, OpenTelemetry

---

## 文件分解

**现有关键文件：**
- `tests/perf/rush_create_order.js`：当前 HTTP 压测脚本，包含 `/order/create` + `/order/poll`
- `tests/perf/lib/summary.js`：k6 汇总输出
- `scripts/perf/prepare_rush_perf_dataset.sh`：构造 2k 用户、2k 座位、签发 purchaseToken
- `scripts/deploy/rebuild_databases.sh`：重置 MySQL / Redis / Kafka
- `scripts/deploy/start_backend.sh`：启动 perf 配置的全部服务
- `services/order-rpc/internal/logic/create_order_logic.go`：`CreateOrder` 热路径入口
- `services/order-rpc/internal/logic/order_create_async_sender.go`：Kafka 异步发送
- `services/order-rpc/internal/rush/attempt_store.go`：Redis Lua admission
- `services/order-rpc/internal/logic/order_create_consumer_runner.go`：consumer worker 启动入口
- `services/order-rpc/etc/order.perf.yaml`：`order-rpc` perf 配置
- `services/gateway-api/etc/gateway-api.perf.yaml`：`gateway` tracing / Prometheus 配置

**计划新增文件：**
- `tests/perf/rush_create_order_rpc.js`：直压 `order-rpc/PerfCreateOrder` 的 gRPC 脚本
- `tests/perf/lib/grpc_dataset.js`：把现有 `users.json` 映射成 gRPC 入参
- `scripts/perf/run_rush_rpc_perf.sh`：一键执行“准备数据 → 启动模式 → 建隧道/直连 → 跑 RPC 压测 → 导出结果”
- `scripts/perf/analyze_create_order_path.sh`：统一汇总 k6、gateway、order-rpc、Redis、Kafka 的结果
- `docs/perf/2026-04-19-rush-rpc-vs-http-summary.md`：最终实验结论文档

**如需补链路追踪，可能新增/修改：**
- `services/order-rpc/internal/server/order_rpc_server.go`：若要挂 gRPC unary interceptor，可从这里接入
- `services/order-rpc/internal/logic/create_order_logic.go`：补阶段耗时埋点
- `services/order-rpc/internal/logic/order_create_async_sender.go`：补 async send 耗时/失败原因日志
- `services/order-rpc/etc/order.perf.yaml`：若启用 OTLP tracing，补 Telemetry 配置
- `deploy/docker-compose/docker-compose.infrastructure.yml`：若本地加 collector / Jaeger / Tempo，可在这里挂载

---

### Task 1: 固化实验口径与基线

**Files:**
- Modify: `docs/perf/2026-04-19-rush-rpc-vs-http-summary.md`
- Verify: `tmp/perf/results/`

- [ ] **Step 1: 固定本轮实验口径**

口径统一为：
- `showTimeId=30001`
- `ticketCategoryId=40001`
- `USER_COUNT=2000`
- `SEAT_COUNT=2000`
- `MIN_TICKET_COUNT=1`
- `MAX_TICKET_COUNT=1`
- `VUS=2000`
- `ITERATIONS=2000`

- [ ] **Step 2: 记录现有 HTTP 基线**

记录当前已有结果：
- `tmp/perf/results/rush-2k-20260419124351/summary.json`
- `tmp/perf/results/rush-2k-20260419124351/gateway_qps.json`
- `tmp/perf/results/rush-2k-20260419124351/final_state.json`

需要在报告里明确区分三种 QPS：
- `k6 场景总时长口径`
- `gateway 首请求到最后响应口径`
- `仅 /order/create handler 响应窗口口径`

- [ ] **Step 3: 明确判断标准**

定义实验成功输出：
- RPC 直压 QPS
- 两次压测的 `avg/p95/p99`
- Redis / Kafka / order-rpc CPU 与延迟变化
- 是否能证明瓶颈在 `gateway/api`、`order-rpc` 本身，还是 `Redis admission`

---

### Task 2: 增加直压 `order-rpc/PerfCreateOrder` 的压测脚本

**Files:**
- Create: `tests/perf/rush_create_order_rpc.js`
- Create: `tests/perf/lib/grpc_dataset.js`
- Test: `tests/perf/lib/summary.js`

- [ ] **Step 1: 复用现有数据集结构**

输入继续使用 `users.json`，每条记录只取：
- `userId`
- `purchaseToken`

gRPC 请求体映射为：

```js
{
  userId: row.userId,
  purchaseToken: row.purchaseToken,
}
```

- [ ] **Step 2: 写最小可跑的 gRPC k6 脚本**

脚本职责：
- 连接 `order-rpc` 的 `:8082`
- 调用 `order.OrderRpc/PerfCreateOrder`
- 统计成功率、失败率、avg、p95、p99
- 不做 `/order/poll`

Run:
`k6 run --vus 2000 --iterations 2000 tests/perf/rush_create_order_rpc.js`

Expected:
- 成功连接 gRPC 服务
- 输出 `summary.json`

- [ ] **Step 3: 保持与 HTTP 版本相同的 summary 输出**

延续 `tests/perf/lib/summary.js` 的字段风格，新增或保证输出：
- `createSuccessCount`
- `businessFailureCount`
- `avg`
- `p95`
- `p99`

- [ ] **Step 4: 增加单独的“纯 create”说明**

在脚本注释或文档里明确：
- 此脚本只测 `PerfCreateOrder`
- 不包含 `/order/poll`
- 只用于测 admission + token verify + async handoff 这条同步返回路径

---

### Task 4: 增加一键 RPC 实验脚本

**Files:**
- Create: `scripts/perf/run_rush_rpc_perf.sh`
- Create: `scripts/perf/analyze_create_order_path.sh`
- Test: `tmp/perf/results/`

- [ ] **Step 1: 固化普通 RPC 直压脚本**

流程：
1. 拉最新代码
2. 重置日志、MySQL、Redis、Kafka
3. 启动 perf 服务
4. 准备 2k 数据集
5. 运行 gRPC `PerfCreateOrder` 压测
6. 汇总结果到 `tmp/perf/results/<run-id>/`

- [ ] **Step 2: 统一导出结果目录**

每轮实验都至少生成：
- `summary.json`
- `timing.json`
- `gateway_qps.json`
- `order_rpc_qps.json`
- `final_state.json`
- `notes.txt`

- [ ] **Step 3: 统一计算两个关键 QPS**

必须输出：
- `客户端起压到全部响应完成` 的 QPS
- `服务端日志首请求到最后响应` 的 QPS

---

### Task 5: 给 `CreateOrder` 热路径补阶段耗时埋点

**Files:**
- Modify: `services/order-rpc/internal/logic/create_order_logic.go`
- Modify: `services/order-rpc/internal/logic/order_create_async_sender.go`
- Modify: `docs/perf/2026-04-19-rush-rpc-vs-http-summary.md`

- [ ] **Step 1: 只补最小阶段埋点，不先上重型 tracing**

先记录三段耗时：
- `purchase_token_verify_ms`
- `redis_admit_ms`
- `async_dispatch_schedule_ms`

示意输出：

```go
logx.WithContext(ctx).Infow(
  "create order perf stage",
  logx.Field("orderNumber", orderNumber),
  logx.Field("userId", in.GetUserId()),
  logx.Field("purchaseTokenVerifyMs", verifyCost.Milliseconds()),
  logx.Field("redisAdmitMs", admitCost.Milliseconds()),
  logx.Field("asyncDispatchScheduleMs", dispatchCost.Milliseconds()),
)
```

- [ ] **Step 2: 给异常路径也打印统一字段**

失败时至少打印：
- `rejectCode`
- `grpcCode`
- `redis_admit_ms`
- `reasonCode`

- [ ] **Step 3: 控制日志量**

压测场景下不要全量打详细日志；二选一：
- 只在 perf 模式开启
- 或按采样率打印

---

### Task 6: 补真正可看的链路追踪

**Files:**
- Modify: `services/order-rpc/etc/order.perf.yaml`
- Modify: `deploy/docker-compose/docker-compose.infrastructure.yml`
- Modify: `services/order-rpc/order.go`
- Modify: `services/order-rpc/internal/server/order_rpc_server.go`

- [ ] **Step 1: 先确认当前已有能力**

当前已知：
- `gateway-api` 已配置 OTLP，见 `services/gateway-api/etc/gateway-api.perf.yaml`
- 但当前环境没有 collector，日志里会报：
  `Post "http://localhost:4318/v1/traces": connect: connection refused`
- `order-rpc` 目前只有 `Prometheus`，没有显式 `Telemetry` 配置

- [ ] **Step 2: 决定 tracing 方案**

推荐优先级：
1. **低成本**：结构化阶段日志
2. **中成本**：给 `order-rpc` 补 OTLP telemetry + 本地 collector / Jaeger
3. **高成本**：补 gRPC unary interceptor，把 `PerfCreateOrder` / `CreateOrder` 内部阶段拆成 span

- [ ] **Step 3: 如果接 OTLP，统一 collector**

建议：
- `gateway-api`
- `order-api`
- `order-rpc`
- `program-rpc`

都指向同一个 collector，再进 Jaeger/Tempo 看 span 树。

- [ ] **Step 4: 如果不接全链路，至少补 Prometheus 指标**

新增 histogram：
- `order_create_token_verify_ms`
- `order_create_redis_admit_ms`
- `order_create_async_dispatch_schedule_ms`

这样即使没有 tracing UI，也能从 `/metrics` 看 p95/p99。

---

### Task 7: 设计新会话实验顺序

**Files:**
- Modify: `docs/perf/2026-04-19-rush-rpc-vs-http-summary.md`

- [ ] **Step 1: 实验 A：保留现状，直压 gRPC `PerfCreateOrder`**

目标：
- 排除 `gateway-api`
- 排除 `order-api`
- 看 `order-rpc` 接入极限

预期解释：
- 如果 QPS 比 HTTP 明显高，说明网关/API 有额外损耗
- 如果提升有限，瓶颈更像 Redis admission 或同进程资源竞争

- [ ] **Step 2: 实验 B：聚合 `PerfCreateOrder` 阶段指标**

目标：
- 直接看 token verify / Redis admission / async dispatch 三段耗时占比

预期解释：
- 如果 `redisAdmitMs` 明显占优，瓶颈更像 Redis Lua admission
- 如果三段都不高，再回头看网关/API 链路与调度开销

- [ ] **Step 3: 实验 C：盯 Redis**

同时采集：
- `INFO commandstats`
- `SLOWLOG GET`
- Redis CPU
- `EVAL/EVALSHA` 次数与耗时

预期解释：
- 如果 Redis 很忙而 order-rpc CPU 不高，瓶颈基本落在 Redis

- [ ] **Step 4: 实验 D：最后才考虑多实例 order-rpc**

前提：
- 已证明瓶颈不在 Redis
- 已证明单实例 order-rpc CPU/调度是约束

注意：
- 多实例必须配置不同 `Xid.NodeId`
- 如果目标是提升 consumer 吞吐，还要同步扩 Kafka partitions

---

### Task 8: 出最终结论文档

**Files:**
- Create: `docs/perf/2026-04-19-rush-rpc-vs-http-summary.md`

- [ ] **Step 1: 统一表格**

至少给四行：
- HTTP `/order/create`
- gRPC `PerfCreateOrder`
- 若有，再加 `多实例 order-rpc`

列统一为：
- `QPS`
- `avg`
- `p95`
- `p99`
- `create successRate`
- `Redis CPU`
- `order-rpc CPU`

- [ ] **Step 2: 明确最终判定**

结论必须落到三选一：
- 主要损耗在 `gateway/api`
- 主要损耗在 `order-rpc` 进程资源
- 主要损耗在 `Redis admission`

- [ ] **Step 3: 给后续动作建议**

根据结论分别给建议：
- 如果是 `gateway/api`：优先直压 RPC 或优化 HTTP 转发链路
- 如果是 `order-rpc`：再评估多实例 + 不同 `NodeId`
- 如果是 `Redis`：优先优化 Lua/key 设计、连接、慢日志，而不是盲目扩 `order-rpc`
