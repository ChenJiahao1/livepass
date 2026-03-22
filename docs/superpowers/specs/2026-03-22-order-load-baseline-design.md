# 订单全链路压测基线设计

## 背景

当前 `damai-go` 的下单链路已经完成：

- 下单前限购预占位使用 Redis ledger
- 锁座主路径使用 Redis seat ledger
- 订单创建采用 Kafka 异步落库

这让首轮压测的主要瓶颈从“锁座全量扫描”转移到了“Kafka 异步落库拓扑”和“网关超时配置”。

当前实现中仍存在以下会污染压测基线的问题：

- `order.create` topic 默认只创建 `1` 个 partition
- `order-rpc` 进程内只有 `1` 个 Kafka consumer loop
- `MaxMessageDelay` 当前为 `5s`
- `gateway-api` 到 `order-api` 的 upstream timeout 当前为 `3000ms`

在这种组合下，压测更容易测到：

- Kafka 串行积压
- `5s` 超时废单保护被提前触发
- 网关超时先于真实容量触发

而不是测到“单机单实例、合理 Kafka 并行度下的全链路基线”。

## 目标

本设计的目标是建立首轮“本地单机单实例、全链路、保留废单语义”的压测基线。

目标包括：

- 保留当前业务语义：`/order/create` 返回成功仅代表 Kafka 发送成功
- 保留异步可见窗口
- 保留“超时废单并释放锁座”语义
- 通过最小结构改造降低 Kafka 串行积压对结果的污染
- 让首轮结果更接近后续真实扩容前的单机基线

## 非目标

本轮不解决以下问题：

- 不引入 DLQ
- 不引入消息对账系统
- 不做多实例压测
- 不改变订单异步落库主流程
- 不把超时废单改成“永不废单”
- 不直接做生产级完整观测体系

## 方案概览

首轮采用“最小改造版”方案：

- 压测环境将 `Kafka.MaxMessageDelay` 从 `5s` 放宽到 `60s`
- 压测环境将 `gateway-api -> order-api` 的 upstream timeout 从 `3000ms` 放宽到 `10000ms`
- `order.create` topic 使用 `4` 个 partition
- 单个 `order-rpc` 进程内启动 `4` 个 consumer worker
- producer 保持按 `orderNumber` 做 hash 分区
- 首轮压测入口仍走 `gateway-api`

该方案保留原有业务语义，但降低“单分区串行积压导致误触废单”的概率。

## 架构调整

### 1. Kafka 配置

为 `services/order-rpc/internal/config/config.go` 的 `KafkaConfig` 增加两个配置项：

- `TopicPartitions`
- `ConsumerWorkers`

建议默认值：

- `TopicPartitions: 1`
- `ConsumerWorkers: 1`

这样可以保持现有开发语义不变，只在压测环境通过专用配置覆盖。

压测环境建议值：

- `MaxMessageDelay: 60s`
- `TopicPartitions: 4`
- `ConsumerWorkers: 4`

### 2. Topic 分区策略

`order.create` topic 的目标分区数由配置驱动，而不是硬编码为 `1`。

但服务启动阶段不自动扩容已有 topic，避免对 Kafka 元数据产生隐式副作用。

推荐行为：

- topic 不存在时：按 `TopicPartitions` 创建
- topic 已存在且分区数满足要求时：正常启动
- topic 已存在但分区数小于目标值时：
  - 服务打印明确告警
  - 服务继续启动
  - 由压测前置脚本或手工命令显式完成扩容

这样能把 Kafka 元数据调整明确纳入压测准备流程，而不是隐藏在服务启动逻辑里。

### 3. 单进程多 consumer worker

`order-rpc` 单进程内按 `ConsumerWorkers` 启动多个 worker。

每个 worker：

- 持有独立的 `kafka.Reader`
- 使用相同 `GroupID`
- 运行独立消费循环

