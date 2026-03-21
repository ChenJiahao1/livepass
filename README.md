# damai-go

基于 Go 与 go-zero 的大麦业务总线重建项目。

当前阶段：用户域链路已打通，`program` 域 Phase 1 只读链路已补齐（category/home/page/detail/ticket-category）。

## 本地基础设施启动

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

## 初始化用户域表结构

MySQL 容器启动后，执行以下命令导入用户域 SQL：

```bash
for f in sql/user/d_user.sql sql/user/d_user_mobile.sql sql/user/d_user_email.sql sql/user/d_ticket_user.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_user < "$f"
done
```

## 初始化 program 域表结构

`damai_program` 会在 MySQL 首次启动时由 `deploy/mysql/init/01-create-databases.sql` 自动创建。导入 program 域 Phase 1 只读表结构和种子数据：

```bash
for f in \
  sql/program/d_program_category.sql \
  sql/program/d_program_group.sql \
  sql/program/d_program.sql \
  sql/program/d_program_show_time.sql \
  sql/program/d_seat.sql \
  sql/program/d_seat_freeze.sql \
  sql/program/d_ticket_category.sql \
  sql/program/dev_seed.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_program < "$f"
done
```

## 初始化 order 域表结构

`damai_order` 会在 MySQL 首次启动时由 `deploy/mysql/init/01-create-databases.sql` 自动创建。导入订单域表结构：

```bash
for f in sql/order/d_order.sql sql/order/d_order_ticket_user.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_order < "$f"
done
```

## 初始化 pay 域表结构

`damai_pay` 会在 MySQL 首次启动时由 `deploy/mysql/init/01-create-databases.sql` 自动创建。导入支付域表结构：

```bash
docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_pay < sql/pay/d_pay_bill.sql
```

## 运行测试

```bash
go test ./...
```

## 启动服务

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
go run services/program-rpc/program.go -f services/program-rpc/etc/program-rpc.yaml
go run services/program-api/program.go -f services/program-api/etc/program-api.yaml
go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay-rpc.yaml
go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml
go run jobs/order-close/order_close.go -f jobs/order-close/etc/order-close.yaml
```

`user-rpc`、`program-rpc`、`pay-rpc` 与 `order-rpc` 默认注册到本地 `etcd`。`user-rpc` 默认监听 `8080`，`order-rpc` 默认监听 `8082`，`program-rpc` 默认监听 `8083`，`pay-rpc` 默认监听 `8084`。`user-api` 默认监听 `8888`，`program-api` 默认监听 `8889`，`order-api` 默认监听 `8890`，`gateway-api` 默认监听 `8081`。`user-rpc` 登录态存储在 `StoreRedis` 指向的 Redis。

`gateway-api` 已启用 `Telemetry` 配置；若要得到完整链路，还需给下游 API/RPC 服务同步补齐 `Telemetry`。

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
- `/program/seat/freeze` 返回 `freezeToken`、`expireTime` 和系统自动分配的座位列表；当前阶段不支持用户手动选座。

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
- 创建成功后会返回新的 `orderNumber`，列表和详情只能看到当前登录用户的订单。
- `/order/pay` 会同步创建模拟支付单、确认冻结座位并把订单状态推进到 `3 paid`。
- `/order/pay/check` 在已支付后会返回支付单号、支付状态和支付时间。
- 支付成功后，再调用 `/order/cancel` 应返回失败，因为已支付订单不能取消。
