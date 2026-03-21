# 下单主路径验收

## 前置条件

- 默认网关地址：`http://127.0.0.1:8081`
- 默认渠道码：`0001`
- 默认验收节目：`programId=10001`
- `order-rpc` 现在依赖 Kafka；基础设施 compose 已包含 Kafka broker，本地默认通过 `127.0.0.1:9094` 暴露，并与 `services/order-rpc/etc/order-rpc.yaml` 中的 `Kafka.Brokers` 对齐
- 本文档所有 HTTP 请求都只经过 `gateway-api`，不直接访问 `user-api`、`program-api`、`order-api` 或任何 RPC 服务。
- 本文档默认本地安装 `curl`、`jq`、`docker`、`go`。
- 建议先准备本次执行使用的环境变量：

```bash
export BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
export CHANNEL_CODE="${CHANNEL_CODE:-0001}"
export PROGRAM_ID="${PROGRAM_ID:-10001}"
export PASSWORD="${PASSWORD:-123456}"

RUN_ID="$(date +%s)"
SUFFIX="$(printf '%08d' "$((RUN_ID % 100000000))")"
export MOBILE="139${SUFFIX}"
```

如需直接执行脚本：

```bash
bash scripts/acceptance/order_checkout.sh
```

失败分支单独见：

```bash
bash scripts/acceptance/order_checkout_failures.sh
```

脚本默认读取：

- `BASE_URL`
- `CHANNEL_CODE`
- `PROGRAM_ID`
- `TICKET_CATEGORY_ID`，可选；默认从 `/program/preorder/detail` 的首个票档解析

脚本也支持额外覆盖：

- `MOBILE`
- `PASSWORD`
- `RUN_ID`
- `ORDER_VISIBLE_WAIT_SECONDS`，创建后等待订单可见的最长秒数，默认 `15`

脚本成功时会打印：

- `USER_ID`
- `TOKEN`
- `TICKET_USER_ID_1`
- `TICKET_USER_ID_2`
- `TICKET_CATEGORY_ID`
- `ORDER_NUMBER`

脚本失败时会在首个失败步骤立即停止；若依赖缺失，会先在预检阶段退出；若服务未就绪，则会在真实网络请求或关键字段提取处停止。

## 启动基础设施

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml ps
```

成功判定：

- `mysql`、`redis`、`etcd`、`kafka` 容器状态均为 `Up`
- 没有持续重启或 `Exited` 的基础设施容器
- Kafka broker 可通过 `127.0.0.1:9094` 连通，且 `order-rpc` 所用 topic / consumer group 可正常连通

## 导入 SQL

执行仓库统一导入脚本，一次性导入用户、节目、订单、支付域表结构和种子数据：

```bash
bash scripts/import_sql.sh
```

该脚本会显式使用 `mysql --default-character-set=utf8mb4` 读取 SQL 文件，避免 `program` 域种子数据里的中文字段写成乱码。若此前已按旧命令导入过 `program` 域 SQL，请重新执行一次脚本以重建表数据。

建议补一组快速校验：

```bash
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT COUNT(*) AS total FROM damai_program.d_program WHERE id = 10001;"
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT title, place FROM damai_program.d_program WHERE id = 10001;"
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT COUNT(*) AS total FROM damai_program.d_ticket_category WHERE program_id = 10001;"
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT id, introduce FROM damai_program.d_ticket_category WHERE program_id = 10001 ORDER BY id;"
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT ticket_category_id, COUNT(*) AS total FROM damai_program.d_seat WHERE program_id = 10001 AND status = 1 AND seat_status = 1 GROUP BY ticket_category_id ORDER BY ticket_category_id;"
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT COUNT(*) AS total FROM damai_order.d_order;"
docker exec docker-compose-mysql-1 mysql -uroot -p123456 -e "SELECT COUNT(*) AS total FROM damai_pay.d_pay_bill;"
```

成功判定：

- `damai_program.d_program` 中能查到 `id=10001`
- `damai_program.d_program` 中 `title=Phase1 示例演出`、`place=北京示例剧场`，没有中文乱码
- `damai_program.d_ticket_category` 中能查到 `program_id=10001` 的票档
- `damai_program.d_ticket_category` 中 `id=40001` 的 `introduce=普通票`、`id=40002` 的 `introduce=VIP票`
- `damai_program.d_seat` 中能查到 `ticket_category_id=40001` 有 `100` 个可售座位、`ticket_category_id=40002` 有 `80` 个可售座位
- `damai_order.d_order` 与 `damai_pay.d_pay_bill` 可正常查询

## 启动服务

分别启动以下服务：

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
go run services/program-rpc/program.go -f services/program-rpc/etc/program-rpc.yaml
go run services/program-api/program.go -f services/program-api/etc/program-api.yaml
go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay-rpc.yaml
go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml
```

成功判定：

