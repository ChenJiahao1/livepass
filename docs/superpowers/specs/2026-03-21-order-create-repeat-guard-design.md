# order/create 防重复处理设计

## 背景

当前 Go 版下单链路的真实入口位于 `services/order-rpc/internal/logic/createorderlogic.go`。一次创建订单请求会串联节目预下单查询、用户与观演人校验、座位冻结、订单主表写入和订单观演人明细写入。现有实现没有像 Java 版那样在入口增加“用户 + 节目”维度的防重复处理，也没有在节目侧增加“节目 + 票档”维度的热点本地锁，因此用户双击、网关短时间重放、同一用户并发提交时，可能触发重复冻结座位和重复写入未支付订单。

Java 版的处理方式不是严格业务幂等，而是“执行窗口内防重复处理”。入口层使用 `@RepeatExecuteLimit(name=CREATE_PROGRAM_ORDER, keys={userId, programId})` 拦截重复提交；节目侧再按 `programId + ticketCategoryId` 加本地 `ReentrantLock` 压热点票档并发。Go 版本次设计对齐这个语义，不引入新的请求号协议，不要求重复请求返回同一 `orderNumber`。

## 目标

- 对齐 Java 版下单防重复语义，在一次请求执行窗口内拦截同一用户对同一节目的重复提交。
- 在节目侧为同一节目、同一票档的座位冻结增加进程内热点锁，减少热点库存并发时的重复扫描和事务冲突。
- 设计应贴合当前 `damai-go` 的服务边界：入口防重复位于 `order-rpc`，热点本地锁位于 `program-rpc`。
- 不改变现有对外 API，不引入新的请求字段，不改动订单号生成规则。
- 保持当前数据库事务和 `for update` 正确性不变；新加锁仅用于抑制重复执行和热点冲突，不承担最终一致性职责。

## 非目标

- 不实现严格业务幂等。第一次请求成功但响应丢失后，再次重试不会返回同一张订单。
- 不在本轮为 `/order/pay`、`/order/refund`、`/order/cancel` 增加统一防重复框架。
- 不引入 Java 版那套注解/AOP 体系；Go 版使用显式 helper 和 service context 注入落地。
- 不引入库存缓存化、Lua 扣减或消息化建单。本轮仅处理重复提交与热点并发。

## 方案对比

### 方案 A：仅在 order-rpc 增加入口防重复

优点是改动最小，能直接拦截用户双击和短时间重放。缺点是节目侧热点票档仍然可能在不同用户并发下高频进入座位扫描和事务竞争，无法对齐 Java 版第二层本地锁语义。

### 方案 B：仅在 program-rpc 增加热点本地锁

优点是能缓解票档热点冲突。缺点是同一用户重复提交仍然会进入完整下单流程，只是部分竞争被拖慢，无法防止重复冻结与重复建单尝试。

### 方案 C：入口防重复 + 节目侧热点本地锁

这是本次采用的方案。第一层在 `order-rpc` 按 `userId + programId` 拒绝重复提交；第二层在 `program-rpc` 按 `programId + ticketCategoryId` 压热点票档并发。这样既对齐 Java 版的两层语义，也不需要改变现有 API 和订单数据模型。

## 选中方案

### 1. order-rpc 入口防重复

在 `services/order-rpc/internal/logic/createorderlogic.go` 的 `CreateOrder` 方法开头增加防重复处理守卫。守卫 key 为：

`create_order:{userId}:{programId}`

守卫行为分两步：

- 先对同一个 key 获取进程内 keyed mutex，并使用 `TryLock` 语义；获取失败立即返回，不阻塞等待。
- 再基于 etcd `Session + Mutex` 获取分布式锁，覆盖多实例部署场景。

本地 keyed mutex 采用 `TryLock` 的原因是对齐 Java 版“抢不到直接拒绝”的语义，而不是等待前一个请求结束后继续执行。否则同一用户双击下单时，第二个请求可能在第一个请求执行完成后继续建单，破坏“执行窗口内拦截重复提交”的预期。