不采用“一个 reader 多协程并发 `FetchMessage`”模式，原因是 `kafka-go` 下“一 worker 一 reader”更稳定，也更贴合 Kafka consumer group 的分配模型。

首轮建议：

- `partitions = 4`
- `workers = 4`

这样单机单实例下最容易获得稳定分配和可解释结果。

## 数据流与语义保持

并行消费后，业务语义保持不变：

1. `gateway-api` 收到 `/order/create`
2. `order-api` 调用 `order-rpc`
3. `order-rpc` 完成：
   - 限购预占位
   - 锁座
   - 生成订单事件
   - Kafka 发送
4. Kafka 发送成功后，接口立即返回订单号
5. `order-rpc` consumer 异步落库
6. 成功后提交 offset
7. 超过 `MaxMessageDelay` 的消息仍按废单补偿处理

并行消费不会破坏当前一致性边界，原因如下：

- producer 已按 `orderNumber` 作为 key 发送
- 订单表存在 `order_number` 唯一键
- consumer 已对重复写入做幂等处理
- 限购预占位和锁座都发生在 Kafka 之前

因此，多 partition + 多 worker 主要影响吞吐，不改变订单创建语义。

## 网关与全链路口径

首轮压测入口保留 `gateway-api`，因为目标是“全链路基线”。

但为了避免网关默认超时先成为瓶颈，压测环境需要将：

- `gateway-api -> order-api` timeout 调整为 `10000ms`

该调整只服务于首轮压测口径，不代表生产默认值结论。

## 压测环境配置策略

为避免污染现有本地开发配置，建议新增压测专用配置文件，而不是直接覆盖默认 `etc/*.yaml`。

建议新增：

- `services/order-rpc/etc/order-rpc.perf.yaml`
- `services/gateway-api/etc/gateway-api.perf.yaml`

其中：

- `order-rpc.perf.yaml` 覆盖 `MaxMessageDelay`、`TopicPartitions`、`ConsumerWorkers`
- `gateway-api.perf.yaml` 覆盖 `order-api` upstream timeout

后续压测脚本统一引用 perf 配置文件启动服务。

## 观测设计

首轮压测要补最小观测面，重点是 QPS、延迟、废单和 lag。

### 1. go-zero 指标

利用 go-zero 自带 Prometheus 指标能力，给以下服务补齐 `Prometheus` 配置：

- `gateway-api`
- `order-api`
- `order-rpc`

关注指标：

- HTTP 请求总数
- HTTP 延迟
- gRPC 请求总数
- gRPC 延迟
- 活跃请求数

QPS 通过 `requests_total` 类 counter 的 `rate(...)` 计算，而不是依赖单独的“QPS 字段”。

### 2. 业务日志与计数

首轮至少增加或明确统计以下业务计数：

- `create success count`
- `expired skip count`
- `duplicate consume count`
- `consumer error count`

其中 `expired skip count` 是首轮压测是否通过的核心指标之一。

### 3. Kafka 观测

需要至少能拿到：

- `order.create` topic 当前 lag
- 测试结束后的 lag 回落时间

首轮可先通过脚本或命令行工具补，不强制要求服务内嵌展示。

## 压测前置准备

在正式压测前必须完成以下准备：

- 使用专门的压测 `programId` 和 `ticketCategoryId`
- 准备足够的座位库存，避免单轮测试中途耗尽
- 准备足够多的用户与观演人数据
- 预生成 JWT，避免将登录链路混入主压测回路
- 预热 `seat ledger`
- 预热 `purchase limit ledger`
- 显式将 `order.create` topic 扩容到目标分区数
- 暂停 `jobs/order-close`，避免测试窗口内引入关单噪音

## 压测口径

首轮只压 `/order/create` 主路径，但入口走 `gateway-api`。

为了保持口径干净：

- 不把登录、注册、查观演人放入主压测回路
- 不在主压测脚本中同步查询 `/order/get`
- 仅做小比例抽样查询 `/order/get`，统计异步可见延迟

