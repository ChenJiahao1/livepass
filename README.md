# damai-go

基于 Go 与 go-zero 的大麦业务总线重建项目，洪峰吞吐优先。

当前已落地能力：

- `user`：注册、登录、用户资料与观演人链路
- `program`：查询链路（category/home/page/detail/ticket-category）、预下单详情、系统自动分配座位冻结
- `order` / `pay`：下单、查单、取消、模拟支付、退款、超时关单
- `gateway` / `agents`：统一 HTTP 入口与 `/agent/chat` 智能客服联调

## 测试目录约束

- 白盒单测保留在被测包旁边，仅用于测试未导出函数、未导出类型和纯内部算法。
- 服务级集成测试统一放在 `services/<service>/tests/`，禁止继续放在 `internal/logic/`、`internal/middleware/`、`internal/config/` 等业务实现目录。
- 跨服务验收测试统一放在根级 `tests/` 或 `scripts/acceptance/`。
- 仅服务内部复用的测试辅助，优先放在 `services/<service>/tests/testkit/`；若只被单个集成测试包使用，可放在 `services/<service>/tests/integration/*_helpers_test.go`。
- 新增或迁移测试时，先遵守本节，再参考 [docs/architecture/testing-layout.md](docs/architecture/testing-layout.md)。

## 本地基础设施启动

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

当前仓库提供的基础设施 compose 已包含 MySQL、Redis、etcd、Kafka。`services/order-rpc/etc/order-rpc.yaml` 的本地 `Kafka.Brokers` 默认指向 `127.0.0.1:9094`，与 compose 暴露端口保持一致。当前下单异步链路语义对齐原 Java 开源版：Kafka 发送失败时立即释放锁座并返回错误；创建消息超过 `Kafka.MaxMessageDelay` 仍未消费时，consumer 会按废单处理并释放锁座，不再补写订单。这不是“消息绝不丢失”的方案。

如需模拟 `order-db-0 + order-db-1` 订单分片库，可使用专用 MySQL compose：

```bash
docker compose -f deploy/mysql/docker-compose.sharding.yml up -d
```

对应端口：

- `order-db-0`: `127.0.0.1:3317`
- `order-db-1`: `127.0.0.1:3318`

## 初始化本地 SQL

MySQL 容器启动后，执行统一导入脚本初始化 user/program/order/pay 域表结构和种子数据：

```bash
bash scripts/import_sql.sh
```

该脚本会显式使用 `mysql --default-character-set=utf8mb4` 读取 SQL 文件，避免 `sql/program/dev_seed.sql` 里的中文文案被错误字符集写坏。

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

如果之前已经按旧命令导入过 `program` 域 SQL，直接重新执行一次该脚本即可重建表并修正中文乱码。

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
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/program-rpc/program.go -f services/program-rpc/etc/program-rpc.yaml
go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay-rpc.yaml
go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.yaml
go run jobs/order-close-worker/order_close_worker.go -f jobs/order-close-worker/etc/order-close-worker.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
go run services/program-api/program.go -f services/program-api/etc/program-api.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.yaml
go run services/pay-api/pay.go -f services/pay-api/etc/pay-api.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml
```

`agents` 组件独立于 go-zero 服务目录，使用 `uv` 启动：

```bash
cd agents
bash scripts/generate_proto_stubs.sh
uv run uvicorn app.main:app --host 0.0.0.0 --port 8891 --reload
```

本地联调时建议启动顺序：

1. 基础设施：MySQL、Redis、etcd、Kafka
2. `user-rpc`、`program-rpc`、`pay-rpc`
3. `order-rpc`
4. `jobs/order-close-worker`
5. `user-api`、`program-api`、`order-api`、`pay-api`
6. `agents`
7. `gateway-api`

`agents` 运行配置可参考 [agents/README.md](agents/README.md) 和 [agents/.env.example](agents/.env.example)，其中至少需要确认：

- `REDIS_URL=redis://127.0.0.1:6379/0`
- `USER_RPC_TARGET=127.0.0.1:8080`
- `PROGRAM_RPC_TARGET=127.0.0.1:8083`
- `ORDER_RPC_TARGET=127.0.0.1:8082`

