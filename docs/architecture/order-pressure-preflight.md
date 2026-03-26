# Order 压测前置检查 Runbook

本文整理 `order/create` 抢票链路每次压测前必须确认的内容，并配套 `preflight` 脚本，避免重复手工探索环境状态。

适用链路：

- `gateway-api -> order-api -> order-rpc -> program-rpc + user-rpc -> Kafka consumer -> MySQL/Redis`

配套脚本：

- [scripts/perf/order_pressure_preflight.sh](/home/chenjiahao/code/project/damai-go/scripts/perf/order_pressure_preflight.sh)

## 1. 使用目标

`preflight` 解决的是“压测开始前环境到底能不能跑”的问题，不负责完整起服务和执行压测。

首版覆盖的重点：

- 基础设施容器是否可用
- Kafka topic 分区数是否满足目标 `order-rpc` 实例数
- 用户池是否重复、JWT 是否过期、MySQL 是否存在对应用户和观演人
- 用户池对应的 `order limit ledger` 是否已经 ready
- 节目、票档、座位数是否满足目标场景
- 压测前数据库和 Redis 是否是干净起跑线
- Redis seat ledger 是否 ready
- 服务端口是否被残留进程占用

当前不会自动做的事：

- 自动补造压测用户池
- 自动补造 `1000` 座位
- 自动拉起 `5` 个或 `10` 个实例
- 自动执行整轮 `k6`

## 2. 三种模式

脚本支持 3 种模式：

- `check`
  - 只检查，不修改环境
- `prepare`
  - 在 `check` 基础上允许做轻量准备动作
  - 当前只包含 Kafka topic 分区修正，以及可选的 ledger warmup
- `report`
  - 和 `check` 一样执行检查，但更适合在人工确认前打印摘要

## 3. 常用参数

最常用的环境变量如下：

- `USER_POOL_FILE`
  - 多用户抢票场景使用的用户池 JSON 文件
- `TARGET_RPC_INSTANCES`
  - 目标 `order-rpc / program-rpc / user-rpc` 实例数
- `PROGRAM_ID`
- `TICKET_CATEGORY_ID`
- `REQUIRED_SEAT_COUNT`
  - 本轮压测期望的座位总量
- `REQUIRE_CLEAN_STATE`
  - 默认 `1`
  - 要求订单分片表、Redis frozen keys 在开跑前是干净的
- `REQUIRE_ZERO_LAG`
  - 默认 `1`
  - 要求 Kafka consumer lag 为 `0`
- `PORTS_TO_CHECK`
  - 可选
  - 不传时脚本会按 `TARGET_RPC_INSTANCES` 自动推导常用端口
- `JWT`
  - 单用户 warmup 或 baseline 场景可选
- `TICKET_USER_IDS`
  - 配合 `PREWARM_ON_PREPARE=1` 使用
- `PREWARM_ON_PREPARE`
  - 默认 `0`
  - `prepare` 模式下若 Redis ledger 未 ready，允许调用 [scripts/perf/prewarm_order_ledgers.sh](/home/chenjiahao/code/project/damai-go/scripts/perf/prewarm_order_ledgers.sh)
- `PREWARM_ORDER_LIMIT_LEDGER_ON_PREPARE`
  - 默认 `1`
  - `prepare` 模式下若用户池里存在缺失的 `order limit ledger`，自动调用 `go run ./scripts/perf/prewarm_order_limit_ledgers.go` 补齐

## 4. 推荐顺序

每次压测前建议固定按这个顺序执行：

1. 启动基础设施
2. 运行 `preflight check`
3. 若 Kafka topic 分区不足，运行 `preflight prepare`
4. 若环境报“脏状态”，先清理数据或重建 seat ledger
5. 再跑一次 `preflight check`
6. 只有全部通过后，才开始起服务和执行 `k6`

## 5. 典型命令

### 5.1 5 实例、2000 用户、1000 座位

