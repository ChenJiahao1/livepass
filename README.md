# damai-go

基于 Go 与 go-zero 的票务业务总线项目，当前采用 `Gateway -> API -> RPC` 分层，面向高并发下单、自动分座、模拟支付与智能客服联调场景。

## 文档职责

- `README.md` 面向项目开发者，说明项目定位、架构分层、目录结构、启动方式、测试方式与联调入口。
- `AGENTS.md` 面向仓库内使用的 AI agent，约束代码生成、命名、目录组织与工作方式。

## 当前架构

当前项目采用三层职责划分：

- `Gateway`：统一 HTTP 入口与路由汇聚点，负责统一鉴权接入、路由转发与观测接入。对应 `services/gateway-api/`。
- `API`：HTTP 适配与协议层，负责 handler/middleware、参数解析、上下文提取、外部响应模型与轻量聚合；不承载核心业务规则。对应 `services/*-api/`。
- `RPC`：服务契约与主要业务逻辑承载层，负责领域规则、状态流转、数据访问、缓存、消息队列与内部服务协作。对应 `services/*-rpc/`。

主调用链路：

```text
client -> gateway-api -> xxx-api -> xxx-rpc
```

## 业务边界

- `user`：注册、登录、资料维护、观演人管理
- `program`：节目、场次、票档、座位、预下单信息、系统自动分座冻结
- `order`：下单、查单、取消、支付检查、退款、超时关单
- `pay`：模拟支付、支付单查询、模拟退款
- `gateway`：统一外部 HTTP 入口
- `agents`：根级 Python 组件，提供基于 `LangGraph + MCP + Redis` 的 `/agent/chat` 能力

当前明确约束：

- `program` 不支持用户手动选座，但保留系统分配座位能力
- `pay` 不接真实支付通道，仅提供模拟支付与模拟退款

## 仓库结构

```text
damai-go/
├── README.md
├── AGENTS.md
├── docs/
├── deploy/
├── jobs/
├── pkg/
├── scripts/
├── services/
│   ├── gateway-api/
│   ├── user-api/        user-rpc/
│   ├── program-api/     program-rpc/
│   ├── order-api/       order-rpc/
│   └── pay-api/         pay-rpc/
├── sql/
├── tests/
└── agents/
```

目录约定：

- `services/*-api/`：HTTP 服务
- `services/*-rpc/`：gRPC 服务
- `jobs/*/`：后台任务与补偿任务
- `pkg/`：通用基础能力，禁止承载具体业务规则
- `agents/`：独立 Python 组件，不纳入 go-zero 服务目录规范

## 已落地能力

- `user`：注册、登录、用户资料与观演人链路
- `program`：分类、首页、分页、详情、票档、预下单详情、自动分座冻结
- `order` / `pay`：下单、查单、取消、模拟支付、退款、超时关单
- `gateway` / `agents`：统一 HTTP 入口与 `/agent/chat` 联调

## 本地依赖

基础设施通过 Docker Compose 启动：

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

当前基础设施包含：

- MySQL
- Redis
- etcd
- Kafka

如果需要模拟 `order-db-0 + order-db-1` 分片库：

```bash
docker compose -f deploy/mysql/docker-compose.sharding.yml up -d
```

对应端口：

- `order-db-0`: `127.0.0.1:3317`
- `order-db-1`: `127.0.0.1:3318`

## 初始化数据

MySQL 容器启动后，执行统一导入脚本初始化 `user/program/order/pay` 域表结构与种子数据：

```bash
bash scripts/import_sql.sh
```

常用覆盖项：

```bash
IMPORT_DOMAINS=program bash scripts/import_sql.sh
MYSQL_CONTAINER=docker-compose-mysql-1 MYSQL_PASSWORD=123456 bash scripts/import_sql.sh
```

可选数据库名覆盖：

```bash
MYSQL_DB_USER=damai_user \
MYSQL_DB_PROGRAM=damai_program \
MYSQL_DB_ORDER=damai_order \
MYSQL_DB_PAY=damai_pay \
bash scripts/import_sql.sh
```

## 运行测试

Go 服务与作业：

```bash
go test ./...
```

`agents` 组件测试：