`jobs/order-close-worker` 负责消费 Asynq 延迟任务，并调用 `order-rpc.CloseExpiredOrder` 做单订单超时关单。当前示例配置复用本地 `127.0.0.1:6379` Redis 便于联调；压测和生产环境应切换到独立 Redis，避免与座位账本、限购账本抢占热点资源。

`jobs/order-close` 仍保留在仓库中，但职责已调整为扫描补偿器：用于回补 Asynq 漏投、漏消费或 Redis 故障期间遗漏的超时订单，不再作为唯一主触发链路。

`user-rpc`、`program-rpc`、`pay-rpc` 与 `order-rpc` 默认注册到本地 `etcd`。`user-rpc` 默认监听 `8080`，`order-rpc` 默认监听 `8082`，`program-rpc` 默认监听 `8083`，`pay-rpc` 默认监听 `8084`。`user-api` 默认监听 `8888`，`program-api` 默认监听 `8889`，`order-api` 默认监听 `8890`，`agents` 默认监听 `8891`，`pay-api` 默认监听 `8892`，`gateway-api` 默认监听 `8081`。`user-rpc` 登录态存储在 `StoreRedis` 指向的 Redis。

`gateway-api` 已启用 `Telemetry` 配置；若要得到完整链路，还需给下游 API/RPC 服务同步补齐 `Telemetry`。

`gateway-api` 当前已透传以下新增兼容接口：

- `POST /order/account/order/count`
- `POST /order/get/cache`
- `POST /pay/common/pay`
- `POST /pay/detail`
- `POST /pay/refund`

## Order 压测基线准备

启动 order baseline 的 perf 配置：

```bash
go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.perf.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.perf.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.perf.yaml
```

Kafka topic 预处理：

```bash
PARTITIONS=4 bash scripts/perf/prepare_order_kafka_topic.sh
```

订单账本预热：

```bash
JWT=<user-jwt> \
PROGRAM_ID=10001 \
TICKET_CATEGORY_ID=40001 \
TICKET_USER_IDS=701,702 \
bash scripts/perf/prewarm_order_ledgers.sh
```

`TICKET_USER_IDS` 使用逗号分隔的观演人 ID 列表，不带方括号。预热脚本会调用 `/order/create`，并轮询 `/order/get` 直到订单可见；如果 `ledger not ready` 或 `order not found` 持续超过阈值，脚本会失败退出。

Prometheus 抓取端点：

- `gateway-api`: `http://127.0.0.1:9101/metrics`
- `order-api`: `http://127.0.0.1:9102/metrics`
- `order-rpc`: `http://127.0.0.1:9103/metrics`
- `pay-api`: `http://127.0.0.1:9104/metrics`

`k6` 基线压测脚本：

```bash
k6 run -e ENV_FILE=scripts/perf/order_create_gateway_baseline.env.example \
  scripts/perf/order_create_gateway_baseline.js
```

可通过 env 文件或命令行覆盖 `JWT`、`PROGRAM_ID`、`TICKET_CATEGORY_ID`、`TICKET_USER_IDS`、`WARMUP_VUS`、`STEADY_VUS` 等参数。`STEADY_START_TIME` 可显式指定稳态阶段启动时间；未设置时默认取 `WARMUP_DURATION + ITERATION_SLEEP_SECONDS + 1s`，避免 warmup 与 steady 在短压测参数下重叠。运行前需本机安装 `k6`。

结果采集：

```bash
BASELINE_START_ORDER_COUNT=<run-before-count> \
SAMPLE_ORDER_NUMBER=<sample-order-number> \
JWT=<user-jwt> \
bash scripts/perf/collect_order_baseline.sh
```

第一版 baseline 通过标准建议至少满足：

- `http_req_failed < 1%`
- `http_req_duration p99 < 10000ms`
- `skip expired order create event` 不持续增长
- 压测停止后 Kafka consumer lag 能回落到接近 `0`

Operator checklist：

1. 启动基础设施：`docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d`
2. 启动 perf 配置：`order-rpc.perf.yaml`、`order-api.perf.yaml`、`gateway-api.perf.yaml`
3. 准备 Kafka topic：`bash scripts/perf/prepare_order_kafka_topic.sh`
4. 预热账本：`bash scripts/perf/prewarm_order_ledgers.sh`
5. 执行 baseline：`k6 run -e ENV_FILE=scripts/perf/order_create_gateway_baseline.env.example scripts/perf/order_create_gateway_baseline.js`
6. 采集结果：`bash scripts/perf/collect_order_baseline.sh`