```bash
USER_POOL_FILE=.tmp/perf/order_user_pool.2000users.unique_20260324.json \
TARGET_RPC_INSTANCES=5 \
PROGRAM_ID=10001 \
TICKET_CATEGORY_ID=40001 \
REQUIRED_SEAT_COUNT=1000 \
bash scripts/perf/order_pressure_preflight.sh check
```

### 5.2 需要顺手修正 Kafka topic 分区

```bash
USER_POOL_FILE=.tmp/perf/order_user_pool.2000users.unique_20260324.json \
TARGET_RPC_INSTANCES=5 \
PROGRAM_ID=10001 \
TICKET_CATEGORY_ID=40001 \
REQUIRED_SEAT_COUNT=1000 \
bash scripts/perf/order_pressure_preflight.sh prepare
```

### 5.3 ledger 未 ready，允许 prepare 顺手 warmup

```bash
USER_POOL_FILE=.tmp/perf/order_user_pool.2000users.unique_20260324.json \
TARGET_RPC_INSTANCES=5 \
PROGRAM_ID=10001 \
TICKET_CATEGORY_ID=40001 \
REQUIRED_SEAT_COUNT=1000 \
JWT='<single-user-jwt>' \
TICKET_USER_IDS='701' \
PREWARM_ON_PREPARE=1 \
bash scripts/perf/order_pressure_preflight.sh prepare
```

### 5.4 5 实例、2000 用户、2000 座位、随机 1-3 张

```bash
USER_POOL_FILE=.tmp/perf/order_user_pool.2000users.random_1_3_20260324.json \
TARGET_RPC_INSTANCES=5 \
PROGRAM_ID=10001 \
TICKET_CATEGORY_ID=40001 \
REQUIRED_SEAT_COUNT=2000 \
bash scripts/perf/order_pressure_preflight.sh prepare
```

如果 `prepare` 成功，说明：

- Kafka topic 分区数已经满足 `5` 实例
- seat ledger 已 ready
- 用户池对应的 `order limit ledger` 已 ready

此时再执行：

```bash
k6 run \
  --summary-export .tmp/perf/run-20260324-5rpc-2000users-random13/results/k6_random_2000users_2000seats.json \
  -e USER_POOL_FILE=/home/chenjiahao/code/project/damai-go/.tmp/perf/order_user_pool.2000users.random_1_3_20260324.json \
  -e TARGET_USERS=2000 \
  -e EXECUTOR=shared-iterations \
  -e VU_COUNT=1000 \
  -e PROGRAM_ID=10001 \
  -e TICKET_CATEGORY_ID=40001 \
  -e MAX_DURATION=120s \
  scripts/perf/order_create_gateway_multi_user_random_seat_count.js
```

## 6. 脚本检查项说明

### 6.1 基础设施

脚本会确认这些依赖可用：

- `docker-compose-mysql-1`
- `docker-compose-redis-1`
- `docker-compose-etcd-1`
- `docker-compose-kafka-1`
- MySQL 可查询
- Redis 可 `PING`
- etcd health 为 `true`

### 6.2 Kafka

脚本会检查：

- `order.create.command.v1` 分区数是否 `>= TARGET_RPC_INSTANCES`
- `damai-go-order-create` 的 consumer lag 是否为 `0`

如果是 `prepare` 模式，且分区数不足，会调用 [scripts/perf/prepare_order_kafka_topic.sh](/home/chenjiahao/code/project/damai-go/scripts/perf/prepare_order_kafka_topic.sh) 自动修正。

### 6.3 用户池

如果设置了 `USER_POOL_FILE`，脚本会检查：

- 文件存在
- JSON 格式合法
- `userId` 唯一
- `ticketUserId` / `ticketUserIds[]` 展开后唯一
- JWT 未过期
- MySQL 中确实存在对应 `d_user` 和 `d_ticket_user`

如果设置了 `JWT`，脚本也会额外检查单个 token 是否过期，并打印 `userId`。

