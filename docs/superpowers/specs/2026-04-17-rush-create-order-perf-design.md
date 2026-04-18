# 抢票核心链路压测设计（`create order` 起压）

## 1. 背景

本次目标是在远端压测环境中，对 `同一场次 + 同一票档` 的抢票核心链路进行超卖竞争型压测。

压测流量不覆盖登录、注册、加观演人等前置链路，而是提前准备好：

- 压测用户
- 每个用户的观演人
- 每个用户对应的 `purchase token`
- 指定场次 / 票档的库存与座位缓存

压测时仅从 `POST /order/create` 开始，随后轮询 `POST /order/poll` 收敛结果。

## 2. 目标

### 2.1 业务目标

- 固定压测范围为 `单 showTimeId + 单 ticketCategoryId`
- 支持通过配置控制压测用户数与库存票数
- 支持每个用户随机购买 `1-3` 张票
- 默认采用超卖竞争模型：
  - `seatCount` 不传时，默认等于 `userCount`
  - 因为随机票数均值约为 `2`，所以总需求期望约为 `2 x seatCount`

### 2.2 技术目标

- 避免压测期间为每次请求生成 JWT
- 避免将正常联调环境的鉴权逻辑全局关闭
- 压测数据、预发 token、压测结果可重复生成
- 输出一组可直接用于简历和复盘的性能指标

## 3. 非目标

- 不压登录、注册、观演人新增链路
- 不覆盖多场次、多票档混合竞争
- 不在本阶段引入手动选座压测
- 不把压测模式作为常规业务模式对外暴露

## 4. 方案对比

### 方案 A：压测模式免 JWT + 预置 `purchase token`（推荐）

做法：

- 在 `gateway-api` 增加仅压测配置生效的免鉴权能力
- 仅对压测链路请求生效，正常路径保持现状
- 通过脚本批量生成用户、观演人、`purchase token`
- k6 直接调用 `/order/create` 和 `/order/poll`

优点：

- 压测入口稳定，脚本简单
- 对正常联调影响最小
- 便于长期复用和写入简历

缺点：

- 需要新增压测模式配置与专用头部/白名单逻辑

### 方案 B：保留 JWT，批量预生成 JWT + `purchase token`

优点：

- 更接近真实用户请求

缺点：

- 前置准备更复杂
- k6 数据装载和 token 生命周期管理更重
- 排障成本更高

### 方案 C：全局禁用 JWT

优点：

- 最快

缺点：

- 改动污染面过大
- 容易误伤正常联调
- 风险与收益不成比例

### 结论

采用 **方案 A**。

## 5. 总体设计

本次改造拆成五块：

1. 压测模式鉴权
2. 场次 / 票档 / 座位造数
3. 压测用户与观演人造数
4. `purchase token` 预发与导出
5. k6 压测脚本与结果汇总

### 5.1 链路形态

```text
prepare_data
  -> create users
  -> create ticket users
  -> create seats and stock
  -> preheat seat ledger / admission quota
  -> issue purchase tokens
  -> export perf dataset

k6
  -> POST /order/create
  -> POST /order/poll
  -> summarize metrics
  -> verify sold seats / orders / guards
```

## 6. 压测模式鉴权设计

### 6.1 目标

不关闭现有 JWT 体系，只给压测数据导入和压测执行提供一个可控的“压测身份注入”入口。

### 6.2 做法

在 `gateway-api` 的鉴权中新增 `PerfMode` 配置：

- `Enabled`
- `HeaderName`
- `HeaderSecret`
- `AllowedPaths`

当 `PerfMode.Enabled=true` 时：

- 若请求命中 `AllowedPaths`
- 且请求头携带正确的压测密钥
- 则允许通过压测头直接注入 `userId`
- 网关继续按内部网关身份头转发到下游

否则：

- 仍走原有 JWT 鉴权逻辑

### 6.3 边界

- 只允许压测专用路径使用压测头
- 压测头仅在压测配置文件中开启
- 默认业务配置保持关闭
- 不修改下游 `user-api` / `order-api` / `order-rpc` 的核心业务语义

### 6.4 推荐允许路径

- `POST /order/create`
- `POST /order/poll`
- 如需脚本化补充验证，可选：
  - `POST /order/get`
  - `POST /order/select/list`

## 7. 数据准备设计

### 7.1 场次与票档

压测固定针对：

- 单个 `showTimeId`
- 单个 `ticketCategoryId`

