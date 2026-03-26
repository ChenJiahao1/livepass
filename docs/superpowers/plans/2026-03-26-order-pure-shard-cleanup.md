# Order Pure Shard Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收敛订单域到纯新系统分片路径，删除旧格式订单兼容、目录表路由和迁移作业。

**Architecture:** 保留按 `user_id/order_number` 直达分片的分片路由，删除 `legacy_only`、双写、目录查找和旧单回填逻辑。`order-rpc` 仓储与配置收敛为单一分片实现，文档同步改成“无存量迁移”口径。

**Tech Stack:** Go, go-zero, MySQL, YAML config, Go test

---

### Task 1: 锁定新语义测试

**Files:**
- Modify: `services/order-rpc/sharding/order_number_codec_test.go`
- Modify: `services/order-rpc/tests/config/config_test.go`
- Modify: `services/order-rpc/tests/integration/query_order_logic_test.go`
- Modify: `services/order-rpc/repository/order_repository_test.go`

- [ ] **Step 1: 写失败测试**
- [ ] **Step 2: 运行相关测试，确认因 legacy 语义失效而失败**
- [ ] **Step 3: 更新断言，改成纯分片/非法旧单语义**
- [ ] **Step 4: 重新运行相关测试，确认通过**

### Task 2: 删除 legacy 兼容实现

**Files:**
- Modify: `services/order-rpc/sharding/order_number_codec.go`
- Modify: `services/order-rpc/sharding/migration_mode.go`
- Modify: `services/order-rpc/sharding/route_map.go`
- Modify: `services/order-rpc/sharding/route_map_test.go`
- Modify: `services/order-rpc/repository/order_repository.go`
- Delete: `services/order-rpc/repository/order_repository_legacy.go`
- Delete: `services/order-rpc/repository/order_repository_dual_write.go`
- Modify: `services/order-rpc/repository/order_repository_sharded.go`
- Modify: `services/order-rpc/repository/order_transaction.go`
- Delete: `services/order-rpc/internal/model/d_order_route_legacy_model.go`
- Delete: `services/order-rpc/internal/model/d_order_route_legacy_model_gen.go`
- Modify: `services/order-rpc/internal/svc/service_context.go`
- Modify: `services/order-rpc/internal/config/config.go`

- [ ] **Step 1: 删除目录表与双写接口依赖**
- [ ] **Step 2: 收敛仓储和服务上下文到纯分片实现**
- [ ] **Step 3: 收敛迁移模式、路由状态和配置默认值**
- [ ] **Step 4: 运行仓储/分片单测确认通过**

### Task 3: 删除迁移资产和遗留测试入口

**Files:**
- Delete: `jobs/order-migrate/`
- Delete: `sql/order/d_order_route_legacy.sql`
- Modify: `scripts/acceptance/order_gene_sharding_migration.sh`
- Modify: `README.md`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go`
- Modify: `services/order-rpc/tests/integration/service_context_kafka_test.go`

- [ ] **Step 1: 删除 order-migrate 作业与目录表 DDL**
- [ ] **Step 2: 清理测试 helper 和脚本里对 legacy 资源的依赖**
- [ ] **Step 3: 运行配置/集成测试确认没有残留引用**

### Task 4: 更新设计文档口径

**Files:**
- Modify: `docs/superpowers/specs/2026-03-24-order-gene-sharding-design.md`
- Modify: `docs/architecture/order-gene-sharding-runbook.md`
- Delete: `docs/architecture/order-gene-sharding-rollback-runbook.md`

- [ ] **Step 1: 删除历史订单迁移与目录表叙述**
- [ ] **Step 2: 改成纯新系统分片设计与运行说明**
- [ ] **Step 3: 检查文档不再承诺 legacy 兼容**

### Task 5: 全量验证

**Files:**
- Test: `services/order-rpc/sharding/...`
- Test: `services/order-rpc/repository/...`
- Test: `services/order-rpc/tests/config/...`
- Test: `services/order-rpc/tests/integration/...`

- [ ] **Step 1: 运行分片与仓储测试**
- [ ] **Step 2: 运行配置与关键集成测试**
- [ ] **Step 3: 汇总剩余风险与未覆盖面**
