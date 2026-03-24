# 2026-03-23 Order 抢单链路压测与调优记录

## 1. 背景与目标

本文记录 2026-03-23 围绕 `order/create` 链路完成的一轮压测、扩容、定位与复盘过程，重点回答 4 个问题：

1. 当前抢单链路在高并发下的真实瓶颈在哪里。
2. `order-rpc` 抢占分布式锁是否把链路退化成了全局串行。
3. 扩实例后瓶颈是否发生转移。
4. 压测完成后，订单、座位冻结、Redis 库存是否一致。

本轮核心压测对象：

- 节目：`programId=10001`
- 票档：`ticketCategoryId=40001`
- 入口链路：`gateway-api/order-api -> order-rpc -> program-rpc + user-rpc -> Kafka consumer -> MySQL/Redis`
- 核心脚本：`.tmp/perf/order_create_gateway_multi_user_three_rounds_exact.js`
- 核心用户池：
  - `.tmp/perf/order_user_pool.220users.json`
  - `.tmp/perf/order_user_pool.1000users.fresh.json`

说明：

- `k6` 脚本里每个成功请求会通过 2 个检查项，所以 `成功请求数 = checks_pass / 2`。
- 本轮后半段的主场景是 `1000 用户、每用户 3 次请求、1000 库存、不限购`。
- 本轮观测到的限购配置为 `0/0/1000`，即单次限购和单账号限购未生效，但总量是 1000。

## 2. 测试时间线

### 2.1 阶段 A：基线确认，先看 100 库存是否稳

| 阶段 | 场景 | 结果 | 结论 |
| --- | --- | --- | --- |
| A1 | `220` 用户池，`200` 请求，库存 `100` | 成功 `100`，失败 `100`，`p95=1709ms` | 没有超卖；失败是正常的库存不足 |
| A2 | `1000` 用户精确 `1000` 请求，库存 `100` | 成功 `100`，失败 `900`，`p95=3495ms` | 仍然没有超卖；失败全部是 `seat inventory insufficient` |

这一阶段的作用不是追求成功率，而是确认在库存明确不足时系统不会超卖。结果基本符合预期。

### 2.2 阶段 B：清场后切到 1000 库存、不限购，观察真实并发瓶颈

清场并恢复库存后，开始主场景压测：

- `1000` 用户
- 每用户 `3` 次请求
- 总请求 `3000`
- 库存 `1000`
- 不限购

默认实例数下连续两次复测结果：

| 阶段 | k6 结果文件 | 成功请求数 | 成功率 | `p95` | 主要问题 |
| --- | --- | --- | --- | --- | --- |
| B1 | `.tmp/perf/k6_retest_20260323_182647_pool1000_rounds3_inventory1000_unlimited.json` | `357` | `11.90%` | `4384ms` | `program.rpc AutoAssignAndFreezeSeats context deadline exceeded` |
| B2 | `.tmp/perf/k6_retest_20260323_183027_pool1000_rounds3_inventory1000_unlimited.json` | `346` | `11.53%` | `4433ms` | 同上，同时出现 `order limit ledger not ready` 噪音 |

到这里可以确认一件事：库存已经不是主问题，主瓶颈已经转移到下游链路，且表现为明显的 `4s` 级超时。

### 2.3 阶段 C：只扩 `order-rpc` 到 10 实例，验证“分布式锁导致串行”的怀疑

场景不变，只把 `order-rpc` 扩到 `10` 实例。

| 阶段 | k6 结果文件 | 成功请求数 | 成功率 | `p95` | 结论 |
| --- | --- | --- | --- | --- | --- |
| C1 | `.tmp/perf/k6_retest_20260323_192803_orderrpc10_pool1000_rounds3_inventory1000_unlimited.json` | `335` | `11.17%` | `4450ms` | 新实例有流量，但整体成功数几乎没提升 |

这一步非常关键，因为它直接推翻了“`order-rpc` 的分布式锁把系统退化成全局串行”的直觉判断。

### 2.4 阶段 D：把 `order-rpc`、`program-rpc`、`user-rpc` 都扩到 10 实例

继续保持主场景，只把三类 RPC 都扩到 `10` 实例。

