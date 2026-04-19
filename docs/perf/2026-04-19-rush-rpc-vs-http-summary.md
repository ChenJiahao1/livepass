# 2026-04-19 Rush CreateOrder：HTTP vs gRPC PerfCreateOrder

## 实验口径

- `showTimeId=30001`
- `ticketCategoryId=40001`
- `USER_COUNT=2000`
- `SEAT_COUNT=2000`
- `MIN_TICKET_COUNT=1`
- `MAX_TICKET_COUNT=1`
- `VUS=2000`
- `ITERATIONS=2000`

## 结果文件

- HTTP 基线：`tmp/perf/results/rush-2k-20260419124351`
- gRPC `PerfCreateOrder` 直压：`tmp/perf/results/rush-rpc-2000-20260419132812`

## 口径说明

- `avg / p95 / p99` 来自 k6 `create_order_duration`
- HTTP 的 `QPS` 取 `gateway_qps.json.qpsByResponseWindow`
- gRPC 的“服务端窗口 QPS”取 `order_rpc_qps.json.qpsByResponseWindow`
- gRPC 的“客户端总时长 QPS”取 `client_qps.json.qpsByClientElapsed`
- gRPC 阶段耗时来自 `PerfCreateOrder` 响应，经 k6 聚合写入 `summary.json`
- gRPC 结果目录里的 `gateway_qps.json` 为占位文件，固定为 0，因为该实验绕过了 gateway
- gRPC `PerfCreateOrder` 直压额外保存 `final_state_immediate.json`，表示 RPC 返回完后立刻采集到的异步中间态
- gRPC `PerfCreateOrder` 直压的 `final_state.json` 表示等待异步消费收敛后重新采集到的最终态

## 汇总表

| 场景 | QPS | QPS 口径 | avg(ms) | p95(ms) | p99(ms) | create successRate | Redis/持久化最终状态 |
| --- | ---: | --- | ---: | ---: | ---: | ---: | --- |
| HTTP `/order/create` | 596.30 | gateway 响应窗口 | 1069.82 | 2028.09 | 2468.63 | 1.00 | `seatFrozen=2000`，下游已基本铺满 |
| gRPC `PerfCreateOrder` | 5347.59 | order-rpc 阶段日志窗口 | 217.02 | 403.00 | 419.03 | 1.00 | 最终 `seatFrozen=2000`，`orderRows=2000` |

## 补充口径

| 场景 | 客户端总时长 QPS |
| --- | ---: |
| HTTP 基线 | 116.80 (`2000 / 17.1232s`) |
| gRPC `PerfCreateOrder` | 160.55 |

## 结论

### 1. `gateway/api` 开销非常明显

HTTP 基线的 `avg/p95/p99` 明显高于直压 gRPC：

- HTTP `avg=1069.82ms`
- gRPC `avg=217.02ms`
- HTTP `p95=2028.09ms`
- gRPC `p95=403.00ms`

这说明在当前 2k 口径下，`gateway-api -> order-api -> order-rpc` 这条 HTTP 访问链路引入了远高于 `order-rpc` 下单同步热路径本身的同步开销。

### 2. gRPC 直压仍需继续拆阶段耗时

当前 `PerfCreateOrder` 已经明显快于 HTTP，但还不能仅凭这轮对比把 RPC 内部瓶颈完全定死。下一步仍应围绕阶段指标继续拆：

- `purchaseTokenVerifyMs`
- `redisAdmitMs`
- `asyncDispatchScheduleMs`

### 3. `Redis admission` 仍是 RPC 热路径里的核心同步段

当前同步路径已经可以直接从 `summary.json` 聚合以下阶段指标：

- `purchaseTokenVerifyMs`
- `redisAdmitMs`
- `asyncDispatchScheduleMs`

结合“直压 gRPC PerfCreateOrder 显著优于 HTTP”的结果，更合理的下一步是继续盯：

- `gateway/api` 链路开销
- `PerfCreateOrder` 汇总里的 `redisAdmitMs`

而不是先扩 `order-rpc` 实例。

## 当前判断

如果必须现在先给三选一结论，当前证据最支持：

> **主要损耗仍在 `gateway/api`；RPC 内部还需要继续拆 `Redis admission` 的同步耗时影响。**

## 已落地改动

- 新增 gRPC 压测脚本：`tests/perf/rush_create_order_rpc.js`
- 新增 gRPC 数据映射：`tests/perf/lib/grpc_dataset.js`
- 新增专用压测 RPC：`order.OrderRpc/PerfCreateOrder`
- 新增实验脚本：
  - `scripts/perf/run_rush_rpc_perf.sh`
  - `scripts/perf/analyze_create_order_path.sh`
- `PerfCreateOrder` 响应与汇总会带出阶段指标：
  - `purchaseTokenVerifyMs`
  - `redisAdmitMs`
  - `asyncDispatchScheduleMs`
- gRPC `PerfCreateOrder` 直压脚本现在会先保存 `final_state_immediate.json`，再等待最终状态收敛并刷新 `final_state.json`

## 下一步建议

1. 直接读取 `summary.json` 里的阶段指标聚合
2. 对比 `redisAdmitMs` 与 `purchaseTokenVerifyMs` 的 `avg/p95/p99`
3. 如阶段指标显示 Redis 明显占优，再采 Redis `INFO commandstats` / `SLOWLOG`
4. 在确认不是 Redis 之前，不建议先做 `order-rpc` 多实例扩容
