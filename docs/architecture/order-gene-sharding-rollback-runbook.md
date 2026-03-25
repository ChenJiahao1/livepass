# Order Gene Sharding Rollback Runbook

## 适用场景

- 切新读后发现某个槽位详情或列表不一致
- 新分片数据补齐滞后，需要立即恢复旧读
- `order-close`、支付、取消、退款链路在目标槽位出现回归

## 回滚原则

- 只做槽位级回滚，不做全量无脑回滚
- 先恢复读正确，再处理写模式
- 保留 `d_order_route_legacy`，不要在回滚期删除目录表

## 回滚步骤

### 1. 锁定问题槽位

记录以下信息：

- `logic_slot`
- `route_map` 当前 `Version`
- 相关 `DBKey` / `TableSuffix`
- 问题订单号样本

### 2. 执行槽位回滚

在 `jobs/order-migrate/etc/order-migrate.yaml` 中把 `Rollback.Slots` 设为目标槽位，然后执行：

```bash
go run ./jobs/order-migrate -f jobs/order-migrate/etc/order-migrate.yaml -action rollback
```

当前回滚会把目标槽位写回 `RouteMap.File`：

- `Status=rollback`
- `WriteMode=legacy_primary`

### 3. 重新加载服务配置

发布或 reload `services/order-rpc`，让新的 `route_map` 生效。当前 `dual_write` 读路径已经支持按槽位状态覆盖全局读模式：

- `primary_new` 槽位优先读新
- `rollback` 槽位强制回旧

### 4. 验证旧读恢复

```bash
go test ./services/order-rpc/repository -run 'TestDualWriteOrderRepositoryReadsLegacyWhenRouteStatusRollback' -count=1
bash scripts/acceptance/order_gene_sharding_migration.sh
```

## 回滚后的收敛动作

1. 保持双写，不要立刻删新分片数据。
2. 重新执行：

```bash
go run ./jobs/order-migrate -f jobs/order-migrate/etc/order-migrate.yaml -action verify
```

3. 修复问题后，再次按槽位执行 `switch`。

## 禁止事项

- 不要直接删分片表数据代替回滚
- 不要跳过 `route_map`，在业务代码里硬编码读旧表
- 不要全量把 `Sharding.Mode` 一次性切回 `legacy_only`，除非所有槽位都确认受影响