分布式锁使用 etcd `concurrency.Session` 的 keepalive 机制维持 lease，相当于自动续租的 watchdog。锁在 `CreateOrder` 方法执行结束时释放，不保留成功标记，也不缓存执行结果。该行为与 Java 版 `RepeatExecuteLimit` 的默认语义一致：在 etcd 健康且 lease keepalive 正常的前提下，防重窗口等于方法执行窗口，而不是成功后固定保留一段时间。

### 2. program-rpc 热点本地锁

在 `services/program-rpc/internal/logic/autoassignandfreezeseatslogic.go` 进入事务前增加热点票档本地锁。锁 key 为：

`program_seat_freeze:{programId}:{ticketCategoryId}`

该锁仅为进程内锁，不承担跨实例一致性职责；跨实例正确性仍然依赖现有数据库事务、行锁和座位状态更新。锁的目标是减少以下开销：

- 同一票档热点并发下重复扫描可售座位；
- 同一票档频繁进入事务导致的锁冲突；
- 高并发下连续触发过期冻结回收逻辑。

### 3. 锁顺序与职责边界

锁获取顺序固定为：

1. `order-rpc` 入口防重复锁；
2. `program-rpc` 热点票档本地锁；
3. `program-rpc` 数据库事务与 `for update`；
4. `order-rpc` 数据库事务写订单。

第一层锁解决“同一用户 + 同一节目”的重复提交。第二层锁解决“同一节目 + 同一票档”的热点并发。数据库事务继续承担最终正确性和状态落库职责。两层锁都不应该跨 RPC 传播，也不应该共享 key 结构。

### 4. 分布式锁协议

`order-rpc` 的分布式锁采用 etcd 官方并发组件：

- 创建 `concurrency.Session(client, WithTTL(ttl), WithContext(ctx))`
- 基于同一 session 创建 `concurrency.NewMutex(session, prefix+lockKey)`
- 方法执行期间由 session 自动 keepalive lease
- 方法返回时调用 `Unlock` 并关闭 session

锁前缀建议为：

`/damai-go/repeat-guard/order-create/`

etcd 锁不需要额外设计 compare-and-del token 协议，因为 lease 与 mutex 生命周期由 session 管理。

### 5. 续租策略

etcd 锁不采用“固定 TTL 必须覆盖最大建单耗时”的假设，而是采用 session keepalive：

- 初始 TTL 只表示 session lease 的基础时长，不表示最大建单时间；
- session 在方法执行期间自动 keepalive；
- session 的生命周期与 `CreateOrder` 的上下文绑定，方法返回时关闭 session。

若 session 在方法执行中途失去 keepalive：

- 记录 error log 和 metric；
- 当前请求继续执行，不主动中断业务流程；
- 该次请求之后到方法结束前，重复提交抑制语义降级为“尽力而为”。

这是基础设施失败场景下的显式取舍。正常验收口径以“etcd 可用且 keepalive 正常”为前提，不把 etcd 故障下的重复进入视为业务逻辑 bug。

## 配置设计

### order-rpc

`order-rpc` 不新增独立 Redis 依赖，直接复用现有 `RpcServerConf.Etcd.Hosts` 创建 repeat guard 的 etcd client。

为 `services/order-rpc/internal/config/config.go` 增加 RepeatGuard 配置段，例如：

- `Prefix`
- `SessionTTL`

`SessionTTL` 只表示 etcd session lease 的基础时长，不表示成功后保留幂等标记，也不是最大建单时长。建议初始默认 `10s`。

`services/order-rpc/internal/svc/service_context.go` 需要注入：

- etcd client
- keyed mutex 容器
- repeat guard helper

### program-rpc

`program-rpc` 只需要 keyed mutex 容器，不需要为本轮新增 Redis 依赖。热点锁是纯进程内能力。

两侧 keyed mutex 容器统一采用“引用计数 + 解锁后回收”，避免 key 集合随节目、票档和用户维度无限增长，同时减少后台清理线程的额外复杂度。

## 错误语义

重复提交时统一返回业务错误：

- 业务错误：`xerr.ErrOrderSubmitTooFrequent`
- gRPC code：`codes.ResourceExhausted`
- `order-api` / `gateway-api` 对外 HTTP：`429 Too Many Requests`
- 错误文案：`提交频繁，请稍后重试`

