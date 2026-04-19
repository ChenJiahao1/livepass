# livepass

基于 Go 与 go-zero 的票务业务总线项目，当前采用 `Gateway -> API -> RPC` 分层，面向高并发下单、自动分座、模拟支付与智能客服联调场景。

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
- `agents`：根级 Python 组件，提供基于 `Python 3.12 + LangGraph 1.1.6 + MCP + Redis + MySQL` 的 `Thread / Message / Run` API

当前明确约束：

- `program` 不支持用户手动选座，但保留系统分配座位能力
- `pay` 不接真实支付通道，仅提供模拟支付与模拟退款

## 仓库结构

```text
livepass/
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
- `gateway` / `agents`：统一 HTTP 入口与 `Thread / Message / Run` 联调

## 快速开始

推荐直接使用一键启动脚本：

```bash
bash scripts/deploy/start_backend.sh
```

该脚本会自动完成：

- 拉起 MySQL、Redis、etcd、Kafka 基础设施
- 检测业务库是否为空；若为空则自动导入 schema 与 seed
- 启动全部 Go RPC / API / Job
- 启动 `order-mcp`、`program-mcp`
- 启动 `agents`
- 最后启动 `gateway-api`
- 默认以前台 supervisor 模式保活；若直接在受控执行环境中启动，脚本不退出时可避免其子进程被宿主清理

常用变体：

```bash
# 强制重新导入 SQL
bash scripts/deploy/start_backend.sh --import-sql

# 强制重启已运行服务后再启动
bash scripts/deploy/start_backend.sh --force-restart

# 只启动后端主链路，跳过 MCP 和 agents
bash scripts/deploy/start_backend.sh --skip-agents

# 只启动 agents 相关链路（user/program/order-rpc + MCP + agents）
bash scripts/deploy/start_backend.sh --only-agents

# 启动完成后立即退出，保留旧的“只拉起进程不保活脚本”行为
bash scripts/deploy/start_backend.sh --detach

# 使用压测配置启动已有 perf 配置的 Go 服务
bash scripts/deploy/start_backend.sh --perf --force-restart
```

如需重建运行数据，请使用独立脚本：

```bash
bash scripts/deploy/rebuild_databases.sh
```

该脚本会：

- 删除并重建 `user/program/order/pay/agents` MySQL 业务库
- 重新导入全部 schema 与 seed
- 清空 Redis DB `0`
- 删除并重建 Kafka 业务 Topic（默认 `ticketing.attempt.command.v1`）

本地初始化后会预置一个普通测试用户：

- 手机号：`13800000000`
- 密码：`123456`

## 基础设施与数据

默认基础设施为：

- MySQL
- Redis
- etcd
- Kafka

如果只想手动拉起基础设施：

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

如果需要模拟 `order-db-0 + order-db-1` 分片库：

```bash
docker compose -f deploy/mysql/docker-compose.sharding.yml up -d
```

对应端口：

- `order-db-0`: `127.0.0.1:3317`
- `order-db-1`: `127.0.0.1:3318`

如需单独导入 SQL：

```bash
bash scripts/import_sql.sh
IMPORT_DOMAINS=program bash scripts/import_sql.sh
IMPORT_DOMAINS=agents bash scripts/import_sql.sh
MYSQL_CONTAINER=docker-compose-mysql-1 MYSQL_PASSWORD=123456 bash scripts/import_sql.sh
```

如需覆盖数据库名：

```bash
MYSQL_DB_USER=livepass_user \
MYSQL_DB_PROGRAM=livepass_program \
MYSQL_DB_ORDER=livepass_order \
MYSQL_DB_PAY=livepass_pay \
MYSQL_DB_AGENTS=livepass_agents \
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

## 手动启动（调试用）

