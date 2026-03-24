# 订单侧基于基因法的分库分表设计

- 日期：`2026-03-24`
- 状态：`draft`
- 适用范围：`services/order-rpc`、`jobs/order-close`、`sql/order`
- 不包含：`services/pay-rpc` 的分库分表改造

## 1. 背景

当前订单域仍是单库单表实现，订单号直接使用全局 `xid.New()` 生成，订单读写模型默认绑定一个 `SqlConn` 和一组固定表名。这个实现能够支撑当前功能，但不满足以下长期目标：

- 按 `order_number` 查询订单详情时精准命中单分片
- 按 `user_id` 查询用户订单列表时也精准命中单分片
- 后续扩容时不需要更改历史订单号格式
- 迁移分片拓扑时支持在线双写、回填、切流与回滚

本设计目标是把“基于用户基因的订单分片”做成真实工程能力，而不是只停留在面试表述。

## 2. 目标与非目标

### 2.1 目标

1. 在订单侧设计稳定的基因法分片体系，将 `user_id` 派生出的稳定基因融入 `order_number`
2. 实现分片位分离，避免把当前物理库表编号直接写入订单号
3. 实现按 `order_number` 与 `user_id` 的双入口精准路由，避免读扩散
4. 保证 `d_order`、`d_order_ticket_user`、`d_user_order_index` 三表同槽位绑定
5. 支持在线扩容、数据回填、分阶段切读切写和回滚
6. 保持订单域内核心状态变更仍然是单分片本地事务

### 2.2 非目标

1. 本阶段不对 `pay-rpc` 的 `d_pay_bill`、`d_refund_bill` 做分库分表
2. 本阶段不改动节目、座位、用户等其他服务的数据分片策略
3. 不引入全局分布式事务框架
4. 不在本阶段解决跨服务最终一致性的所有历史问题，只保证订单分片改造不扩大风险面

## 3. 现状与约束

### 3.1 现状

- 当前创建订单时直接通过 `xid.New()` 生成 `order_number`
- `d_order` 与 `d_order_ticket_user` 通过单连接和单表 SQL 访问
- `ListOrders` 直接按 `user_id` 在 `d_order` 上分页
- `CloseExpiredOrders` 通过扫描 `d_order` 中超时未支付订单实现

### 3.2 约束

- 项目仍遵循 `go-zero` 组织方式，服务级代码留在 `services/order-rpc/`
- 订单域内已有支付、取消、退款等逻辑依赖本地事务更新订单主表与订单明细表
- 历史订单号必须保持可读、可路由，不能因为扩容导致老数据失效
- 当前下单主路径是异步落库，订单创建消息需要天然携带可路由的 `order_number`

## 4. 核心设计概览

整体上引入四层稳定结构：

```text
userId
  -> gene hash / mix
  -> db_gene + table_gene
  -> logic_slot
  -> route_map(versioned)
  -> physical db/table

orderNumber
  -> parse db_gene + table_gene
  -> logic_slot
  -> route_map(versioned)
  -> physical db/table
```

这里最重要的原则是：

- 订单号里存稳定基因，不存当前物理库表编号
- 逻辑槽位稳定，物理库表可变
- 扩容时调整的是 `logic_slot -> physical db/table` 的映射，不是订单号格式

## 5. 订单号设计

### 5.1 目标

订单号需要同时满足：

- 全局唯一
- 趋势递增，便于排查与时间排序
- 能解析出稳定路由基因
- 为未来扩容预留足够逻辑空间

### 5.2 位布局

订单号采用正 `int64`，推荐布局如下：

```text
0 | 31 bit seconds_since_epoch | 5 bit db_gene | 5 bit table_gene | 10 bit worker_id | 12 bit sequence
```

说明：

- 最高位固定为 `0`，保证结果始终是正数
- `seconds_since_epoch` 使用自定义纪元后的秒数
- `db_gene` 与 `table_gene` 是稳定基因位，不随当前物理拓扑变化
- `worker_id` 与 `sequence` 保留当前雪花式高并发生成能力

### 5.3 基因生成

不能直接使用 `userId % N`。推荐流程：

