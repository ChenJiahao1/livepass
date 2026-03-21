# 下单失败分支验收

## 范围

本次补齐 4 条失败分支：

1. 重复观演人
2. 库存不足
3. 订单取消后再次支付
4. 超时关单

脚本入口：

```bash
bash scripts/acceptance/order_checkout_failures.sh
```

## 前置条件

- 基础设施、SQL 与 `gateway-api`/各下游服务已按 [下单主路径验收](/home/chenjiahao/code/project/damai-go/docs/api/order-checkout-acceptance.md) 启动
- 本机可用 `curl`、`jq`、`docker`、`go`
- MySQL 仍通过容器 `docker-compose-mysql-1` 暴露，默认 root 密码为 `123456`
- `order-rpc` 已启动；脚本会自行触发一次 `jobs/order-close/order_close.go` 来验证超时关单，不要求常驻启动 job

## 默认环境变量

```bash
export BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
export CHANNEL_CODE="${CHANNEL_CODE:-0001}"
export PROGRAM_ID="${PROGRAM_ID:-10001}"
export PASSWORD="${PASSWORD:-123456}"
export MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"
```

可选覆盖：

- `INVENTORY_FAIL_TICKET_CATEGORY_ID`：库存不足场景使用的票档；默认优先取预下单详情里的第 2 个票档，否则回退到第 1 个
- `ORDER_CLOSE_CONFIG`：超时关单 job 的配置文件路径
- `ORDER_CLOSE_WAIT_SECONDS`：等待 `order-close` 首次执行完成的秒数，默认 `30`

## 脚本行为

脚本会先复用主路径前半段能力，自动完成：

- 注册
- 登录
- 添加两个观演人
- 查询观演人列表
- 查询预下单详情

然后顺序执行以下失败场景：

### 1. 重复观演人

- 再次调用 `/ticket/user/add`
- 期望响应体包含 `ticket user already exists`

### 2. 库存不足

- 通过 MySQL 临时把目标票档当前可售座位标记为不可售
- 重新查询 `/program/preorder/detail`，确认该票档 `remainNumber=0`
- 调用 `/order/create`
- 期望响应体包含 `seat inventory insufficient`
- 脚本结束前会自动恢复本次临时改动的座位状态

### 3. 订单取消后再次支付

- 正常创建一笔未支付订单
- 调用 `/order/cancel`
- 再调 `/order/get`，期望 `orderStatus=2`
- 再调 `/order/pay`
- 期望响应体包含 `order status invalid`

### 4. 超时关单

- 再创建一笔未支付订单
- 通过 MySQL 把该订单的 `order_expire_time` 改到当前时间之前
- 启动一次 `go run jobs/order-close/order_close.go -f jobs/order-close/etc/order-close.yaml`
- 再调 `/order/get`，期望 `orderStatus=2`
- 再调 `/order/pay`
- 期望响应体包含 `order status invalid`
- 再查一次 `/program/preorder/detail`，期望对应票档余量恢复到创建订单前

## 通过标准

- 脚本最终打印 `失败分支执行成功`
- 4 个失败场景都命中预期错误语义
- 库存不足场景的临时座位变更被恢复
- 超时关单场景中，被关闭订单对应的座位余量恢复
