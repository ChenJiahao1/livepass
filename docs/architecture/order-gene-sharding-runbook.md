# Order Gene Sharding Runbook

## 目标

把订单域从 `legacy_only` 平滑推进到“可双写、可回填、可校验、可按槽位切新读”的状态，并保持默认发布模式仍可回退。

## 前置条件

- 已执行分片 DDL：
  - `sql/order/d_user_order_index.sql`
  - `sql/order/d_order_route_legacy.sql`
  - `sql/order/sharding/d_order_shards.sql`
  - `sql/order/sharding/d_order_ticket_user_shards.sql`
  - `sql/order/sharding/d_user_order_index_shards.sql`
- `services/order-rpc/etc/order-rpc.yaml` 或目标环境配置已包含：
  - `Sharding.Mode`
  - `Sharding.LegacyMySQL`
  - `Sharding.Shards`
  - `Sharding.RouteMap`
- `jobs/order-close/etc/order-close.yaml` 已配置槽位扫描窗口：
  - 旧表默认使用 `ScanSlotStart=0`、`ScanSlotEnd=0`
  - 新分片扫描建议使用 `ScanSlotStart=0`、`ScanSlotEnd=1023`
- `jobs/order-migrate/etc/order-migrate.yaml` 已配置：
  - `LegacyMySQL`
  - `Shards`
  - `RouteMap.File`
  - `Backfill.CheckpointFile`

## 阶段 1：初始化路由与分片表

1. 生成或准备一份独立的 `route_map` 文件，至少包含 `Version` 和所有 `Entries`。
2. 确认每个 `logic_slot` 都能映射到唯一 `DBKey + TableSuffix`。
3. 在联调环境先执行：

```bash
go test ./services/order-rpc/sharding ./services/order-rpc/repository -count=1
```

## 阶段 2：开启影子双写

1. 把 `services/order-rpc` 的 `Sharding.Mode` 切到 `dual_write_shadow`。
2. 保持 `route_map` 初始状态为：
   - `Status=shadow_write`
   - `WriteMode=dual_write`
3. 验证新订单能同时落旧表和新分片表：

```bash
go test ./services/order-rpc/tests/integration -run 'TestCreateOrderConsumerPersistsShadowTablesInDualWriteShadowMode' -count=1
```

## 阶段 3：执行回填

1. 配置 `jobs/order-migrate/etc/order-migrate.yaml`：
   - `Backfill.BatchSize`
   - `Backfill.CheckpointFile`
   - `RouteMap.File`
2. 执行回填：

```bash
go run ./jobs/order-migrate -f jobs/order-migrate/etc/order-migrate.yaml -action backfill
```

3. 回填期要检查：
   - `d_order_xx`
   - `d_order_ticket_user_xx`
   - `d_user_order_index_xx`
   - `d_order_route_legacy`
4. 断点续跑依赖 `CheckpointFile`，重复执行同一命令即可继续。

## 阶段 4：执行校验

1. 配置 `Verify.Slots` 为本轮要校验的槽位集合。
2. 执行：

```bash
go run ./jobs/order-migrate -f jobs/order-migrate/etc/order-migrate.yaml -action verify
```

3. 当前校验至少覆盖：
   - 行数
   - 金额总和
   - 状态分布
   - 详情抽样
   - 用户列表抽样

## 阶段 5：按槽位切新读

1. 仅对已通过校验的槽位执行：

```bash
go run ./jobs/order-migrate -f jobs/order-migrate/etc/order-migrate.yaml -action switch
```

2. `switch` 会把目标槽位写回 `RouteMap.File`：
   - `Status=primary_new`
   - `WriteMode=dual_write`
3. 发布或 reload `services/order-rpc` 配置，让新 `route_map` 生效。
4. 验证切新后读一致：

```bash
bash scripts/acceptance/order_gene_sharding_migration.sh
```

## 阶段 6：收敛为 shard_only

1. 当所有槽位都完成切新读且观察窗口稳定后，再把 `Sharding.Mode` 从双写模式切到 `shard_only`。
2. 切到 `shard_only` 前，必须确认：
   - `jobs/order-migrate/tests/integration` 全绿
   - `jobs/order-close` 扫描窗口已覆盖 `0..1023`
   - `d_order_route_legacy` 仍保留

## 日常验收命令

```bash
bash scripts/acceptance/order_gene_sharding_smoke.sh
bash scripts/acceptance/order_gene_sharding_migration.sh
```

## 备注

- 当前仓库里 `docs/api/order-checkout-acceptance.md`、`docs/api/order-checkout-failure-acceptance.md`、`docs/api/order-refund-acceptance.md` 存在外部删除态；本 runbook 不依赖这些文件。
- 默认发布模式建议仍保持 `legacy_only`，只有在影子双写、回填、校验和切新读都完成后才提升模式。
