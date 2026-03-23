# Order Timeout And MySQL Pool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 显式化订单创建链路的 timeout 配置，并把 MySQL 连接池参数纳入项目配置与服务初始化。

**Architecture:** 保持 go-zero 默认服务组织不变，只在配置层和 `pkg/xmysql` 增加显式参数入口。各 RPC 服务继续使用 `sqlx`，通过 `RawDB()` 统一应用连接池参数，避免服务层散落默认值。

**Tech Stack:** Go, go-zero, gRPC, `database/sql`, MySQL

---

### Task 1: 补配置测试

**Files:**
- Modify: `services/order-api/tests/config/config_test.go`
- Create: `services/order-rpc/tests/config/config_test.go`
- Modify: `services/gateway-api/tests/config/config_test.go`

- [ ] 为 `order-api` 配置测试补上显式 timeout 断言。
- [ ] 新增 `order-rpc` 配置测试，覆盖 server/client timeout 与 MySQL pool 字段加载。
- [ ] 如需调整 gateway 的 order upstream timeout，同步补测试断言。

### Task 2: 补 xmysql 红灯测试

**Files:**
- Modify: `pkg/xmysql/mysql_test.go`
- Modify: `services/order-rpc/tests/integration/service_context_redis_test.go`

- [ ] 为 `xmysql.Config` 默认池参数与归一化逻辑写失败测试。
- [ ] 为 `order-rpc` service context 增加连接池参数生效断言。

### Task 3: 实现配置与连接池落地

**Files:**
- Modify: `pkg/xmysql/mysql.go`
- Modify: `services/order-api/etc/order-api.yaml`
- Modify: `services/order-api/etc/order-api.perf.yaml`
- Modify: `services/order-rpc/etc/order-rpc.yaml`
- Modify: `services/order-rpc/etc/order-rpc.perf.yaml`
- Modify: `services/gateway-api/etc/gateway-api.yaml`
- Modify: `services/program-rpc/internal/svc/service_context.go`
- Modify: `services/user-rpc/internal/svc/service_context.go`
- Modify: `services/pay-rpc/internal/svc/service_context.go`
- Modify: `services/order-rpc/internal/svc/service_context.go`

- [ ] 在 `pkg/xmysql` 增加池参数默认值与 `ApplyPool` 能力。
- [ ] 在订单链路配置中显式声明 timeout。
- [ ] 在各 RPC 服务的 MySQL 初始化处统一应用连接池参数。

### Task 4: 运行验证

**Files:**
- Modify: `go.work.sum` if needed by test execution

- [ ] 运行 `go test` 目标测试集，确认红绿转换完成。
- [ ] 如无异常，再运行相关服务包构建验证配置与代码编译通过。
