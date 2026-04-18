# 2026-04-17 rush create order 2k 数据汇报

## 一、结论摘要

- 压测目标：`showTimeId=30001`、`ticketCategoryId=40001`、`VUS=2000`、`ITERATIONS=2000`
- 本轮已将 `purchaseToken TTL` 与 `JWT TokenExpire` 均调整为 `24h`
- 本轮不按支付态统计，统一按“**冻结即成功占座**”口径汇报
- 最新一轮 `k6` 结果：
  - `createSuccessCount = 1014`
  - `inventoryInsufficientCount = 986`
  - `successRate = 50.7%`
  - `pollSuccessCount = 609`
  - `pollSuccessRate = 60.06%`
  - `create avg = 138.00ms`
  - `create p95 = 210.40ms`
  - `create p99 = 304.93ms`
- 最新一轮冻结事实：
  - 冻结订单 `895` 单
  - 冻结座位 `1768` 座
  - `d_seat.seat_status=2 = 1768`
  - `d_order_ticket_user_* = 1768`
  - `d_order_seat_guard = 1768`
- 最新一轮 `/order/create` 日志窗口重算 QPS：`1928.640/s`

## 二、配置变更

### 1. TTL 调整

- `purchaseToken` TTL 改为 `24h`
  - `services/order-rpc/etc/order.yaml`
  - `services/order-rpc/etc/order.perf.yaml`
- JWT `TokenExpire` 改为 `24h`
  - `services/user-rpc/etc/user.yaml`

### 2. 配置验证

- 配置测试：`go test ./services/order-rpc/tests/config ./services/user-rpc/tests/config`
- JWT 实测：`exp - iat = 86400s`
- `purchaseToken` 抽样解码：`ttlHoursApprox = 23.99`

## 三、压测对象与口径

### 1. 压测对象

- 同场次 + 同票档 + 从 `create order` 开始 + 超卖竞争型
- `showTimeId=30001`
- `ticketCategoryId=40001`

### 2. 统计口径

- `create` 指 `POST /order/create` 热路径返回结果
- `poll` 指 `POST /order/poll` 在脚本超时窗口内收敛的结果
- 最终业务成功口径采用“**冻结即成功占座**”
- 本文**不采用支付成功口径**

## 四、数据准备

### 1. 最新一轮数据集

- 数据集目录：`tmp/perf/rush-30001-40001-2000-20260417233113`
- 用户数据：`tmp/perf/rush-30001-40001-2000-20260417233113/users.json`
- 元数据：`tmp/perf/rush-30001-40001-2000-20260417233113/meta.json`

### 2. 准备动作

- 清理 `show_time_id=30001` 订单域污染数据
- 重建 `2000` 用户 / `2000` 库存
- 预热 rush runtime 与 seat ledger
- 批量签发 `2000` 个 `purchaseToken`

## 五、k6 指标结果

### 1. 执行命令

```bash
DATASET_PATH=/home/chenjiahao/code/project/livepass/tmp/perf/rush-30001-40001-2000-20260417233113/users.json \
BASE_URL=http://127.0.0.1:8081 \
PERF_SECRET=livepass-perf-secret-0001 \
k6 run --summary-trend-stats 'avg,min,med,max,p(90),p(95),p(99)' \
  --vus 2000 --iterations 2000 tests/perf/rush_create_order.js
```

### 2. 汇总指标

| 指标 | 数值 |
|---|---:|
| datasetSize | 2000 |
| createSuccessCount | 1014 |
| inventoryInsufficientCount | 986 |
| businessFailureCount | 0 |
| create successRate | 50.7% |
| pollSuccessCount | 609 |
| pollSuccessRate | 60.06% |
| create avg | 138.00ms |
| create p95 | 210.40ms |
| create p99 | 304.93ms |

### 3. 结果说明

- 本轮 `create` 成功与库存不足接近 `1:1`
- `pollSuccessRate` 明显低于 `createSuccessRate`，说明异步建单在脚本轮询窗口内未完全收敛
- 因此最终业务成功量应以下面的“冻结事实”统计为准

## 六、冻结口径一致性

### 1. 时间窗

为隔离上一轮迟到消费，本轮按最新一轮 `create` 后的订单时间窗过滤：

- 过滤条件：`create_order_time >= '2026-04-17 23:31:57'`

### 2. 订单域事实

| 指标 | 数值 |
|---|---:|
| recent_order_rows | 895 |
| recent_ticket_sum | 1768 |
| recent_ticket_rows | 1768 |
| recent_seat_guard_rows | 1768 |
| recent_user_guard_rows | 895 |
| recent_viewer_guard_rows | 1768 |

### 3. 节目域事实

| 指标 | 数值 |
|---|---:|
| seat_frozen | 1768 |
| seat_sold | 0 |
| seat_available | 232 |

### 4. 一致性结论

- `seat_frozen = 1768`
- `recent_ticket_rows = 1768`
- `recent_seat_guard_rows = 1768`
- 三者一致，说明按“冻结即成功占座”口径，本轮一致性成立
- 最终可汇报的业务成功量为：
  - `895` 单
  - `1768` 座

## 七、/order/create 响应窗口与 QPS

### 1. 日志来源

- 日志文件：`.codex/runlogs/gateway-api.perf.log`

### 2. 最新一轮窗口

| 指标 | 数值 |
|---|---:|
| first response | `2026-04-17T23:32:08.118+08:00` |
| last response | `2026-04-17T23:32:09.155+08:00` |
| window_seconds | `1.037` |
| response_count | `2000` |
| HTTP 200 | `1014` |
| HTTP 400 | `986` |
| create QPS | `1928.640/s` |

### 3. 说明

- 本 QPS 按网关日志中的 `/order/create` 响应窗口重算
- 该值反映的是最新一轮压测的实际响应吞吐

## 八、已知现象

- `scripts/perf/verify_rush_perf_result.sh` 仍按“已售态”口径校验，当前不适用于本轮汇报口径
- 因当前汇报口径为“冻结即成功占座”，所以应以：
  - `d_seat.seat_status=2`
  - `d_order_ticket_user_*`
  - `d_order_seat_guard`
  为准

## 九、最终汇报口径

建议对外或对内汇报时直接使用以下数据：

- 压测模型：`showTimeId=30001` / `ticketCategoryId=40001` / `VUS=2000` / `ITERATIONS=2000`
- TTL 修正：`purchaseToken=24h`，`JWT=24h`
- 热路径结果：
  - `/order/create` 成功 `1014`
  - `/order/create` 库存不足 `986`
- 热路径性能：
  - `avg 138.00ms`
  - `p95 210.40ms`
  - `p99 304.93ms`
  - `create QPS 1928.640/s`
- 业务成功口径（冻结即成功占座）：
  - 成功冻结 `895` 单
  - 成功冻结 `1768` 座