### 6.4 Order Limit Ledger

如果设置了 `USER_POOL_FILE`，脚本还会检查：

- `damai-go:order:purchase-limit:ledger:{userId}:{programId}` 是否对整批用户都已经 ready
- 若 `MODE=prepare` 且 `PREWARM_ORDER_LIMIT_LEDGER_ON_PREPARE=1`
  - 自动调用 [scripts/perf/prewarm_order_limit_ledgers.go](/home/chenjiahao/code/project/damai-go/scripts/perf/prewarm_order_limit_ledgers.go) 批量补齐

这是 `2000` 用户并发场景的必要前置条件。否则首单高并发下会大面积命中 `order limit ledger not ready`，导致压测结果主要反映“账本冷启动失败”，而不是真正的抢票承压表现。

### 6.5 数据准备

脚本会检查：

- `programId` 是否存在
- `ticketCategoryId` 是否存在
- 该票档总座位数是否等于 `REQUIRED_SEAT_COUNT`
- `remain_number` 是否等于 `REQUIRED_SEAT_COUNT`
- MySQL 可售座位数是否等于 `REQUIRED_SEAT_COUNT`
- 若 `REQUIRE_CLEAN_STATE=1`
  - `damai_order.d_order_00 + damai_order.d_order_01 = 0`

### 6.6 Redis seat ledger

脚本会检查：

- `damai-go:program:seat-ledger:stock:{programId}:{ticketCategoryId}` 是否存在
- `available_count` 是否等于 `REQUIRED_SEAT_COUNT`
- `available zset` 是否等于 `REQUIRED_SEAT_COUNT`
- frozen keys 是否为 `0`

## 7. 结果判读

脚本输出分为三类：

- `PASS`
  - 当前检查项满足压测要求
- `WARN`
  - 当前项被跳过，或者只是提示，不阻塞压测
- `FAIL`
  - 当前项不满足压测前提，应该先处理再开跑

结尾会输出摘要：

```text
Summary: pass=<n> warn=<n> fail=<n> mode=<mode>
```

只要 `fail > 0`，脚本就会返回非 `0` 退出码。

## 8. 当前已知注意事项

### 8.1 仓库默认 seed 不是 1000 座

当前仓库 [sql/program/dev_seed.sql](/home/chenjiahao/code/project/damai-go/sql/program/dev_seed.sql) 默认只给 `40001` 导入了 `100` 个普通票座位。

如果你要跑：

- `2000` 用户
- `1000` 座位

那必须先保证测试库里真的已经扩成 `1000` 座。`preflight` 只会报错，不会替你自动补座位。

### 8.2 当前 `/order/create` 的冻结口径看 Redis，不看 MySQL

现有实现里，创建订单阶段调用的是 Redis seat ledger 的冻结逻辑；MySQL 不作为冻结状态的前置校验来源。

因此压测前的“冻结是否清空”需要同时看：

- MySQL 的脏表状态
- Redis 的 seat ledger 状态

### 8.3 首轮冷启动噪音

如果 `order-api -> order-rpc` 的服务发现还没稳定，首轮可能出现短暂 `503`。

建议：

- 起完服务后先做一次 warmup
- 或者把第一次结果只当冷启动参考，不作为稳定 QPS 口径

### 8.4 `order limit ledger` 不是 seat ledger 的替代项

`seat ledger` 解决的是“有没有座位可冻”；`order limit ledger` 解决的是“这个用户在该节目维度还能不能继续下单”。

两者都必须 ready：

- 只预热 `seat ledger`，首单并发仍可能被 `order limit ledger not ready` 打穿
- 只预热 `order limit ledger`，座位冻结仍会因为 seat ledger 缺失而失败

## 9. 建议的下一步

如果后面还想继续减少人工操作，下一阶段建议再补两件事：

- 一键清场和 seat ledger 重建脚本
- 一键启动 `5` 实例 / `10` 实例的压测拓扑脚本