- 所有进程都能保持运行，不出现启动即退出
- `gateway-api` 监听 `8081`
- 下游 API/RPC 能完成注册到本地 `etcd`

## 执行主路径

以下步骤默认都使用：

```bash
COMMON_HEADERS=(-H 'Content-Type: application/json')
ORDER_HEADERS=(-H 'Content-Type: application/json' -H "Authorization: Bearer ${TOKEN}" -H "X-Channel-Code: ${CHANNEL_CODE}")
```

1. 注册用户

```bash
curl -sS -X POST "${BASE_URL}/user/register" \
  "${COMMON_HEADERS[@]}" \
  -d "{\"mobile\":\"${MOBILE}\",\"password\":\"${PASSWORD}\",\"confirmPassword\":\"${PASSWORD}\"}" | jq
```

成功判定：

- 返回 JSON 中 `success=true`

2. 登录并提取 `userId` / `token`

```bash
LOGIN_RESP="$(
  curl -sS -X POST "${BASE_URL}/user/login" \
    "${COMMON_HEADERS[@]}" \
    -d "{\"code\":\"${CHANNEL_CODE}\",\"mobile\":\"${MOBILE}\",\"password\":\"${PASSWORD}\"}"
)"

export USER_ID="$(printf '%s' "${LOGIN_RESP}" | jq -er '.userId')"
export TOKEN="$(printf '%s' "${LOGIN_RESP}" | jq -er '.token')"

printf '%s\n' "${LOGIN_RESP}" | jq
printf 'USER_ID=%s\nTOKEN=%s\n' "${USER_ID}" "${TOKEN}"
```

成功判定：

- `userId` 为非空数字
- `token` 为非空字符串

3. 新增两个观演人

```bash
curl -sS -X POST "${BASE_URL}/ticket/user/add" \
  "${COMMON_HEADERS[@]}" \
  -d "{\"userId\":${USER_ID},\"relName\":\"张三\",\"idType\":1,\"idNumber\":\"110101199001011234\"}" | jq

curl -sS -X POST "${BASE_URL}/ticket/user/add" \
  "${COMMON_HEADERS[@]}" \
  -d "{\"userId\":${USER_ID},\"relName\":\"李四\",\"idType\":1,\"idNumber\":\"110101199202021234\"}" | jq
```

成功判定：

- 两次响应都返回 `success=true`

4. 查询用户和观演人列表

```bash
USER_TICKET_RESP="$(
  curl -sS -X POST "${BASE_URL}/user/get/user/ticket/list" \
    "${COMMON_HEADERS[@]}" \
    -d "{\"userId\":${USER_ID}}"
)"

export TICKET_USER_ID_1="$(printf '%s' "${USER_TICKET_RESP}" | jq -er '.ticketUserVoList[0].id')"
export TICKET_USER_ID_2="$(printf '%s' "${USER_TICKET_RESP}" | jq -er '.ticketUserVoList[1].id')"

printf '%s\n' "${USER_TICKET_RESP}" | jq
printf 'TICKET_USER_ID_1=%s\nTICKET_USER_ID_2=%s\n' "${TICKET_USER_ID_1}" "${TICKET_USER_ID_2}"
```

成功判定：

- `ticketUserVoList` 至少有 2 个元素
- 能提取出两个非空 `ticketUserId`

5. 查询 `/program/preorder/detail`

```bash
PREORDER_RESP="$(
  curl -sS -X POST "${BASE_URL}/program/preorder/detail" \
    "${COMMON_HEADERS[@]}" \
    -d "{\"id\":${PROGRAM_ID}}"
)"

export TICKET_CATEGORY_ID="$(printf '%s' "${PREORDER_RESP}" | jq -er '.ticketCategoryVoList[0].id')"

printf '%s\n' "${PREORDER_RESP}" | jq
printf 'TICKET_CATEGORY_ID=%s\n' "${TICKET_CATEGORY_ID}"
```

成功判定：

- 返回 `id=10001`
- `ticketCategoryVoList` 非空
- 能提取到非空 `ticketCategoryId`
- `permitChooseSeat=0`

6. 调用 `/order/create`

```bash
CREATE_ORDER_RESP="$(
  curl -sS -X POST "${BASE_URL}/order/create" \
    "${ORDER_HEADERS[@]}" \
    -d "{\"programId\":${PROGRAM_ID},\"ticketCategoryId\":${TICKET_CATEGORY_ID},\"ticketUserIds\":[${TICKET_USER_ID_1},${TICKET_USER_ID_2}],\"distributionMode\":\"express\",\"takeTicketMode\":\"paper\"}"
)"

export ORDER_NUMBER="$(printf '%s' "${CREATE_ORDER_RESP}" | jq -er '.orderNumber')"

printf '%s\n' "${CREATE_ORDER_RESP}" | jq
printf 'ORDER_NUMBER=%s\n' "${ORDER_NUMBER}"
```

