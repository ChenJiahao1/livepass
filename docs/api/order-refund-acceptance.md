# 订单退款主路径验收

## 前置条件

- 默认网关地址：`http://127.0.0.1:8081`
- 默认渠道码：`0001`
- 默认验收节目：`programId=10001`
- 已完成 [下单主路径验收](/home/chenjiahao/code/project/damai-go/docs/api/order-checkout-acceptance.md) 中的基础设施、SQL 导入和服务启动步骤
- 本机可用 `curl`、`jq`、`docker`、`go`
- `damai_pay` 已导入 `sql/pay/d_pay_bill.sql` 与 `sql/pay/d_refund_bill.sql`

直接执行脚本：

```bash
bash scripts/acceptance/order_refund.sh
```

脚本默认读取：

- `BASE_URL`
- `CHANNEL_CODE`
- `PROGRAM_ID`
- `PASSWORD`
- `TICKET_CATEGORY_ID`，可选；默认读取 `/program/preorder/detail` 首个票档
- `REFUND_REASON`，默认 `行程变更`
- `MYSQL_CONTAINER`，默认 `docker-compose-mysql-1`
- `MYSQL_ROOT_PASSWORD`，默认 `123456`

## 验收目标

本轮退款验收需要同时确认：

- 能通过 `gateway-api` 调用 `/order/refund`
- 退款后订单状态变为 `4 refunded`
- 支付单状态变为 `3 refunded`
- `d_refund_bill` 成功落库且 `refundBillNo > 0`
- 同一票档的可售库存相较退款前恢复 2 张

## 验收步骤

1. 复用下单主路径，自动完成：
   - 注册用户
   - 登录
   - 新增两个观演人
   - 查询预下单详情
   - 创建订单
   - 支付订单
2. 记录退款前票档余量：

```bash
curl -sS -X POST "${BASE_URL}/program/preorder/detail" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":${PROGRAM_ID}}" | jq
```

3. 调用 `/order/refund`：

```bash
curl -sS -X POST "${BASE_URL}/order/refund" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Channel-Code: ${CHANNEL_CODE}" \
  -d "{\"orderNumber\":${ORDER_NUMBER},\"reason\":\"${REFUND_REASON}\"}" | jq
```

成功判定：

- `orderNumber == ORDER_NUMBER`
- `orderStatus == 4`
- `refundAmount > 0`
- `refundPercent > 0`
- `refundBillNo > 0`
- `refundTime` 非空

4. 调用 `/order/pay/check`：

```bash
curl -sS -X POST "${BASE_URL}/order/pay/check" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Channel-Code: ${CHANNEL_CODE}" \
  -d "{\"orderNumber\":${ORDER_NUMBER}}" | jq
```

成功判定：

- `orderStatus == 4`
- `payStatus == 3`

5. 调用 `/order/get`：

```bash
curl -sS -X POST "${BASE_URL}/order/get" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Channel-Code: ${CHANNEL_CODE}" \
  -d "{\"orderNumber\":${ORDER_NUMBER}}" | jq
```

成功判定：

- `orderStatus == 4`
- `orderTicketInfoVoList` 长度仍为 `2`

6. 直接查询支付库确认退款单落库：

```bash
docker exec "${MYSQL_CONTAINER:-docker-compose-mysql-1}" \
  mysql -N -uroot -p"${MYSQL_ROOT_PASSWORD:-123456}" damai_pay \
  -e "SELECT pay_status FROM d_pay_bill WHERE order_number = ${ORDER_NUMBER};"

docker exec "${MYSQL_CONTAINER:-docker-compose-mysql-1}" \
  mysql -N -uroot -p"${MYSQL_ROOT_PASSWORD:-123456}" damai_pay \
  -e "SELECT refund_bill_no, refund_amount, refund_status FROM d_refund_bill WHERE order_number = ${ORDER_NUMBER};"
```

成功判定：

- `d_pay_bill.pay_status == 3`
- `d_refund_bill` 返回 1 行
- `refund_bill_no > 0`
- `refund_status == 2`

7. 再次查询 `/program/preorder/detail`，确认余量恢复：

```bash
curl -sS -X POST "${BASE_URL}/program/preorder/detail" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":${PROGRAM_ID}}" | jq
```

成功判定：

- 退款后同一 `ticketCategoryId` 的 `remainNumber == remain_before_refund + 2`

## 预期成功标记

脚本成功结束时会输出：

- `ORDER_NUMBER`
- `REFUND_BILL_NO`
- `REFUND_AMOUNT`
- `REMAIN_BEFORE_REFUND`
- `REMAIN_AFTER_REFUND`

并打印 `退款主路径执行成功`。