建议新增专用压测造数脚本，而不是继续扩展 `sql/program/dev_seed.sql`。

原因：

- `dev_seed.sql` 适合初始化演示数据
- 压测数据规模大、参数化需求强
- 不应把大量压测座位固化进默认 seed

### 7.2 座位库存

造数脚本支持：

- `showTimeId`
- `ticketCategoryId`
- `seatCount`
- `rowCount`
- `colCount`
- `seatPrice`

默认规则：

- `seatCount` 不传时，取 `userCount`
- 若未显式指定排列，默认按接近方阵分布
- 对你当前目标，允许显式配置：
  - `rowCount=50`
  - `colCount=100`
  - `seatCount=5000`

写库规则：

- 重建指定票档已有座位
- 重置 `d_ticket_category.total_number`
- 重置 `d_ticket_category.remain_number`
- 将该票档全部座位写入 `d_seat`
- 所有座位初始为 `seat_status=1`

### 7.3 用户与观演人

造数脚本支持：

- `userCount`
- `ticketUsersPerUser`

默认规则：

- 每个压测用户固定创建 `3` 个观演人
- 保证随机购买 `1-3` 张时，始终有足够观演人可绑定

用户数据建议打上压测前缀：

- mobile 前缀
- relName 前缀
- 可选 remark / tag

方便：

- 清理
- 对账
- 结果归因

### 7.4 购买张数模型

每个压测用户在数据集中预先确定一个随机 `ticketCount in [1,3]`。

原因：

- 便于 `purchase token` 预发
- 保证 k6 执行阶段只做读取，不做额外前置接口调用
- 同一轮压测结果可复现

随机策略：

- 使用固定随机种子生成
- 将 `ticketCount` 固化写入导出数据集

## 8. `purchase token` 预发设计

### 8.1 原则

因为压测从 `create order` 开始，所以每个压测用户在开压前都必须拥有：

- 已确定的 `ticketUserIds`
- 已确定的随机购票张数
- 对应的 `purchaseToken`

### 8.2 预发流程

批量执行：

1. 为压测用户挑选前 `ticketCount` 个观演人
2. 调用 `POST /order/purchase/token`
3. 获取 `purchaseToken`
4. 导出压测数据行

每条导出记录至少包含：

- `userId`
- `showTimeId`
- `ticketCategoryId`
- `ticketCount`
- `ticketUserIds`
- `purchaseToken`

### 8.3 导出格式

建议同时导出两份：

- `JSON`：供 k6 直接读取
- `CSV`：供人查看和结果分析

输出目录建议：

- `tmp/perf/<dataset-id>/users.json`
- `tmp/perf/<dataset-id>/users.csv`
- `tmp/perf/<dataset-id>/meta.json`

`meta.json` 记录：

- 生成时间
- showTimeId
- ticketCategoryId
- userCount
- seatCount
- rowCount
- colCount
- randomSeed

## 9. 库存预热设计

压测前必须预热：

- `order-rpc` admission quota
- `program-rpc` seat ledger

优先复用现有入口：

- `scripts/prime_rush_inventory_tmp.sh`

若后续发现需要更稳定的批量场景入口，再补正式压测脚本包装。

## 10. k6 压测设计

### 10.1 输入

k6 读取预发好的数据集，按行使用：

- `userId`
- `purchaseToken`
- `ticketCount`

### 10.2 请求头

使用压测模式头注入 `userId`，不走 JWT。

例如：

- `X-LivePass-Perf-Secret`
- `X-LivePass-Perf-User-Id`

实际名称以配置文件为准。

### 10.3 压测流程

每个虚拟用户执行：

1. 使用本行 `purchaseToken` 调用 `POST /order/create`
2. 若成功返回 `orderNumber`
3. 轮询 `POST /order/poll` 直到：
   - `done=true`
   - 或超时
4. 记录结果：
   - success
   - processing timeout
   - inventory insufficient
   - duplicate / account limited / other business reject

### 10.4 并发模型

至少支持两种执行方式：

- 一次性洪峰
- 阶梯升压

推荐首版先支持：

- `shared-iterations`
- `ramping-vus`

### 10.5 压测参数

建议支持：

- `datasetPath`
- `baseUrl`
- `pollIntervalMs`
- `pollTimeoutMs`
- `vus`
- `stages`
- `batchSize`

## 11. 结果统计设计