如需跳过一键脚本、手动排查链路，可按下面方式启动：

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
go run services/order-rpc/cmd/order_mcp_server/main.go -f services/order-rpc/etc/order-mcp.yaml
go run services/program-rpc/cmd/program_mcp_server/main.go -f services/program-rpc/etc/program-mcp.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml
```

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
- `order-mcp`: `9082`
- `program-mcp`: `9083`

## 关键运行语义

- `user-rpc`、`program-rpc`、`order-rpc`、`pay-rpc` 默认注册到本地 `etcd`
- `gateway-api` 是统一外部入口，负责把 HTTP 请求转发到各域 API 或 `agents`
- `agents` 的运行态支持 `GET /agent/runs/{runId}/events?after=` 回放 run 事件，并对 `resume/cancel` 重试保持接口级幂等
- `/order/create` 采用 `accept + async` 模式：Redis admission 成功后立即返回 `orderNumber`；Kafka 由进程内异步任务发送，producer 失败只通过 `PENDING -> FAILED` CAS 尝试回补
- `/order/poll` 在 TTL 期间优先读取 Redis attempt；attempt miss 时查 MySQL by `orderNumber`，DB 有单为成功，DB 无单为失败
- `jobs/order-close/cmd/worker` 负责消费 Asynq 延迟任务，并调用 `order-rpc.CloseExpiredOrder` 推进超时关单
- `jobs/order-close/cmd/dispatcher` 负责扫描 `d_delay_task_outbox(order.close_timeout)`，补发延迟任务到 Asynq；真正的业务关闭仍只走 `order-rpc.CloseExpiredOrder`
- `gateway-api` 已启用 `Telemetry`；若要得到完整链路，需要给下游 API/RPC 同步补齐 `Telemetry`

## 验收与联调

推荐优先使用仓库内现成脚本与文档：

- 下单主路径：`docs/api/order-checkout-acceptance.md`
- 下单失败分支：`docs/api/order-checkout-failure-acceptance.md`
- 退款主路径：`docs/api/order-refund-acceptance.md`
- 智能客服联调：`scripts/acceptance/agent_threads.sh`

## 抢票核心链路压测

当前仓库已补充“从 `create order` 起压”的压测准备与执行脚本，默认面向：

- 单 `showTimeId`
- 单 `ticketCategoryId`
- 每用户随机 `1-3` 张
- 超卖竞争模型

### 1. 使用压测配置启动

压测模式通过一键启动脚本的 `--perf` 参数开启，会将已有 `*.perf.yaml` 的 Go 服务统一切到压测配置；当前包括 `gateway/user/program/order/pay` 的 API/RPC 服务。未提供 perf 配置的 Job、MCP 与 agents 继续使用默认配置。

```bash
bash scripts/deploy/start_backend.sh --perf --force-restart
```

### 2. 准备压测数据集

默认会：

- 重建目标票档座位
- 批量插入压测用户与观演人
- 预热 rush runtime 与 seat ledger
- 批量申请 `purchase token`
- 导出 `users.json` / `users.csv` / `meta.json`

示例：

```bash
BASE_URL=http://127.0.0.1:8081 \
SHOW_TIME_ID=30001 \
TICKET_CATEGORY_ID=40001 \
USER_COUNT=5000 \
SEAT_COUNT=5000 \
ROW_COUNT=50 \
COL_COUNT=100 \
PERF_SECRET=livepass-perf-secret-0001 \
bash scripts/perf/prepare_rush_perf_dataset.sh
```

输出目录默认位于：

```bash
tmp/perf/<dataset-id>/
```

其中：

- `users.json`：给 k6 直接读取
- `users.csv`：人工查看
- `meta.json`：记录参数与生成时间

### 3. 执行 k6 压测

```bash
k6 run \
  -e DATASET_PATH=tmp/perf/<dataset-id>/users.json \
  -e BASE_URL=http://127.0.0.1:8081 \
  -e PERF_SECRET=livepass-perf-secret-0001 \
  tests/perf/rush_create_order.js
```

### 4. 校验压测结果

```bash
SHOW_TIME_ID=30001 \
TICKET_CATEGORY_ID=40001 \
bash scripts/perf/verify_rush_perf_result.sh
```

校验脚本会输出：

- 票档总库存
- 票档剩余库存
- `seat_status = 3` 的已售座位数
- `d_order_seat_guard*` 聚合数量
- `d_order_ticket_user*` 聚合数量

可执行脚本：

```bash
bash scripts/acceptance/order_checkout.sh
bash scripts/acceptance/order_checkout_failures.sh
bash scripts/acceptance/order_refund.sh
JWT=<user-jwt> bash scripts/acceptance/agent_threads.sh
```

## 常用联调配置

`agents` 至少需要确认以下配置：

- `REDIS_URL=redis://127.0.0.1:6379/0`
- `USER_RPC_TARGET=127.0.0.1:8080`
- `PROGRAM_RPC_TARGET=127.0.0.1:8083`
- `ORDER_RPC_TARGET=127.0.0.1:8082`

## 开发原则

- 核心业务规则进入 `RPC`
- `API` 保持薄层，避免回流核心业务逻辑
- `Gateway` 只做入口、路由与横切能力，不承载业务规则
- 新增 go-zero 服务时，遵循 `services/*-api/` 与 `services/*-rpc/` 的目录规范