推荐单轮结构：

- 预热：`1m`
- 稳态：`5m`

单轮压测时间应控制在 `CloseAfter=15m` 内，避免未支付订单在同轮测试中跨入超时关单窗口。

## 验收指标

首轮压测至少观察以下指标：

- 网关侧：
  - `/order/create` 的 `2xx / 4xx / 5xx / timeout`
  - `p50 / p95 / p99`
- 订单异步侧：
  - 从返回成功到 `/order/get` 可见的 `p50 / p95 / p99`
- Kafka 侧：
  - lag 峰值
  - lag 回落时间
- 业务一致性侧：
  - `create success count`
  - `expired skip count`
  - `最终落库订单数`
- 数据库侧：
  - 慢 SQL
  - 事务耗时
  - 锁等待

## 首轮通过标准

推荐首轮通过标准如下：

- 目标稳态 RPS 下，`expired skip count = 0`
- `/order/create` 的 `5xx + timeout` 接近 `0`
- 测试结束后 Kafka lag 能在短时间内回落到 `0`
- `create success count` 与最终落库订单数一致
- 若保留抽样对账，应满足：
  - `create success = persisted + expired`
- 异步可见延迟显著小于 `60s`

建议先以：

- `异步可见延迟 p99 < 10s`

作为首轮基线目标，而不是仅以“未超过 60s”作为通过标准。

## 风险与折中

### 风险 1：60s 仍可能掩盖真实积压

这是有意的压测折中。

首轮目标是测单机基线，不是验证废单保护阈值的敏感度。等基线稳定后，再逐步收紧到 `30s`、`15s` 或更低，观察废单拐点。

### 风险 2：多 worker 不能替代完整消息治理

本方案只解决“单机单实例压测基线污染”的问题，不解决：

- DLQ
- 对账
- 重试治理
- 消息补偿体系

这些工作应在后续生产化压测阶段处理。

### 风险 3：全链路压测仍会受网关影响

这是全链路口径本身的组成部分。

通过将 `gateway -> order-api timeout` 放宽到 `10s`，可以降低“人为过紧超时”对结果的污染，但不会完全剥离网关成本。

## 实施顺序

建议按以下顺序落地：

1. 为 `order-rpc` 增加 `TopicPartitions` 和 `ConsumerWorkers` 配置
2. 改造 topic 创建逻辑，支持按目标分区数创建并对不足分区告警
3. 改造 consumer runner，支持单进程多 worker
4. 新增 perf 配置文件
5. 给 `gateway-api`、`order-api`、`order-rpc` 补 Prometheus 配置
6. 增加压测前置准备和 topic 扩容脚本
7. 再补首轮压测脚本与结果采集脚本

## 测试策略

实现完成后至少需要覆盖：

- 配置解析测试：
  - `TopicPartitions`
  - `ConsumerWorkers`
- Kafka topic 初始化测试：
  - 新建 topic 使用目标分区数
  - 已有 topic 分区不足时触发告警路径
- consumer runner 测试：
  - 按 worker 数量启动多个 consumer
  - stop 时能完整退出
- 集成测试：
  - 多 worker 场景下重复消费仍幂等
  - `MaxMessageDelay=60s` 时正常消息不会误废单
- 压测前置脚本测试：
  - 能正确扩容 topic
  - 能完成 ledger 预热

## 决策结论

首轮压测基线采用以下固定方案：

- 全链路入口走 `gateway-api`
- 压测环境保留废单语义，但将 `MaxMessageDelay` 放宽到 `60s`
- `gateway-api -> order-api timeout` 调整为 `10000ms`
- `order.create` topic 使用 `4` 个 partition
- 单个 `order-rpc` 进程内启动 `4` 个 consumer worker
- 补最小 Prometheus 指标与业务计数
- 压测前完成 topic 扩容、ledger 预热和数据准备

该方案优先保证“首轮压测基线准确”，而不是维持当前单分区串行拓扑的极限表现。
