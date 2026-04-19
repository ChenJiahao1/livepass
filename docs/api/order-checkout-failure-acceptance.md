# 下单失败分支验收

本文档用于说明 `scripts/acceptance/order_checkout_failures.sh` 覆盖的失败分支验收。

## 关键语义

- `/order/create` 返回 `orderNumber` 只代表请求已进入抢票下单状态机，不代表 Kafka 已发送成功或 DB 已落单。
- 最终结果以 `/order/poll` 为准：排队中 / 成功 / 失败。
- Redis attempt 处于未知或处理中时不会被直接投影为失败；明确失败才写 `FAILED` 并按状态机规则回补。

## 执行方式

```bash
bash scripts/acceptance/order_checkout_failures.sh
```

可选环境变量：

- `BASE_URL`：网关地址，默认 `http://127.0.0.1:8081`
- `SHOW_TIME_ID`：验收场次，默认 `30001`
- `MYSQL_CONTAINER`：MySQL 容器名，默认 `docker-compose-mysql-1`
- `ORDER_CLOSE_CONFIG`：超时关单 dispatcher 配置，默认 `jobs/order-close/etc/order-close-dispatcher.yaml`
- `INVENTORY_FAIL_TICKET_CATEGORY_ID`：指定库存不足场景使用的票档

## 验收场景

脚本会复用主路径注册、登录、观演人和预下单步骤，并覆盖：

- 重复观演人：新增相同证件观演人应返回失败。
- 库存不足：无可用座位时购买令牌或下单应失败；若已进入状态机，则通过 `/order/poll` 收敛为失败。
- 取消订单：成功落单后取消，后续支付应因订单状态非法失败。
- 超时关单：成功落单后推进过期，关单任务应释放库存并使后续支付失败。

## 通过标准

- 明确业务失败返回对应错误或通过 `/order/poll` 收敛为 `orderStatus=4`。
- 取消和超时关单后的订单状态为已取消。
- 超时关单后预下单库存恢复到关单前数量。