| 阶段 | k6 结果文件 | 成功请求数 | 成功率 | `p90` | `p95` | 平均耗时 | 结论 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| D1 | `.tmp/perf/k6_retest_20260323_194716_allrpc10_pool1000_rounds3_inventory1000_unlimited.json` | `428` | `14.26%` | `1087ms` | `1165ms` | `244ms` | `4s` 超时瓶颈明显缓解，但数据库连接数成为新瓶颈 |

和阶段 C 相比：

- 成功请求数从 `335` 提升到 `428`
- `p95` 从约 `4.45s` 降到约 `1.16s`

这说明前一阶段的真正瓶颈并不在 `order-rpc` 本身，而是在 `program-rpc` / `user-rpc` 的下游处理能力上。

## 3. 过程中发现的问题、阻塞点，以及处理方式

### 3.1 阻塞点一：1000 用户池 JWT 已过期

现象：

- 新一轮压测前，原始 `1000` 用户池已不再可靠，继续使用会引入认证噪音。

处理：

- 按现有渠道密钥重新签发一份用户池：
- `.tmp/perf/order_user_pool.1000users.fresh.json`

结论：

- 这一步解决的是测试环境阻塞，不是业务瓶颈，但如果不先处理，后续所有压测结果都会混入认证层噪音。

### 3.2 阻塞点二：误以为 `order-rpc` 分布式锁导致全局串行

最初怀疑点：

- `order` 创建订单前要抢分布式锁，看起来像是整个链路被锁串行了。

代码证据：

- `services/order-rpc/internal/logic/create_order_logic.go`
- `services/order-rpc/internal/repeatguard/guard.go`

关键事实：

- 锁键是 `create_order:{userId}:{programId}`
- 这不是全局锁，只会限制“同一个用户对同一个节目”的重复提交

处理：

- 把 `order-rpc` 单独扩到 `10` 实例做验证

结果：

- 新实例确实吃到了流量
- 但成功数没有显著提升，`p95` 仍然卡在 `4.4s` 左右

结论：

- `order-rpc` 的重复提交保护不是这轮压测的主瓶颈
- 真正的串行化/拥塞点在更下游

### 3.3 阻塞点三：`program-rpc AutoAssignAndFreezeSeats` 成为第一主瓶颈

现象：

- 默认实例数时，大量请求在 `AutoAssignAndFreezeSeats` 上 `context deadline exceeded`
- `order-rpc` 的 `CreateOrder` 端到端慢调用集中在 `4.1s ~ 4.3s`

证据：

- `.tmp/perf/order-rpc-scale-10/order-rpc.scale8.log`
- `.tmp/perf/order-rpc.log`

处理：

- 把 `program-rpc`、`user-rpc` 一并扩到 `10` 实例

结果：

- `p95` 从 `4.45s` 降到 `1.16s`
- 说明首个主瓶颈确实在 `program-rpc` / `user-rpc` 下游链路，而不是 HTTP 入口，也不是 `order-rpc` 的锁本身

### 3.4 阻塞点四：扩容后瓶颈转移到 MySQL 连接数

现象：

- 三类 RPC 都扩到 `10` 实例后，超时问题显著缓解，但开始出现大量 `Too many connections`
- `program-rpc`、`user-rpc`、`order-rpc consumer`、HTTP 入口都开始连锁失败

现场证据：

- MySQL `max_connections = 151`
- 扩容配置中的连接池上限：
  - `order-rpc MaxOpenConns = 128`
  - `program-rpc MaxOpenConns = 64`
  - `user-rpc MaxOpenConns = 64`
- 仅这三类 RPC 在 `10` 实例下的理论连接上限就已经远大于 `151`

相关文件：

- `.tmp/perf/order-rpc-scale-10/order-rpc.perf.scale1.yaml`
- `.tmp/perf/rpc-scale-10/configs/program-rpc.scale1.yaml`
- `.tmp/perf/rpc-scale-10/configs/user-rpc.scale1.yaml`
- `.tmp/perf/program-rpc.log`
- `.tmp/perf/user-rpc.log`
- `.tmp/perf/order-rpc.log`
- `.tmp/perf/order-api.log`
- `.tmp/perf/gateway-api.log`

具体表现：

- `program-rpc` / `user-rpc` 查询报 `Error 1040: Too many connections`
- `order-rpc` 内部调用 `GetProgramPreorder` / `GetUserAndTicketUserList` 直接失败
- `order create consumer worker=3 exited: Error 1040: Too many connections, restarting in 1s`
- `order-api` / `gateway-api` 出现大量 `[http] dropped` 和 `503`