## 下单主路径验收

完整步骤见 `docs/api/order-checkout-acceptance.md`。
可执行脚本见 `scripts/acceptance/order_checkout.sh`。
首次执行前，先按文档校验 `programId=10001` 的票档和 `d_seat` 座位种子已导入。
创建成功后只代表 Kafka 发送已成功并进入异步落库阶段；`/order/get` 与 `/order/select/list` 允许短时间不可见。当前语义对齐 Java 开源版：发送失败回滚、超时废单回滚。验收脚本会在支付前自动轮询订单可见性。

## 下单失败分支验收

失败分支说明见 `docs/api/order-checkout-failure-acceptance.md`。
可执行脚本见 `scripts/acceptance/order_checkout_failures.sh`。
当前覆盖重复观演人、库存不足、取消后支付失败、超时关单 4 条路径。
失败脚本同样会在需要操作订单前等待异步落库完成，避免把短暂的 `order not found` 误判成业务错误。

## 订单退款主路径验收

完整步骤见 `docs/api/order-refund-acceptance.md`。
可执行脚本见 `scripts/acceptance/order_refund.sh`。
首次执行前，除下单链路依赖外，还需确认 `sql/pay/d_refund_bill.sql` 已导入 `damai_pay`。
默认执行主动退款路径：`REFUND_MODE=proactive bash scripts/acceptance/order_refund.sh`。
补偿退款路径可执行：`REFUND_MODE=compensation bash scripts/acceptance/order_refund.sh`。

当前退款验收覆盖两条链路：

- 用户主动调用 `/order/refund`，订单、支付单和退款单统一收敛到退款终态。
- 订单先取消，再模拟支付晚到，随后通过 `/order/pay/check` 触发补偿退款并收敛到相同终态。

## Agents Chat 验收

`gateway-api` 已透传 `/agent/chat` 到根级 `agents` 服务，并在鉴权成功后注入 `X-User-Id`。

Python 侧 JSON 契约测试：

```bash
cd agents
uv run pytest tests/test_e2e_contract.py -v
```

真实联调脚本：

```bash
JWT=<user-jwt> bash scripts/acceptance/agent_chat.sh
```

该脚本默认通过 `http://127.0.0.1:8081/agent/chat` 访问网关，并覆盖活动咨询、订单查询、退款预检、退款发起和人工转接；其中订单查询到退款预检会复用同一个 `conversationId`，用于验证多轮会话续接。

## 手工验证用户链路

注册：

```bash
curl -X POST http://127.0.0.1:8081/user/register \
  -H 'Content-Type: application/json' \
  -d '{"mobile":"13800000003","password":"123456","confirmPassword":"123456"}'
```

预期响应：

```json
{"success":true}
```

登录：

```bash
curl -X POST http://127.0.0.1:8081/user/login \
  -H 'Content-Type: application/json' \
  -d '{"code":"0001","mobile":"13800000003","password":"123456"}'
```

预期响应包含 `userId` 与 `token`：

```json
{"userId":116260553874210817,"token":"<jwt>"}
```

按 ID 查询用户：

```bash
curl -X POST http://127.0.0.1:8081/user/get/id \
  -H 'Content-Type: application/json' \
  -d '{"id":116260553874210817}'
```

预期返回用户基础信息：

```json
{"id":116260553874210817,"name":"","relName":"","gender":1,"mobile":"13800000003","emailStatus":0,"email":"","relAuthenticationStatus":0,"idNumber":"","address":""}
```

## 手工验证 program Phase 1 链路

查询演出分类：

```bash
curl -X POST http://127.0.0.1:8081/program/category/select/all \
  -H 'Content-Type: application/json' \
  -d '{}'
```

首页分类分组：

```bash
curl -X POST http://127.0.0.1:8081/program/home/list \
  -H 'Content-Type: application/json' \
  -d '{"parentProgramCategoryIds":[1,2]}'
```

分页查询：

```bash
curl -X POST http://127.0.0.1:8081/program/page \
  -H 'Content-Type: application/json' \
  -d '{"parentProgramCategoryId":1,"timeType":0,"pageNumber":1,"pageSize":10,"type":1}'
```

查询演出详情：

```bash
curl -X POST http://127.0.0.1:8081/program/detail \
  -H 'Content-Type: application/json' \
  -d '{"id":10001}'
```