1. 对 `userId` 做稳定混洗，如 `SplitMix64` 风格 avalanche mix
2. 从混洗结果中切出：
   - `db_gene = hash & 0x1F`
   - `table_gene = (hash >> 5) & 0x1F`
3. 组合逻辑槽位：
   - `logic_slot = (db_gene << 5) | table_gene`

这样可以获得 `1024` 个稳定逻辑槽位，足够支撑后续多轮扩容。

### 5.4 编解码要求

需要在订单域实现明确的订单号编解码器：

- `BuildOrderNumber(userId, now, workerId, sequence) -> orderNumber`
- `ParseOrderNumber(orderNumber) -> order_number_parts`
- `LogicSlotByUserID(userId) -> logicSlot`
- `LogicSlotByOrderNumber(orderNumber) -> logicSlot`

验收要求：

- 同一个 `userId` 经过 `LogicSlotByUserID` 计算出的槽位必须与其生成出的 `orderNumber` 解析槽位完全一致
- 历史格式的订单号如果仍存在，需要提供兼容解析路径或明确拒绝策略

### 5.5 历史订单号兼容

这是本方案必须正面解决的问题。

当前历史订单号仍是旧 `xid` 格式，不包含可解析的稳定基因位，因此它们不能像新订单那样仅靠 `orderNumber` 直接反解到 `logic_slot`。为保证迁移后的历史订单详情查询仍然不读扩散，需要补一层过渡路由目录。

设计如下：

- 新增过渡目录表 `d_order_route_legacy`
- 主键与唯一键都按 `order_number` 建立
- 记录字段至少包括：
  - `order_number`
  - `user_id`
  - `logic_slot`
  - `route_version`
  - `status`
  - `create_time`
  - `edit_time`

使用规则：

- 新格式基因订单号：优先本地解析，不查目录表
- 旧格式历史订单号：先查 `d_order_route_legacy`，拿到 `logic_slot` 后精准路由
- 目录表在历史数据全部迁移稳定、且旧格式订单完全退出主查询窗口后才能评估下线

这张表的意义不是长期替代基因路由，而是保证迁移期间和历史兼容期间，旧订单仍然可以通过 `orderNumber` 精准定位，不退化为跨分片扫描。

## 6. 路由层设计

### 6.1 路由抽象

在 `services/order-rpc/internal/` 下引入独立路由层，替换当前“单 `SqlConn` + 固定模型”的使用方式。

核心接口建议如下：

```text
type Route struct {
  LogicSlot int
  DBKey     string
  TableSet  string
  Version   string
}

RouteByUserID(userID) -> Route
RouteByOrderNumber(orderNumber) -> Route
Transact(route, fn) -> error
```

### 6.2 路由映射

引入版本化 `route_map`：

```text
logic_slot -> physical db/table group
```

建议映射配置具备以下字段：

- `version`
- `logic_slot`
- `db_key`
- `table_suffix`
- `status`
- `read_weight`
- `write_mode`

其中：

- `status` 用于表示槽位状态机
- `write_mode` 控制当前是旧表写、新表写还是双写
- `read_weight` 用于迁移观察期的灰度读控制

`route_map` 的推荐来源为“配置表 + 本地缓存”：

- 配置表作为单一事实源，方便审计、切流和回滚
- 服务启动时全量加载，并支持定时刷新或显式 reload
- 业务线程只读取本地不可变快照，不在主路径上临时拼接路由

### 6.3 槽位状态机

每个逻辑槽位有自己的迁移状态：

- `stable`
- `shadow_write`
- `backfilling`
- `verifying`
- `primary_new`
- `rollback`

状态机必须是显式、可审计、幂等迁移，不允许业务代码自行推断。

## 7. 存储模型设计

### 7.1 物理表

订单域拆分后涉及三类表：

- `d_order_xx`
- `d_order_ticket_user_xx`
- `d_user_order_index_xx`

迁移兼容期额外保留一张目录表：

- `d_order_route_legacy`

其中 `xx` 表示该库下的物理表后缀。

### 7.2 Binding Table 约束