结论：

- 扩容不是没效果，而是把瓶颈从 RPC 超时成功推到了数据库连接数上限
- 这属于“上一层优化成功，下一层资源上限暴露”的典型现象

### 3.5 阻塞点五：日志里混入 `order limit ledger not ready`，掩盖主因

现象：

- 压测过程中持续出现 `order limit ledger not ready`

代码位置：

- `services/order-rpc/internal/logic/create_order_logic.go`
- `services/order-rpc/internal/logic/order_create_compensation.go`

原因：

- 当前限购配置未启用单账号限购
- 但失败补偿路径仍然会无条件调用 purchase-limit release
- 没有预留 ledger 时就会打出 `order limit ledger not ready`

影响：

- 这不是当前主因
- 但它会制造大量错误日志噪音，干扰对真正瓶颈的判断

### 3.6 过程中的取数阻塞：本机没有 `mysql` / `redis-cli`，Redis 键名也容易读错

现象：

- 宿主机没有安装 `mysql` 和 `redis-cli`
- 一开始还把 Redis 的 `available_count` 当成独立 key 去读，结果拿到空值

处理：

- 改用 `docker exec docker-compose-mysql-1 ...`
- 改用 `docker exec docker-compose-redis-1 ...`
- 回到代码确认 Redis 键结构：
  - 前缀是 `damai-go:`
  - 可售库存存放在哈希 `damai-go:program:seat-ledger:stock:{programId}:{ticketCategoryId}` 的 `available_count` 字段

这不是业务问题，但属于本轮实际排障过程中遇到的工具链阻塞，值得记录。

## 4. 根因分析

### 4.1 `order-rpc` 的重复提交锁不是主因

锁粒度是 `userId + programId`，它只能防止单用户重复点击，不会把所有用户请求串成一个全局队列。

### 4.2 第一主瓶颈在 `program-rpc` 的锁座/冻结链路

在默认实例数下，主症状是 `AutoAssignAndFreezeSeats` `4s` 超时。只扩 `order-rpc` 无效，而把 `program-rpc`、`user-rpc` 一并扩容后延迟马上下降，说明真正的热点在锁座、冻结和其关联的下游资源上。

### 4.3 扩容后数据库连接数成为第二主瓶颈

当 RPC 层并发能力上去以后，MySQL `151` 的连接上限立刻被打满，后果是：

- RPC 查询失败
- consumer 重启
- breaker 打开
- HTTP 入口直接掉 `503`

也就是说，本轮扩容并没有“失败”，而是把瓶颈从上游推进到了数据库层。

### 4.4 当前架构下，“接口成功”不等于“订单已经落库”

从 `services/order-rpc/internal/logic/create_order_logic.go` 可以看出，`order-rpc` 在：

1. 获取节目预下单信息
2. 查询用户信息
3. 调用 `AutoAssignAndFreezeSeats`
4. 发送订单创建消息

之后就会直接返回 `orderNumber`。

这意味着：

- 如果 HTTP 返回成功，但 Kafka consumer 后续因为数据库连接数问题失败，那么用户已经拿到了成功响应，座位也已经冻结，但订单可能还没真正落库

这正是本轮后段最危险的现象。

## 5. 压测后的状态快照

### 5.1 压测结束时的核心判断

在三类 RPC 都扩到 `10` 实例后的主场景里：

- HTTP 成功请求数：`428`
- 实际订单落库数：`133`
- 活跃冻结记录数：`341`

这三个数字不在一个数量级，说明当前系统已经出现了明显的“冻结成功但订单未完成持久化”的不一致风险。

### 5.2 复查时重新取数的当前快照

以下数据是后续再次通过容器内 `mysql` / `redis-cli` 复查得到的结果：

MySQL：

- `max_connections = 151`
- `d_order(program_id=10001) = 133`
- `d_order_ticket_user = 133`
- `distinct seat_id = 133`
- `duplicate seat_id = 0`
- `Redis 冻结元数据(program_id=10001, ticket_category_id=40001, freeze_status=1) = 341`
- `d_seat(ticket_category_id=40001, seat_status=1) = 659`
- `d_seat(ticket_category_id=40001, seat_status=2) = 341`

