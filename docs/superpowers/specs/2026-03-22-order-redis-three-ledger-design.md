# 订单域 Redis 三本账设计

## 背景

当前订单创建链路存在两个核心问题：

- 单账号限购 `perAccountLimit` 依赖数据库统计，但订单创建已改成 Kafka 异步落库，存在“请求已成功返回、订单尚未落库”的时间窗，导致限购可能失真。
- 节目域的锁座仍依赖 MySQL 全量拉取可售座位并 `for update` 后在应用层配座，高并发时热点票档会退化成数据库锁竞争。

结合原 Java 项目的职责划分，本次设计不再尝试用一套 Redis 数据同时解决所有问题，而是拆成三本独立账本：

- 限购账本：用户在节目维度已占用的票数
- 库存账本：票档维度剩余库存
- 座位账本：票档下可售座位、锁定座位和已售座位

## 目标

- 将单账号限购前置到 Redis，避免异步落库窗口导致限购失真
- 将票档库存校验和自动配座前置到 Redis Lua 原子执行，降低热点票档对 MySQL 的锁竞争
- 明确服务职责边界：`order-rpc` 负责限购账本，`program-rpc` 负责库存账本和座位账本
- Redis 关键 key 缺失时直接拒单，并异步初始化；初始化完成前持续拒单
- 自动配座规则沿用现有 Go 实现：优先同排连座，找不到则取排序后的前 `N` 个

## 非目标

- 不支持用户手动选座
- 不在缺 key 时同步回源 MySQL 兜底
- 不把三本账全部塞进同一个服务或同一个 Redis key
- 不改变当前“待支付、已支付都算限购”的业务口径
- 不在本次设计里引入分库分表、多 broker Kafka 或复杂多级缓存

## 方案选型

### 方案 1：严格 Redis 前置

下单只认 Redis 三本账，缺 key 直接拒单，并异步初始化。初始化完成前持续拒单。

优点：

- 账本口径单一
- 高并发下不把热点重新打回 MySQL
- 更符合“Redis 原子校验 + 扣减”的目标

缺点：

- 冷启动和 key 失效恢复期间会有短暂拒单窗口

### 方案 2：缺 key 时同步回源 MySQL

优点：

- 首次访问成功率更高

缺点：

- 热点流量会重新穿透到数据库
- 与本次明确确认的“缺 key 直接拒单，异步初始化”不一致

### 结论

采用方案 1。

## 服务边界

### Order RPC

`order-rpc` 负责：

- `userId + programId` 维度的限购账本
- 下单时限购原子校验和占用
- Kafka 发送失败、废单、取消、超时关闭、退款时回滚限购占用

### Program RPC

`program-rpc` 负责：

- `programId + ticketCategoryId` 维度的库存账本
- `programId + ticketCategoryId` 下可售座位、锁定座位、已售座位的维护
- 自动配座、锁座、释放锁座、确认已售

这种拆分与原 Java 项目的职责更接近：

- 订单域维护账户购票数量
- 节目域维护票档余票和座位状态

## 数据模型

### 限购账本

维度：

- `userId + programId`

语义：

- 当前用户在该节目下已占用的票数
- 口径包含 `待支付` 和 `已支付`

建议 key：

- `order:limit:user_program:{userId}:{programId}`

### 库存账本

维度：

- `programId + ticketCategoryId`

语义：

- 当前票档剩余库存

建议 key：

- `program:stock:{programId}:{ticketCategoryId}`

### 座位账本

维度：

- `programId + ticketCategoryId`

语义：

- 可售座位集合
- 锁定座位集合
- 已售座位集合

建议 key：

- `program:seat:no_sold:{programId}:{ticketCategoryId}`
- `program:seat:lock:{programId}:{ticketCategoryId}`
- `program:seat:sold:{programId}:{ticketCategoryId}`

### 订单预占明细

语义：

- 记录某个订单或冻结请求到底占用了多少限购、多少库存、哪些 seat
- 用于 Kafka 发送失败、消费者废单、取消、超时关闭、退款时精确回滚

建议 key：

- `order:reserve:{orderNumber}`

内容至少包含：

- `userId`
- `programId`
- `ticketCategoryId`
- `ticketCount`
- `seatIds`
- `freezeToken`
- `status`

## 预热与初始化

### 初始化原则

- Redis 关键 key 缺失时直接拒单
- 不做同步回源 MySQL
- 触发异步初始化任务
- 初始化完成前，相关请求持续拒单

### 预热对象

- 限购账本：`userId + programId`
- 库存账本：`programId + ticketCategoryId`
- 座位账本：`programId + ticketCategoryId`

### 初始化方式

- 请求触发异步初始化
- 后台幂等装载
- 可选补充定时预热或对账任务

### 加载中标记

为避免并发重复装载，需要短 TTL 的 loading 标记，例如：

- `order:limit:loading:{userId}:{programId}`
- `program:stock:loading:{programId}:{ticketCategoryId}`

## 自动配座规则

不支持用户手动选座，只保留系统自动安排座位。

自动配座规则完全对齐当前 Go 实现：

1. 按 `row_code asc, col_code asc, id asc` 排序
2. 优先寻找同排连续的 `N` 个座位
3. 若不存在满足条件的连座，则直接取排序后的前 `N` 个座位

本次设计调整的是执行位置，不调整配座规则本身。现有规则从“基于 MySQL 全量拉取后在 Go 层执行”迁移为“Redis Lua 原子执行”。

## 创建订单时序

1. `order-rpc` 做基础校验：
   - 参数
   - 购票人
   - 单笔限购