`d_order`、`d_order_ticket_user`、`d_user_order_index` 必须共享同一 `logic_slot` 和同一路由结果。不能出现：

- 主表在分片 A
- 明细表在分片 B
- 用户索引在分片 C

否则支付、取消、退款、详情查询、列表分页都会退化为跨分片操作。

### 7.3 新增用户订单索引表

新增 `d_user_order_index` 的目的是把按用户列表查询从大宽表中拆出来，避免在 `d_order` 上做跨状态分页排序。

建议字段：

- `id`
- `order_number`
- `user_id`
- `program_id`
- `order_status`
- `ticket_count`
- `order_price`
- `create_order_time`
- `status`
- `create_time`
- `edit_time`

索引要求：

- 唯一键：`uk_order_number`
- 列表分页索引：`idx_user_status_time(user_id, order_status, create_order_time, id)`
- 兜底索引：`idx_user_time(user_id, create_order_time, id)`

## 8. 业务链路改造

### 8.1 CreateOrder

创建订单链路改造为：

1. 根据 `userId` 生成带基因的 `orderNumber`
2. 把 `orderNumber` 放入订单创建事件
3. Kafka consumer 收到消息后通过 `orderNumber` 解析槽位
4. 在单分片事务内一次写入：
   - `d_order`
   - `d_order_ticket_user`
   - `d_user_order_index`

### 8.2 GetOrder / PayOrder / CancelOrder / RefundOrder

这些链路统一只按 `orderNumber` 路由：

- 根据 `orderNumber` 解析 `logic_slot`
- 命中唯一分片
- 继续保留订单主表与明细表的本地事务更新

### 8.3 ListOrders

列表链路只按 `userId` 路由：

1. 用 `userId` 计算 `logic_slot`
2. 命中唯一分片的 `d_user_order_index`
3. 在索引表上分页拿到一批 `order_number`
4. 按 `order_number` 批量回表 `d_order`

这样可以保证：

- 不读扩散
- 列表查询性能稳定
- 主表字段变宽也不影响分页主路径

### 8.4 CloseExpiredOrders

当前基于单表超时扫描的模式不再适用，需要改为以下两种之一：

1. `槽位扫描器`
   - job 周期性遍历逻辑槽位
   - 每次在单槽位对应分片查询超时订单
2. `延迟任务化`
   - 创建订单时写入到期任务
   - 到期时按 `orderNumber` 路由取消

本阶段优先落 `槽位扫描器`，因为它更容易与现有 `order-close` job 对齐；后续再演进到延迟任务模式。

## 9. 在线迁移设计

### 9.1 总体策略

采用“版本化路由映射 + 双写 + 回填 + 校验 + 分槽位切流 + 回滚”。

迁移模式定义：

- `legacy_only`
- `dual_write_shadow`
- `dual_write_new_read_old`
- `dual_write_new_read_new`
- `shard_only`

### 9.2 分阶段流程

#### 阶段 1：上线路由层但保持旧表读写

- 新代码具备基因订单号与路由能力
- 订单新写仍可先写旧表
- 新分片库表先建好但不参与主读写

#### 阶段 2：双写影子阶段

- 新订单写旧表成功后，影子写新分片
- 主读仍读旧表
- 影子写失败不阻塞用户请求，但必须记录补偿任务与报警

#### 阶段 3：历史数据回填

- 从旧表按游标扫描订单
- 根据 `orderNumber` 算出目标槽位和分片
- 幂等写入新分片三张表
- 对旧格式订单额外写入 `d_order_route_legacy`
- 记录 checkpoint，支持断点续跑

#### 阶段 4：校验

- 行数一致
- 关键聚合一致
- 随机抽样详情一致
- 随机抽样列表一致

#### 阶段 5：按槽位切新读

- 对验证通过的槽位，将读主切到新分片
- 写继续双写，保留回滚缓冲

#### 阶段 6：完全切换

- 全部槽位都进入 `primary_new`
- 观察窗口稳定后进入 `shard_only`
- 关闭旧表写入，但保留只读观测窗口

### 9.3 回滚设计

回滚不能依赖修改历史订单号，只能依赖路由与状态切换：