Redis：

- `available_count = 659`
- `available zset = 659`
- `frozen keys = 341`
- `purchase-limit ledger keys = 27`

说明：

- Redis 这一层是时点数据，会随冻结超时、补偿、后台释放继续变化
- 本轮现场更早一次记录里，Redis 曾经短时间观测到 `572 / 428 / 8` 这样的值；后续复查时已经回落到 `659 / 341 / 27`
- 因此，后续复盘 Redis 相关数据时必须带时间点，不能把不同时间的快照直接混在一起比较

## 6. 当前结论

1. 只扩 `order-rpc` 没有意义，当前瓶颈不在 `order-rpc` 的重复提交锁。
2. 第一主瓶颈是 `program-rpc` 锁座/冻结链路，第二主瓶颈是 MySQL 连接数。
3. 当前最严重的风险不是“慢”，而是“HTTP 成功后订单未落库、冻结座位残留”，也就是一致性风险。
4. `order limit ledger not ready` 不是主因，但必须尽快降噪，否则后续很难快速定位真实故障。

## 7. 下一步建议

### 7.1 先处理数据库连接总量

优先级最高：

- 重新核算所有服务实例的 `MaxOpenConns`
- 不要让应用总连接池上限远高于 MySQL `max_connections`
- 如有必要，单独为 consumer 预留稳定连接池，避免和同步请求抢连接

### 7.2 增加“冻结成功但订单未落库”的一致性监控

至少补 4 类指标：

- `CreateOrder` 成功返回数
- Kafka 订单创建消息发送成功数
- consumer 实际落库成功数
- 活跃冻结数 / 孤儿冻结数

### 7.3 修正 purchase-limit 补偿逻辑

建议只在真正调用过 `Reserve` 成功后才执行 `Release`，避免继续刷 `order limit ledger not ready`。

### 7.4 后续压测建议

下一轮建议按下面顺序继续：

1. 控制住总连接数后，复测 `1000 用户 * 3 次请求`
2. 对比 `成功请求数 / 落库订单数 / 活跃冻结数`
3. 如果订单落库仍显著落后，再单独分析 Kafka consumer 和下游事务路径

## 8. 证据索引

压测结果：

- `.tmp/perf/k6_retest_20260323_182647_pool1000_rounds3_inventory1000_unlimited.json`
- `.tmp/perf/k6_retest_20260323_183027_pool1000_rounds3_inventory1000_unlimited.json`
- `.tmp/perf/k6_retest_20260323_192803_orderrpc10_pool1000_rounds3_inventory1000_unlimited.json`
- `.tmp/perf/k6_retest_20260323_194716_allrpc10_pool1000_rounds3_inventory1000_unlimited.json`
- `.tmp/perf/retest_20260323_175634_pool220_qps200_1s_summary.txt`
- `.tmp/perf/retest_20260323_181717_pool1000_exact1000_summary.txt`

关键日志：

- `.tmp/perf/order-rpc.log`
- `.tmp/perf/program-rpc.log`
- `.tmp/perf/user-rpc.log`
- `.tmp/perf/order-api.log`
- `.tmp/perf/gateway-api.log`
- `.tmp/perf/order-rpc-scale-10/order-rpc.scale8.log`

关键代码：

- `services/order-rpc/internal/logic/create_order_logic.go`
- `services/order-rpc/internal/repeatguard/guard.go`
- `services/order-rpc/internal/logic/order_create_compensation.go`
- `services/program-rpc/internal/seatcache/seat_stock_keys.go`

## 9. 2026-03-24 连接数调优后复测

### 9.1 本轮变更前提

相对 2026-03-23 记录中的环境，本轮至少有两项变化：

- MySQL `max_connections` 已从 `151` 调整到 `320`
- 原 `1000` 用户池 JWT 已于 `2026-03-23 21:46:36` 过期，因此本轮基于 `damai_user.d_ticket_user` 重新签发了新的压测用户池

本轮仍然使用和 2026-03-23 阶段 D 等价的主场景：

- `1000` 用户
- 每用户 `3` 次请求
- 总请求 `3000`
- 库存 `1000`
- 不限购
- `order-rpc`、`program-rpc`、`user-rpc` 均维持 `10` 实例拓扑

### 9.2 主场景结果

本轮 `k6` 结果文件：