```bash
cd agents
uv run pytest -v
```

## 启动服务

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user.yaml
go run services/program-rpc/program.go -f services/program-rpc/etc/program.yaml
go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay.yaml
go run services/order-rpc/order.go -f services/order-rpc/etc/order.yaml
go run jobs/order-close/cmd/worker/main.go -f jobs/order-close/etc/order-close-worker.yaml
go run jobs/order-close/cmd/dispatcher/main.go -f jobs/order-close/etc/order-close-dispatcher.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
go run services/program-api/program.go -f services/program-api/etc/program-api.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.yaml
go run services/pay-api/pay.go -f services/pay-api/etc/pay-api.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml
```

`agents` 组件独立启动：

```bash
cd agents
bash scripts/generate_proto_stubs.sh
uv run uvicorn app.main:app --host 0.0.0.0 --port 8891 --reload
```

建议启动顺序：

1. MySQL、Redis、etcd、Kafka
2. `user-rpc`、`program-rpc`、`pay-rpc`
3. `order-rpc`
4. `jobs/order-close/cmd/worker`
5. `jobs/order-close/cmd/dispatcher`
6. `user-api`、`program-api`、`order-api`、`pay-api`
7. `agents`
8. `gateway-api`

## 默认端口

- `user-rpc`: `8080`
- `gateway-api`: `8081`
- `order-rpc`: `8082`
- `program-rpc`: `8083`
- `pay-rpc`: `8084`
- `user-api`: `8888`
- `program-api`: `8889`
- `order-api`: `8890`
- `agents`: `8891`
- `pay-api`: `8892`

## 关键运行语义

- `user-rpc`、`program-rpc`、`order-rpc`、`pay-rpc` 默认注册到本地 `etcd`
- `gateway-api` 是统一外部入口，负责把 HTTP 请求转发到各域 API 或 `agents`
- `/order/create` 采用 `accept + async` 模式：Redis admission 成功后立即返回 `orderNumber`，异步 consumer 再完成锁座、写单与 guard 落库
- `/order/poll` 优先读取 Redis 中的 rush attempt 投影，并在非终态时回查 MySQL 是否已出现未支付订单
- `jobs/order-close/cmd/worker` 负责消费 Asynq 延迟任务，并调用 `order-rpc.CloseExpiredOrder` 推进超时关单
- `jobs/order-close/cmd/dispatcher` 负责扫描 `d_delay_task_outbox(order.close_timeout)`，补发延迟任务到 Asynq；真正的业务关闭仍只走 `order-rpc.CloseExpiredOrder`
- `gateway-api` 已启用 `Telemetry`；若要得到完整链路，需要给下游 API/RPC 同步补齐 `Telemetry`

## 验收与联调

推荐优先使用仓库内现成脚本与文档：

- 下单主路径：`docs/api/order-checkout-acceptance.md`
- 下单失败分支：`docs/api/order-checkout-failure-acceptance.md`
- 退款主路径：`docs/api/order-refund-acceptance.md`
- 智能客服联调：`scripts/acceptance/agent_chat.sh`

可执行脚本：

```bash
bash scripts/acceptance/order_checkout.sh
bash scripts/acceptance/order_checkout_failures.sh
bash scripts/acceptance/order_refund.sh
JWT=<user-jwt> bash scripts/acceptance/agent_chat.sh
```

`agents` 契约测试：

```bash
cd agents
uv run pytest tests/test_e2e_contract.py -v
```

## 常用联调配置

`agents` 至少需要确认以下配置：

- `REDIS_URL=redis://127.0.0.1:6379/0`
- `USER_RPC_TARGET=127.0.0.1:8080`
- `PROGRAM_RPC_TARGET=127.0.0.1:8083`
- `ORDER_RPC_TARGET=127.0.0.1:8082`

访问统一入口时，订单相关接口通常需要：

- `Authorization: Bearer <jwt>`

## 开发原则

- 核心业务规则进入 `RPC`
- `API` 保持薄层，避免回流核心业务逻辑
- `Gateway` 只做入口、路由与横切能力，不承载业务规则
- 新增 go-zero 服务时，遵循 `services/*-api/` 与 `services/*-rpc/` 的目录规范