查询演出下票档：

```bash
curl -X POST http://127.0.0.1:8081/ticket/category/select/list/by/program \
  -H 'Content-Type: application/json' \
  -d '{"programId":10001}'
```

预期：以上五个接口都返回 HTTP 200，且能看到 `dev_seed.sql` 中的分类、演出、场次和票档数据。

## 手工验证 program Phase 2 预下单链路

查询预下单详情：

```bash
curl -X POST http://127.0.0.1:8081/program/preorder/detail \
  -H 'Content-Type: application/json' \
  -d '{"id":10001}'
```

冻结预下单座位：

```bash
curl -X POST http://127.0.0.1:8081/program/seat/freeze \
  -H 'Content-Type: application/json' \
  -d '{"programId":10001,"ticketCategoryId":40001,"count":2,"requestNo":"preorder-demo-001","freezeSeconds":900}'
```

预期：

- `/program/preorder/detail` 返回当前演出场次、限购字段、`permitChooseSeat=0`，以及按 `d_seat` 实时聚合的票档余量。
- `/program/seat/freeze` 返回 `freezeToken`、`expireTime` 和系统自动分配的座位列表；当前阶段不支持用户手动选座，系统优先分配同排连坐，找不到连坐时拆座兜底。

## 手工验证 order + pay Phase 1 链路

先登录获取 JWT：

```bash
JWT=$(
  curl -s -X POST http://127.0.0.1:8081/user/login \
    -H 'Content-Type: application/json' \
    -d '{"code":"0001","mobile":"13800000003","password":"123456"}' | jq -r '.token'
)
```

创建订单：

```bash
curl -X POST http://127.0.0.1:8081/order/create \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${JWT}" \
  -H 'X-Channel-Code: 0001' \
  -d '{"programId":10001,"ticketCategoryId":40001,"ticketUserIds":[1001,1002],"distributionMode":"express","takeTicketMode":"paper"}'
```

查询订单列表：

```bash
curl -X POST http://127.0.0.1:8081/order/select/list \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${JWT}" \
  -H 'X-Channel-Code: 0001' \
  -d '{"pageNumber":1,"pageSize":10,"orderStatus":1}'
```

查询订单详情：

```bash
curl -X POST http://127.0.0.1:8081/order/get \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${JWT}" \
  -H 'X-Channel-Code: 0001' \
  -d '{"orderNumber":<orderNumber>}'
```

模拟支付订单：

```bash
curl -X POST http://127.0.0.1:8081/order/pay \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${JWT}" \
  -H 'X-Channel-Code: 0001' \
  -d '{"orderNumber":<orderNumber>,"subject":"大麦演出票","channel":"mock"}'
```

查询支付状态：

```bash
curl -X POST http://127.0.0.1:8081/order/pay/check \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${JWT}" \
  -H 'X-Channel-Code: 0001' \
  -d '{"orderNumber":<orderNumber>}'
```

取消订单：

```bash
curl -X POST http://127.0.0.1:8081/order/cancel \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${JWT}" \
  -H 'X-Channel-Code: 0001' \
  -d '{"orderNumber":<orderNumber>}'
```

预期：

- 所有 order 接口都要求 `Authorization: Bearer <jwt>` 和 `X-Channel-Code: 0001`。
- 创建成功后会返回新的 `orderNumber`，但这只表示 Kafka 发送已成功，不表示订单已经同步落库；短时间内 `/order/get`、`/order/select/list`、`/order/pay`、`/order/cancel` 仍可能返回 `order not found`。
- 当前创建链路对齐 Java 开源版：发送失败时立即回滚锁座；若创建消息延迟超过 `Kafka.MaxMessageDelay`，consumer 会按废单处理并释放锁座，不再补写订单。
- 列表和详情只能看到当前登录用户的订单；如需支付、取消或退款，先轮询到订单可见再继续。
- `/order/pay` 会同步创建模拟支付单、确认冻结座位并把订单状态推进到 `3 paid`。
- `/order/pay/check` 在已支付后会返回支付单号、支付状态和支付时间。
- `/order/refund` 会同步发起模拟退款、释放已售座位并把订单状态推进到 `4 refunded`。
- 支付成功后，再调用 `/order/cancel` 应返回失败，因为已支付订单不能取消。