2. `order-rpc` 在 Redis 对限购账本做原子校验并占用
3. 若限购 key 缺失，则直接拒单并触发异步初始化
4. `order-rpc` 调用 `program-rpc` 执行自动配座和锁座
5. `program-rpc` 在 Redis Lua 中一次完成：
   - 校验库存账本是否足够
   - 扣减票档库存
   - 从可售座位集合中自动配座
   - 将选中的座位移入锁定集合
   - 记录冻结明细
6. `program-rpc` 返回 `freezeToken` 和座位快照
7. `order-rpc` 组装订单事件并发 Kafka
8. Kafka 发送成功后返回下单成功

## 失败补偿

### 限购占用失败

- 直接返回失败
- 不调用 `program-rpc`
- 不发 Kafka

### Program RPC 锁座失败

- 回滚限购占用
- 返回失败

### Kafka 发送失败

- 回滚限购占用
- 调用 `program-rpc` 释放库存和锁座
- 返回失败

### Consumer 过期废单

- 回滚限购占用
- 调用 `program-rpc` 释放库存和锁座
- 记录日志和观测事件

所有回滚操作都必须幂等。

## 落库与后续状态流转

### Consumer 正常落库

- Kafka consumer 负责把订单落到 MySQL
- 正常落库成功后，不再变更 Redis 的限购主账本和库存主账本
- 因为这两本账在下单成功时已经生效

### 支付成功

- 不回补限购
- 不回补库存
- 只把座位从“锁定”转换为“已售”

原因：

- `待支付` 和 `已支付` 都算限购

### 取消订单

- 回补限购账本
- 回补库存账本
- 把座位从“锁定”退回“可售”

### 超时关闭

- 与取消订单走同一套回补逻辑

### 退款

- 回补限购账本
- 回补库存账本
- 座位从“已售”退回“可售”

## MySQL 的角色

MySQL 在本方案中的职责变为：

- 订单持久化
- 订单查询
- 启动预热和异步初始化时的数据来源
- 定时对账的最终校验来源

MySQL 不再承担下单主路径上的高并发限购判断和热点票档锁座。

## 配置改动

### Order RPC

新增 Redis 配置以及限购账本相关参数：

- Redis 地址
- 限购 key TTL
- 初始化冷却时间
- loading 标记 TTL

### Program RPC

新增 Redis 配置以及库存/座位账本相关参数：

- Redis 地址
- 库存 key TTL
- 座位 key TTL
- loading 标记 TTL

## 模块拆分

### Order RPC 新增模块

建议新增：

- `services/order-rpc/internal/limitcache/`

包含：

- `purchase_limit_store.go`
- `purchase_limit_keys.go`
- `purchase_limit_loader.go`
- `purchase_limit_lua.go`

### Program RPC 新增模块

建议新增：

- `services/program-rpc/internal/seatcache/`

包含：

- `seat_stock_store.go`
- `seat_stock_keys.go`
- `seat_stock_loader.go`
- `seat_stock_lua.go`

## 现有逻辑改造点

### Order RPC

- `create_order_logic.go`
  增加限购账本原子占用和缺 key 拒单逻辑
- `create_order_consumer_logic.go`
  增加废单回滚限购逻辑
- `order_domain_helper.go`
  取消订单成功后回补限购
- `close_expired_orders_logic.go`
  复用取消回补逻辑
- `refund_order_logic.go`
  退款后释放限购占用
- `service_context.go`
  注入 Redis 客户端
- `config.go` 和 `etc/order-rpc.yaml`
  增加 Redis 相关配置

### Program RPC

- `auto_assign_and_freeze_seats_logic.go`
  从 MySQL `for update` 模式改为 Redis Lua 模式
- `release_seat_freeze_logic.go`
  回补库存并释放锁座
- `confirm_seat_freeze_logic.go`
  把座位从锁定转为已售
- `service_context.go`
  注入 Redis 客户端
- `config.go` 和 `etc/program-rpc.yaml`
  增加 Redis 相关配置

## 测试与验证

### Order RPC 集成测试

- 限购 key 缺失时直接拒单，并触发异步初始化
- 限购占用成功后 Kafka 发送失败，能正确回滚
- Kafka 废单时能正确回滚限购
- 取消、超时关闭、退款后能正确释放限购
- 重复取消、重复回滚保持幂等

### Program RPC 集成测试

- 库存/座位 key 缺失时直接拒单，并触发异步初始化
- 自动配座优先同排连座
- 找不到连座时取排序前 `N` 个
- 扣库存、锁座、释放、确认已售保持正确
- 重复释放、重复确认保持幂等

### API 与验收验证

- Redis 未预热时下单被拒
- 预热完成后同请求可以成功下单
- 下单成功后，支付、取消、超时关闭、退款分别满足预期

## 风险与约束

- 冷启动和 key 失效恢复期间会有拒单窗口
- Redis 账本成为下单主路径的关键依赖，必须补齐监控和告警
- 初始化和回滚操作必须严格幂等，否则三本账会漂移
- 后续需要补充对账任务，定期用 MySQL 修正 Redis 偏差

## 实施顺序

1. 先补 `order-rpc` 限购账本和缺 key 拒单能力
2. 再补 `program-rpc` 的 Redis 库存和座位账本
3. 将自动配座从 MySQL `for update` 迁移到 Redis Lua
4. 补齐取消、超时关闭、退款的 Redis 回补
5. 增加初始化、幂等和对账相关测试

## 预期结果

- 下单主路径不再依赖 MySQL 统计限购和全量锁座
- 限购、库存、座位三条高并发热点路径前置到 Redis
- Redis 缺 key 时行为清晰：直接拒单、异步初始化、初始化完成前持续拒单
- 自动配座规则保持不变，但执行成本从数据库锁竞争转为 Redis 原子脚本