成功判定：

- 返回非空 `orderNumber`
- 这里的成功仅表示 `order-rpc` 已完成锁座并把创建指令写入 Kafka，不保证订单已经同步落库

7. 轮询 `/order/get`，等待订单异步可见

```bash
until curl -sS -X POST "${BASE_URL}/order/get" \
  "${ORDER_HEADERS[@]}" \
  -d "{\"orderNumber\":${ORDER_NUMBER}}" | jq -e ".orderNumber == ${ORDER_NUMBER}" >/dev/null 2>&1; do
  sleep 1
done
```

成功判定：

- 在合理等待窗口内最终能查询到目标订单
- 在订单真正落库前，`/order/get` 返回 `order not found` 视为允许行为
- 同一窗口内，`/order/select/list`、`/order/pay`、`/order/cancel` 也可能暂时返回订单不存在

8. 调用 `/order/pay`

```bash
curl -sS -X POST "${BASE_URL}/order/pay" \
  "${ORDER_HEADERS[@]}" \
  -d "{\"orderNumber\":${ORDER_NUMBER},\"subject\":\"大麦演出票\",\"channel\":\"mock\"}" | jq
```

成功判定：

- 返回中的 `orderNumber` 等于刚创建的订单
- `payBillNo` 为非空
- `payStatus` 为已支付状态

9. 调用 `/order/pay/check`

```bash
curl -sS -X POST "${BASE_URL}/order/pay/check" \
  "${ORDER_HEADERS[@]}" \
  -d "{\"orderNumber\":${ORDER_NUMBER}}" | jq
```

成功判定：

- 返回中的 `orderNumber` 等于目标订单
- `payStatus` 仍为已支付状态

10. 调用 `/order/get`

```bash
curl -sS -X POST "${BASE_URL}/order/get" \
  "${ORDER_HEADERS[@]}" \
  -d "{\"orderNumber\":${ORDER_NUMBER}}" | jq
```

成功判定：

- 返回中的 `orderNumber` 等于目标订单
- `orderTicketInfoVoList` 包含 2 条观演人快照
- 每条观演人快照都包含系统分配的 `seatId` / `seatRow` / `seatCol`
- `orderStatus` 为已支付

## 成功判定

- 能从零开始完成注册、登录、观演人新增、预下单详情查询、下单、模拟支付、支付校验、订单详情校验全链路
- 主路径中不需要手填 `userId`、`ticketUserId`、`ticketCategoryId`、`orderNumber`
- 所有订单相关接口都显式携带：
  - `Authorization: Bearer <token>`
  - `X-Channel-Code: 0001`
- `/order/create` 成功后允许存在短暂异步窗口，只有轮询到 `/order/get` 可见后，才继续支付或取消类操作
- 最终订单详情里可见：
  - 2 个观演人
  - 已分配座位
  - 已支付状态

补充说明：

- 当前项目不支持用户自主选座。
- 系统分配座位时优先同排连坐；如果当前余座无法满足连坐，会自动拆座兜底。
- 创建消息如果超过 `Kafka.MaxMessageDelay` 仍未被消费，consumer 会主动释放锁座，不再补写订单。

一次已验证通过的成功标记示例：

```text
/program/preorder/detail:
- ticketCategoryVoList[0].id = 40001
- ticketCategoryVoList[0].remainNumber = 100
- ticketCategoryVoList[1].id = 40002
- ticketCategoryVoList[1].remainNumber = 80

/order/pay:
- orderStatus = 3
- payStatus = 2
- payBillNo > 0

/order/get:
- ticketCount = 2
- orderStatus = 3
- orderTicketInfoVoList[0].seatRow = 1
- orderTicketInfoVoList[0].seatCol = 1
- orderTicketInfoVoList[1].seatRow = 1
- orderTicketInfoVoList[1].seatCol = 2
```

## 常见失败点

- `gateway-api` 可用，但下游 API/RPC 未启动或未注册到 `etcd`
- Kafka broker 未启动，或 `services/order-rpc/etc/order-rpc.yaml` 的 `Kafka.Brokers`（本地默认 `127.0.0.1:9094`）不可达，导致 `/order/create` 返回内部错误
- MySQL 已启动，但 `programId=10001` 的节目、票档或座位种子数据未导入
- `damai_program.d_seat` 没有为 `programId=10001` 导入座位行，导致 `/program/preorder/detail` 的 `remainNumber=0`，随后 `/order/create` 返回 `seat inventory insufficient`
- 订单请求缺少 `Authorization` 或 `X-Channel-Code`
- 观演人 ID 手写，导致 `/order/create` 返回观演人归属校验失败
- 创建成功后立刻调用 `/order/get`、`/order/pay`、`/order/cancel`，可能命中短暂的订单不可见窗口
- `jq -e` 提取字段失败，说明前序接口没有返回预期结构