- `.tmp/perf/k6_retest_20260324_105844_allrpc10_pool1000_rounds3_inventory1000_unlimited_after_conn_tuning.json`

核心结果：

- HTTP 总请求数：`3000`
- 成功请求数：`1000`
- 失败请求数：`2000`
- 成功率：`33.33%`
- 整体 `p90 = 1242ms`
- 整体 `p95 = 1299ms`
- 成功请求 `avg = 1177ms`
- 成功请求 `p90 = 1360ms`
- 成功请求 `p95 = 1487ms`

这次 `k6` 仍然以非零状态退出，但主因不是链路异常，而是脚本阈值仍保留了：

- `http_req_failed rate < 0.01`

在“`1000` 库存、`3000` 请求”的场景下，卖完后的 `2000` 次失败本来就会把这个阈值打穿，因此这次退出码不能直接解读为系统退化。

### 9.3 一致性快照

压测结束后立即复查：

MySQL：

- `max_connections = 320`
- `Max_used_connections = 321`
- `d_order(program_id=10001) = 1000`
- `d_order_ticket_user(ticket_category_id=40001) = 1000`
- `distinct seat_id = 1000`
- `duplicate seat_id = 0`
- `Redis 冻结元数据(program_id=10001, ticket_category_id=40001, freeze_status=1) = 1000`
- `d_seat(ticket_category_id=40001, seat_status=1) = 0`
- `d_seat(ticket_category_id=40001, seat_status=2) = 1000`
- `d_seat(ticket_category_id=40001, seat_status=3) = 0`
- `d_order.order_status=1 = 1000`

Redis：

- `available_count = 0`
- `available zset = 0`
- `frozen keys = 1000`
- `purchase-limit ledger keys = 0`

Kafka：

- `order.create.command.v1` 各分区 consumer lag 均为 `0`

结论：

- 本轮 `1000` 次 HTTP 成功全部完成落库，没有出现 2026-03-23 那种“成功返回远高于实际落库数”的明显不一致
- 座位冻结、订单快照、Redis 座位账本三者在压测结束时是一致的
- 当前的 `1000` 条冻结记录对应的是 `1000` 笔未支付订单，属于业务语义内冻结，不是孤儿冻结

### 9.4 剩余问题

虽然主结果已经明显改善，但数据库连接问题没有彻底消失。

现场证据：

- MySQL `Max_used_connections = 321`
- 关键日志中仍能检索到大量 `Error 1040: Too many connections`
- 未再出现 2026-03-23 默认实例数下那种大面积 `context deadline exceeded`

本轮抽样日志统计：

- `Too many connections`：`1635`
- `context deadline exceeded`：`0`
- `seat inventory insufficient`：`320`

这里要特别注意：

- `seat inventory insufficient` 只是卖完后的正常失败，不能当成故障
- `Too many connections` 虽然没有再把主结果打穿，但说明当前连接池总上限仍然偏大，数据库层依旧处在被打满的边缘

### 9.5 对 2026-03-23 结论的修正

和 2026-03-23 阶段 D 的 `428 / 133 / 341` 相比，这次结果已经发生根本变化：

- HTTP 成功请求数从 `428` 提升到 `1000`
- 实际订单落库数从 `133` 提升到 `1000`
- 活跃冻结数从 `341` 变成和订单数一致的 `1000`
- Kafka lag 最终回落到 `0`

因此可以确认：

1. 把 MySQL 连接上限从 `151` 提升到 `320` 后，2026-03-23 最突出的“HTTP 成功但订单未落库”风险在本轮主场景中已不再复现。
2. 但当前应用侧连接池总预算仍然超过数据库安全水位，因为 `Max_used_connections` 已经冲到 `321`，`Too many connections` 仍在日志中持续出现。
3. 下一轮优化重点不再是验证能否卖完，而是把连接峰值压回安全区，避免系统靠“打满数据库也刚好扛住”这种状态运行。

### 9.6 新增证据

- `.tmp/perf/order_user_pool.1000users.refresh_20260324.json`
- `.tmp/perf/current-run/logs/order-api.log`
- `.tmp/perf/current-run/logs/order-rpc.base.log`
- `.tmp/perf/current-run/logs/program-rpc.base.log`
- `.tmp/perf/current-run/logs/user-rpc.base.log`