- 读回滚：把目标槽位读主切回旧表
- 写回滚：切回旧表主写，或继续双写但旧表为主
- 回填任务保留 checkpoint，可重试
- 补偿任务以 `order_number` 为幂等键重放

回滚原则：

- 先保证读正确
- 再收敛写模式
- 禁止一键全量无脑回滚所有槽位
- 必须支持槽位级回滚

## 10. 失败语义与幂等

### 10.1 主写与影子写

- 主写目标失败：请求失败，不能返回成功
- 影子写失败：允许记录补偿后异步修复，但不允许静默吞掉

### 10.2 幂等键

以下动作统一以 `order_number` 为幂等键：

- 订单创建双写
- 回填重放
- 补偿修复
- 切流后数据补齐

### 10.3 槽位状态幂等

槽位状态机切换必须满足：

- 重复执行同一步无副作用
- 非法跃迁直接拒绝
- 所有状态切换可审计

## 11. 测试与验收

### 11.1 单测

覆盖：

- 基因 hash / mix
- 订单号编码与解码
- `LogicSlotByUserID`
- `LogicSlotByOrderNumber`
- 路由映射解析
- 槽位状态机

### 11.2 集成测试

覆盖：

- `CreateOrder` 在分片下写入三表一致
- `GetOrder` 通过 `orderNumber` 精准命中
- `ListOrders` 通过 `userId` 精准命中
- `PayOrder` / `CancelOrder` / `RefundOrder` 在单分片事务内仍正确
- `CloseExpiredOrders` 可按槽位扫描关闭

### 11.3 迁移测试

覆盖：

- 双写成功
- 影子写失败后补偿成功
- 回填断点续跑
- 校验脚本发现不一致
- 切新读后结果一致
- 回滚后结果一致

### 11.4 验收标准

必须满足：

1. `GetOrder(orderNumber)` 不读扩散
2. `ListOrders(userId)` 不读扩散
3. `d_order`、`d_order_ticket_user`、`d_user_order_index` 一致
4. 主写失败不丢单，影子写失败可补偿
5. 任一槽位支持独立切新与独立回滚
6. 扩容不修改历史订单号格式

## 12. 落地实施建议

建议按以下顺序实施：

1. 先落订单号编解码与路由层
2. 再落分片仓储与三表写入
3. 然后改造详情、列表、状态流转
4. 最后补双写、回填、校验、切流、回滚脚本

原因：

- 先把稳定路由中枢立住
- 再把核心业务链路迁到新抽象
- 最后叠加迁移机制，避免同时改业务和迁移两层

## 13. 风险与取舍

### 13.1 风险

- 迁移期双写链路变长，错误面上升
- 旧表与新分片短期并存，运维复杂度增加
- `CloseExpiredOrders` 从单表扫描改为分槽位扫描后，job 调度逻辑更复杂

### 13.2 取舍

- 不引入分布式事务，而是把订单域内事务控制在单分片内
- 不直接依赖中间件透明分片，而是在应用层保留路由显式控制力
- 用索引表换取列表精准路由和分页稳定性

## 14. 本阶段拟新增内容

### 14.1 代码

- `services/order-rpc/internal/sharding/`
- `services/order-rpc/internal/repository/`
- `services/order-rpc/internal/migration/`
- `jobs/order-migrate/` 或 `scripts/order-migrate/`

### 14.2 SQL

- `sql/order/sharding/`
- `sql/order/d_user_order_index.sql`
- 分片库表建表模板

### 14.3 文档

- 迁移 runbook
- 回滚 runbook
- 路由配置说明

## 15. 决策结论

本设计采用“稳定基因 + 逻辑槽位 + 版本化路由映射”的基因法分片方案。

它满足以下关键诉求：

- `order_number` 与 `user_id` 双入口精准路由
- 订单详情和用户订单列表不读扩散
- 订单域内仍保留本地事务语义
- 支持在线扩容、回填、切流与回滚
- 历史订单号格式不因拓扑变化而失效

这套方案也是后续把“订单侧基于基因法的分库分表”写成真实项目亮点的最小可信工程基线。
