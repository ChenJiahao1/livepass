# 下单主路径验收

本文档用于说明 `scripts/acceptance/order_checkout.sh` 覆盖的主路径验收。

## 关键语义

- `/order/create` 返回 `orderNumber` 只代表请求已进入抢票下单状态机，不代表 Kafka 已发送成功或 DB 已落单。
- 最终结果以 `/order/poll` 为准：排队中 / 成功 / 失败。
- `/order/poll` 在 Redis attempt TTL 期间优先返回状态机投影；attempt miss 后才按 `orderNumber` 查 DB 收敛。

## 执行方式

```bash
bash scripts/acceptance/order_checkout.sh
```

可选环境变量：

- `BASE_URL`：网关地址，默认 `http://127.0.0.1:8081`
- `SHOW_TIME_ID`：验收场次，默认 `30001`
- `TICKET_CATEGORY_ID`：指定票档；未指定时使用预下单返回的第一个票档
- `ORDER_PROGRESS_WAIT_SECONDS`：等待 `/order/poll` 终态的最长秒数，默认 `15`

## 验收步骤

脚本会依次完成：

- 注册并登录测试用户。
- 新增两个观演人并读取预下单详情。
- 申请购买令牌，调用 `/order/create` 获取 `orderNumber`。
- 轮询 `/order/poll`，直到状态到达成功终态。
- 模拟支付、检查支付状态，并查询订单详情确认已分配座位。

## 通过标准

- `/order/create` 返回有效 `orderNumber`。
- `/order/poll` 最终返回 `orderStatus=3` 且 `done=true`。
- `/order/pay` 和 `/order/pay/check` 返回已支付状态。
- `/order/get` 返回订单与观演人票据快照，且票据包含系统分配座位。
