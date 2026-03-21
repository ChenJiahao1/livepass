# Kafka 纳入基础设施 Compose 设计

## 背景

当前仓库提供的基础设施编排文件是 `deploy/docker-compose/docker-compose.infrastructure.yml`，只包含 `etcd`、`mysql` 和 `redis`。但订单创建链路已经改为 `Redis 锁座 + Kafka 异步落库`，`README.md` 以及下单验收文档都要求开发者额外准备 Kafka broker。

这导致本地启动路径分裂：

- 基础设施入口命令无法一次拉起所有必需依赖
- 文档需要额外解释 Kafka 需单独准备
- 新成员按 README 执行后，`/order/create` 仍可能因为 Kafka 不可用而失败

本次变更的目标，是把 Kafka 纳入统一的基础设施 compose 入口，降低本地联调和验收门槛。

## 目标

- 在 `deploy/docker-compose/docker-compose.infrastructure.yml` 中新增单节点 Kafka 服务
- 保持本地基础设施一条命令启动：`docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d`
- 让宿主机启动的 Go 服务能通过固定地址连接 Kafka
- 让 compose 网络内的其他容器也能通过服务名连接 Kafka
- 同步更新 README 和订单验收文档，消除“Kafka 需额外单独启动”的过时说明

## 非目标

- 不修改 `services/order-rpc/` 的业务逻辑、producer/consumer 实现和消息模型
- 不引入 Kafka UI、topic 初始化容器或额外运维组件
- 不扩展成多 broker、高可用或生产部署拓扑
- 不为当前变更新增业务级集成测试

## 方案选型

### 方案 1：纳入现有基础设施 compose

直接在 `deploy/docker-compose/docker-compose.infrastructure.yml` 中增加 Kafka service，与 `etcd`、`mysql`、`redis` 同级。

优点：

- 最符合当前仓库“统一基础设施入口”的使用习惯
- 本地联调和验收路径最短
- 文档和命令最容易统一

缺点：

- 启动基础设施时会比现在多一个依赖

### 方案 2：单独维护 Kafka compose 文件

新建专用 Kafka compose 文件，再由 README 指导用户同时启动两个 compose 文件。

优点：

- 职责分组更清晰

缺点：

- 增加使用复杂度
- 与“统一入口”目标不一致

### 结论

采用方案 1。当前项目处于本地开发和验收优先阶段，统一入口比拆分文件更重要。

## 目标架构

基础设施 compose 变更后包含四个 service：

- `etcd`
- `mysql`
- `redis`
- `kafka`

Kafka 使用单节点 KRaft 模式，避免额外引入 ZooKeeper。监听地址区分两类访问：

- 宿主机访问：`localhost:9094`
- compose 网络内访问：`kafka:9092`

这样可以同时满足两种运行方式：

- 开发者在宿主机直接执行 `go run services/order-rpc/order.go ...`
- 后续如果某些服务迁入 compose 网络，可直接通过服务名访问 broker

## 组件设计

### Kafka service

Kafka service 需要具备以下特征：

- 使用支持 KRaft 单节点模式的镜像
- 对宿主机暴露固定端口 `9094`
- 容器内保留 broker listener `9092`
- 显式配置 `advertised listeners`，避免客户端拿到错误地址
- 挂载数据卷或使用命名卷保存 broker 数据，避免容器重建后元数据混乱
- 增加基础健康检查，尽量减少 compose 已启动但 broker 尚不可用的情况

### 文档更新范围

需要同步更新的文档包括：

- `README.md`
- `docs/api/order-checkout-acceptance.md`
- `docs/api/order-checkout-failure-acceptance.md`

更新重点：

- 说明基础设施 compose 已包含 Kafka
- 说明启动成功判定应包含 Kafka
- 明确 `services/order-rpc/etc/order-rpc.yaml` 中 `Kafka.Brokers` 在本地应与 `localhost:9094` 对齐

## 数据流与运行方式

本次不改变业务数据流，只改变本地基础设施提供方式。

变更前：

1. 开发者启动基础设施 compose
2. compose 中只包含 `etcd/mysql/redis`
3. 开发者还需自行准备 Kafka broker
4. `order-rpc` 使用额外 broker 地址收发消息

变更后：

1. 开发者执行统一命令启动基础设施 compose
2. compose 同时拉起 `etcd/mysql/redis/kafka`
3. `order-rpc` 使用 `localhost:9094` 连接本地 Kafka
4. 下单链路仍按现有逻辑写入 Kafka 并由 consumer 异步消费

## 错误处理

本次设计主要关注基础设施可用性和文档一致性：

- 如果 Kafka 容器未就绪，`docker compose ps` 应能体现异常状态，帮助定位问题
- 如果 `Kafka.Brokers` 仍指向旧地址，README 和验收文档要明确提示应与 compose 暴露地址保持一致
- 如果用户只启动旧版 compose 或未更新本地配置，`/order/create` 仍可能失败，但这属于使用方未按文档对齐环境，不在本次代码层兜底范围

## 测试与验证

本次变更的验证分为两层：

### 静态校验

- 执行 `docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml config`
- 确认 compose 语法合法，Kafka service 展开结果符合预期

### 文档一致性校验

- 检查 `README.md` 中的基础设施说明是否已改为包含 Kafka
- 检查下单主路径与失败分支验收文档是否不再要求“单独启动 Kafka”
- 检查文档中的 broker 地址说明是否与 `localhost:9094` 一致

### 可选运行校验

若当前环境允许执行 Docker，可额外运行：

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d kafka
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml ps kafka
```

成功标准：

- Kafka 容器保持 `Up`
- 没有持续重启

## 实施边界

本次实施只允许修改以下范围：

- `deploy/docker-compose/docker-compose.infrastructure.yml`
- `README.md`
- `docs/api/order-checkout-acceptance.md`
- `docs/api/order-checkout-failure-acceptance.md`
- 必要时新增与 Kafka compose 相关的最小注释或说明

不应触碰：

- `services/order-rpc/internal/**`
- 任何消息体定义、topic 常量、消费逻辑
- 任何与支付、节目、用户域无关的代码

## 实施后预期结果

- 本地基础设施可以通过统一 compose 命令启动完整的订单链路依赖
- README 与验收文档不再出现“Kafka 需要额外单独启动”的冲突说明
- 开发者只需保证 `order-rpc` 配置中的 `Kafka.Brokers` 指向 `localhost:9094`，即可在本机完成下单异步链路联调