### 11.1 k6 原生指标

输出：

- 请求总数
- 平均 RPS
- 峰值 RPS
- `http_req_duration` 的 `p50/p90/p95/p99`
- `create order` 成功率
- `poll` 收敛成功率

### 11.2 业务指标

压测脚本汇总：

- 总用户数
- 总库存票数
- 总需求票数
- 竞争倍率 = 总需求票数 / 总库存票数
- 成功订单数
- 成功购票人数
- 成功售出票数
- 失败订单数
- 库存不足失败数
- 其他失败数
- 库存售罄时间

### 11.3 一致性校验

压测结束后增加结果校验脚本，对账以下指标：

- `d_seat` 中 `seat_status=3` 的数量
- 订单明细已占座数量
- `d_order_seat_guard` 数量
- `d_ticket_category.remain_number`

要求：

- 已售座位数一致
- 剩余库存与售出票数能闭环
- 不出现明显超卖

### 11.4 简历可用指标

最终输出一份摘要报告，建议直接包含：

- 场次 / 票档 / 库存
- 用户数 / 总需求票数 / 竞争倍率
- 峰值 RPS / 平均 RPS
- `create order` 成功率
- `poll` 收敛成功率
- `p95/p99`
- 库存售罄耗时
- 最终一致性校验结果

## 12. 实现拆分

### 12.1 网关

涉及：

- `services/gateway-api/internal/middleware/auth_middleware.go`
- `services/gateway-api/etc/gateway-api.perf.yaml`
- 相关 config struct / config loader

职责：

- 增加压测模式头鉴权
- 将压测 `userId` 转换成内部身份头

### 12.2 造数脚本

建议新增：

- `scripts/perf/prepare_rush_perf_dataset.sh`
- `scripts/perf/prepare_rush_perf_dataset.sql`
- 或 `scripts/perf/prepare_rush_perf_dataset.go`

职责：

- 创建 / 重建压测用户
- 创建观演人
- 创建指定票档座位
- 重置库存
- 预热缓存
- 预发 `purchase token`
- 导出数据集

建议优先使用 `Go` 实现主逻辑，Shell 只做编排。

原因：

- 需要批量调接口与结构化导出
- 需要较强可维护性
- 后续可直接扩展统计和清理逻辑

### 12.3 k6

建议新增：

- `tests/perf/rush_create_order.js`
- `tests/perf/lib/dataset.js`
- `tests/perf/lib/summary.js`

职责：

- 加载数据集
- 执行 `create order` + `poll`
- 汇总业务统计

### 12.4 结果校验

建议新增：

- `scripts/perf/verify_rush_perf_result.sh`

职责：

- 查询 MySQL / Redis
- 输出售卖结果和一致性检查

## 13. 风险与处理

### 风险 1：`purchase token` 过期

处理：

- 控制 token 预发和压测启动间隔
- 若有效期较短，支持在 prepare 阶段最后一步再发 token

### 风险 2：库存缓存与 DB 不一致

处理：

- 每次压测前先清理并重建目标票档座位
- 强制执行库存预热
- 压测后做一致性校验

### 风险 3：压测模式误用于正常环境

处理：

- 压测配置文件单独维护
- 默认配置不启用 `PerfMode`
- 压测头要求密钥校验

### 风险 4：5000 用户准备时间过长

处理：

- 批量插入用户与观演人
- token 预发阶段做有限并发
- 数据集持久化，可重复使用

## 14. 验收标准

满足以下条件即认为该方案完成：

1. 可通过配置指定：
   - `showTimeId`
   - `ticketCategoryId`
   - `userCount`
   - `seatCount`
   - `rowCount`
   - `colCount`
2. 可批量准备压测数据集
3. 可从 `create order` 开始执行 k6 压测
4. 可输出业务指标与性能指标摘要
5. 可完成压测后库存一致性校验
6. 正常非压测路径仍保持原 JWT 鉴权行为

## 15. 推荐默认参数

第一轮推荐：

- `showTimeId=30001`
- `ticketCategoryId=40001`
- `userCount=5000`
- `seatCount=5000`
- `rowCount=50`
- `colCount=100`
- `ticketUsersPerUser=3`
- `ticketCountRange=1..3`

在该配置下：

- 总库存 = `5000`
- 总需求期望 ≈ `10000`
- 竞争倍率期望 ≈ `2.0`

这正符合“超卖竞争型”压测目标。