该错误属于显式拒绝，不回退成库存不足、订单失败或系统异常。这样用户和上层调用方能区分“竞争失败”和“真实业务校验失败”。测试和上游识别都应以“错误码/状态码 + 文案”联合断言，不能只匹配文案。

热点锁只是排他控制；若在获取热点锁后进入数据库事务仍然因为库存不足失败，仍按现有语义返回座位库存不足，不应改写为“提交频繁”。

基础设施错误单独处理：

- 本地 keyed mutex `TryLock` 失败：视为重复提交，返回 `codes.ResourceExhausted`
- etcd mutex 已被占用：视为重复提交，返回 `codes.ResourceExhausted`
- 创建 etcd client / session / lock 请求超时或 etcd 不可用：采用 `fail-closed`，拒绝本次下单，但返回基础设施错误而不是“提交频繁”；gRPC 建议映射 `codes.Unavailable`，对外 HTTP 建议映射 `503 Service Unavailable`

这里明确采用 `fail-closed`，因为该能力属于下单入口的并发保护基础设施，etcd 不可用时继续放量建单会直接扩大重复冻结和重复建单风险。

## 测试设计

### order-rpc

新增集成测试覆盖：

- 同一 `userId + programId` 的两次并发 `CreateOrder`，一条成功，一条返回 `codes.ResourceExhausted + xerr.ErrOrderSubmitTooFrequent`。
- 不同用户、同一节目并发创建订单，不因入口防重复被互相阻塞。
- 同一用户、不同节目并发创建订单，不因入口防重复被互相阻塞。
- 本地 keyed mutex 使用 `TryLock`，第二个请求不会阻塞等待前一个请求完成。
- etcd 不可用时，`CreateOrder` 返回 `codes.Unavailable`，不会静默降级为继续建单。

### program-rpc

新增集成测试覆盖：

- 同一 `programId + ticketCategoryId` 的并发冻结请求，在进程内热点锁存在时仍只冻结正确数量的座位。
- 不同票档的冻结请求不会共享同一热点锁。
- 重复 `requestNo` 的既有幂等语义保持不变。

### 回归验证

需回归现有下单、支付、退款、超时关单测试，重点确认：

- `CreateOrder` 正常路径未受影响；
- 冻结失败后的补偿释放逻辑不受新锁影响；
- `program-rpc` 既有库存测试仍通过。
- 通过 `order-api` 或 `gateway-api` 的对外请求，重复提交能稳定得到 HTTP 429。
- etcd 故障注入时，对外请求能稳定得到 HTTP 503。

## 风险与取舍

### 风险 1：etcd keepalive 失效

如果 `order-rpc` 的 etcd session 在方法执行期间失去 keepalive，重复请求抑制窗口会提前失效。缓解方式是：

- 将 session TTL 显式配置化；
- 对 session `Done()` 提前关闭增加日志和指标；
- 在压测环境验证慢请求下 keepalive 能稳定覆盖整个执行窗口。

### 风险 2：进程内热点锁只在单实例内生效

这是有意识的取舍。Java 版第二层也是本地锁，目的不是跨实例一致性，而是压热点冲突。跨实例正确性继续依赖数据库行锁和座位状态更新。

### 风险 3：入口防重复语义不是严格幂等

这是与 Java 版对齐后的已知取舍。若后续要支持“客户端超时重试返回同一订单”，需要另起一轮做 `requestNo` 业务幂等，不应在本次改造中混入。

## 交付范围

本次交付仅包括：

- `order-rpc` 下单入口防重复处理；
- `program-rpc` 座位冻结热点本地锁；
- 对应配置、测试和错误语义。

不包括：

- 新 API 字段；
- 数据库表结构调整；
- 支付、退款、取消接口的统一防重复框架；
- Redis/Lua 库存扣减改造。

## 验收标准

- 并发双击下单时，同一用户对同一节目不会并发进入完整建单流程。
- 热点票档的座位冻结在单实例内不会出现高频重复扫描导致的明显竞争放大。
- 现有下单、支付、退款、关单集成测试全部通过。
- 错误语义清晰，重复提交返回统一业务错误，不被混淆为库存不足或系统错误。
