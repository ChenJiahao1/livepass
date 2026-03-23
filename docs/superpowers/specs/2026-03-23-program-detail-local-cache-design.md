# Program Detail 本地缓存设计

## 背景

当前 `program-rpc` 的 `GetProgramDetail` 会在一次请求中组合多次查询：

- `d_program` 主表
- `d_program_show_time` 首场次
- `d_program_category` 全量分类
- `d_program_group` 分组信息
- `d_ticket_category` 节目票档

现有实现位于 [get_program_detail_logic.go](/home/chenjiahao/code/project/damai-go/services/program-rpc/internal/logic/get_program_detail_logic.go)，每次请求都会直接打到这些 model。

从 go-zero 能力看，框架确实提供基于 Redis 的 model cache，但当前仓库里的 `program-rpc` model 仍然是纯 `sqlx.SqlConn` 版本，尚未接入 `sqlc.CachedConn`。这意味着当前 `GetProgramDetail` 既没有本地缓存，也没有复用 go-zero 的 Redis 读缓存。

本轮希望为 `GetProgramDetail` 建立一套“本地 L1 + go-zero Redis L2”的分层缓存方案，以降低热点详情页的数据库压力，并在允许短时间旧数据的前提下提升稳定性。

## 目标

本设计的目标是：

- 仅覆盖 `program-rpc` 的 `GetProgramDetail`
- 在热点请求场景下减少重复的对象组装、Redis 访问和数据库访问
- 复用 go-zero 自带的 model Redis cache 作为 L2
- 补充进程内 L1 本地缓存，降低跨节点重复访问 Redis 的压力
- 对不存在的 `programId` 做短 TTL 负缓存，避免持续穿透
- 保持最终一致，不追求强一致

## 非目标

本轮不做以下内容：

- 不覆盖 `GetProgramPreorder`
- 不覆盖 `ListTicketCategoriesByProgram`
- 不引入 Redis Stream 广播本地缓存失效
- 不引入布隆过滤器
- 不抽象成 `pkg/` 下的全局缓存框架
- 不对所有 `program-rpc` model 全量缓存化

## 现状判断

### 1. 当前 `program-rpc` 尚未接入 go-zero model cache

当前 `program-rpc` 的 model 构造函数仍然只接收 `sqlx.SqlConn`，例如：

- [d_program_model.go](/home/chenjiahao/code/project/damai-go/services/program-rpc/internal/model/d_program_model.go)
- [d_program_group_model.go](/home/chenjiahao/code/project/damai-go/services/program-rpc/internal/model/d_program_group_model.go)
- [d_program_show_time_model.go](/home/chenjiahao/code/project/damai-go/services/program-rpc/internal/model/d_program_show_time_model.go)

这说明“go-zero 自带 Redis 读缓存”在框架能力上存在，但在当前仓库 `program-rpc` 上还没有落地。

### 2. go-zero L2 更适合单行查询，不适合直接覆盖当前所有子查询

`GetProgramDetail` 涉及的查询里：

- `d_program.FindOne` 适合直接走 go-zero model cache
- `d_program_group.FindOne` 适合直接走 go-zero model cache
- `d_program_show_time.FindFirstByProgramId` 虽然是自定义查询，但仍属于单行查询，可手工接 `sqlc.CachedConn.QueryRowCtx`
- `d_program_category.FindAll` 是列表查询，不适合直接走这套单行 model cache
- `d_ticket_category.FindByProgramId` 是列表查询，也不适合直接走这套单行 model cache

因此本轮不能简单理解为“只要打开 go-zero cache，`GetProgramDetail` 全链路就都进 Redis”。首期收益会来自：

- L1 详情对象本地缓存
- L2 单行子查询 Redis 缓存
- 分类快照本地缓存

## 方案概览

首轮采用“L1 本地详情缓存 + L2 go-zero model Redis cache”的分层方案：

- `GetProgramDetail` 在 `program-rpc` 内新增 `ProgramDetailCache`
- `ProgramDetailCache` 先查本地 L1
- L1 miss 时进入 `DetailLoader`
- `DetailLoader` 复用 cached model 查询 `d_program`、`d_program_group`、`first_show_time`
- `d_program_category` 使用独立本地快照缓存
- `d_ticket_category` 首期仍允许在 L1 miss 时直接查 DB
- 最终组装成 `pb.ProgramDetailInfo` 后写回 L1

该方案避免了首期同时引入：

- detail 视图级 Redis 二次缓存
- Stream 失效广播
- Bloom 存在性过滤

整体复杂度更低，也更贴合当前仓库的实现基础。

## 架构设计

### 1. 缓存层位置

缓存层放在 `services/program-rpc/internal/programcache/`，不下沉到 `pkg/`。

原因：

- 当前需求只覆盖 `GetProgramDetail`
- 缓存键、装配逻辑、失效边界都与 `program` 域强相关
- 首期不需要抽象成跨服务公共能力

建议新增组件：

- `ProgramDetailCache`
- `DetailLoader`
- `CategorySnapshotCache`

### 2. ServiceContext 注入

在 `services/program-rpc/internal/svc/service_context.go` 中注入：

- cached `DProgramModel`
- cached `DProgramGroupModel`
- cached `DProgramShowTimeModel`
- `CategorySnapshotCache`
- `ProgramDetailCache`

`GetProgramDetailLogic` 只依赖 `ProgramDetailCache.Get(ctx, programId)`，不再直接拼装多表查询。

### 3. 调用链

建议调用链如下：

```text
GetProgramDetailLogic
  -> ProgramDetailCache.Get(programId)
    -> L1 本地缓存命中: 直接返回
    -> L1 未命中:
       -> DetailLoader.Load(programId)
          -> DProgramModel.FindOne
          -> DProgramShowTimeModel.FindFirstByProgramId
          -> DProgramGroupModel.FindOne
          -> CategorySnapshotCache.GetAll
          -> DTicketCategoryModel.FindByProgramId
          -> assemble pb.ProgramDetailInfo
       -> 回填 L1
       -> 返回结果
```

## 缓存分层设计

### 1. L1 本地缓存

L1 使用 go-zero 自带的进程内缓存 [cache.go](/home/chenjiahao/go/pkg/mod/github.com/zeromicro/go-zero@v1.10.0/core/collection/cache.go)。

L1 只缓存最终的 `ProgramDetailInfo` 视图，而不是单个 model 对象。

建议：

- key: `program:detail:view:<programId>`
- not-found key: `program:detail:view:notfound:<programId>`
- 正常 TTL: `20s`
- not-found TTL: `5s`
- 容量上限: `5000` 到 `10000`

为避免共享引用被后续代码意外修改，L1 建议缓存序列化后的字节数据，读取时再反序列化为新对象。

### 2. L2 Redis 缓存

L2 不再自建一层“详情 Redis 视图缓存”，而是复用 go-zero model cache。

建议首期为以下查询接入 L2：

- `d_program.FindOne`
- `d_program_group.FindOne`
- `d_program_show_time.FindFirstByProgramId`

其中：

- `d_program` 和 `d_program_group` 可直接采用 goctl `--cache` 风格生成的 model 结构
- `FindFirstByProgramId` 需在自定义 model 内手工使用 `sqlc.CachedConn.QueryRowCtx`

自定义 key 建议：

- `cache:dProgramShowTime:first:programId:<programId>`

### 3. 分类快照缓存

`d_program_category.FindAll` 是全量分类快照，首期不放 Redis，直接用本地快照缓存。

建议：

- key 固定为 `program:category:snapshot`
- TTL: `5m`

分类数据体量小、修改频率低，本地快照足以覆盖首期需求。

### 4. 票档查询策略

`d_ticket_category.FindByProgramId` 首期保持直查 DB。

原因：

- 当前只做 `GetProgramDetail`
- 已有 L1 详情缓存后，该查询只在 L1 miss 时才发生
- 首期先观察是否仍构成瓶颈，再决定是否为票档列表单独增加 Redis 列表缓存

## 配置设计

在 `services/program-rpc/internal/config/config.go` 中新增两类配置：

### 1. go-zero model cache 配置

- `Cache cache.CacheConf`

用于初始化 `sqlc.CachedConn`。

### 2. 本地缓存配置

建议新增：

- `DetailTTL`
- `DetailNotFoundTTL`
- `DetailCacheLimit`
- `CategorySnapshotTTL`

也可以包装为独立 `LocalCache` 配置块。

## TTL 策略

当前需求允许短时间旧数据，但要求最终一致，因此 TTL 是首要一致性保障。

建议值：

- L1 detail TTL: `20s`
- L1 detail not-found TTL: `5s`
- L2 Redis TTL: `5m`
- L2 Redis not-found TTL: `30s`
- category snapshot TTL: `5m`

原则如下：

- L1 明显短于 L2，保证各节点最终收敛快
- L2 足够长，保证 L1 miss 时尽量优先回 Redis
- not-found TTL 明显更短，避免新节目上线后长时间不可见

## 失效策略

### 1. 首期失效方式

首期仅支持：

- 主动删除当前节点 L1
- 主动删除 L2 Redis key
- TTL 兜底收敛

不做跨节点广播。

### 2. 主动失效接口

`ProgramDetailCache` 建议提供：

- `Invalidate(ctx, programId int64)`
- `InvalidateCategories()`

用于后续后台修改链路接入。

### 3. 写路径要求

如果未来接入节目后台修改链路，更新完成后至少需要删除：

- `d_program` 主键缓存
- `d_program_group` 主键缓存
- `first_show_time` 自定义缓存 key
- 当前节点 L1 `program:detail:view:<programId>`

如果写路径不统一，首期不追求完整主动失效，仍依赖 TTL 完成最终收敛。

## 错误处理

### 1. NotFound

节目不存在时：

- loader 返回 `programNotFoundError()`
- L1 写入短 TTL 负缓存
- L2 中 `d_program.FindOne` 由 go-zero not-found placeholder 承担第一层负缓存

### 2. Redis 异常

当 L2 Redis 不可用时：

- 不继续穿透性依赖 Redis
- 允许在 L1 miss 时直接走 DB 查询链路
- 返回真实查询结果或错误

首期不因为 L2 不可用而直接失败整个详情查询。

### 3. 本地缓存异常

本地缓存反序列化失败时：

- 删除坏数据
- 重新回源 loader

避免因单个坏缓存导致持续报错。

## 方案取舍

### 1. 为什么首期不做 Bloom

原因：

- `GetProgramDetail` 是单 key 查询，不是大规模随机穿透的第一优先场景
- go-zero L2 已自带 not-found placeholder
- L1 本地负缓存已能进一步挡住短时间重复穿透

因此 Bloom 的收益不足以覆盖首期复杂度。

### 2. 为什么首期不做 Redis Stream

原因：

- 当前只做一个读接口
- 业务允许短时间旧数据
- L1 TTL 已足够短
- 现阶段后台修改链路也尚未统一

在这个阶段引入 Stream 会增加消费游标、重启恢复、失败补偿等复杂度，但收益有限。

### 3. 为什么不直接做 detail 视图级 Redis 缓存

原因：

- 首期已经有 go-zero model cache 作为 L2
- 再叠一层 Redis detail 视图缓存，会增加键管理和失效复杂度
- 当前更大的缺口是“缺少 L1 本地缓存”和“`program` model 尚未接入 go-zero cache”

因此首轮先用“L1 视图缓存 + L2 model cache”即可。

## 实施顺序

建议按以下顺序落地：

1. 为 `program-rpc` 配置新增 `Cache` 与本地缓存参数
2. 将 `DProgramModel`、`DProgramGroupModel` 切换为 go-zero cached model 结构
3. 为 `DProgramShowTimeModel.FindFirstByProgramId` 接入 `sqlc.CachedConn.QueryRowCtx`
4. 实现 `CategorySnapshotCache`
5. 实现 `ProgramDetailCache` 与 `DetailLoader`
6. 改造 `GetProgramDetailLogic` 使用 `ProgramDetailCache.Get`
7. 预留 `Invalidate` 接口，暂不接 Stream

## 测试设计

### 1. 单测

新增 `programcache` 单测，覆盖：

- L1 hit
- L1 miss 回源
- not-found 缓存
- `Invalidate(programId)`
- 分类快照缓存命中与过期

### 2. 集成测试

在 `services/program-rpc/tests/integration/` 下补充：

- 首次请求回源成功
- 第二次请求命中 L1
- 删除 DB 数据后，在 L1 TTL 内仍返回旧值
- `Invalidate(programId)` 后重新回源
- 不存在节目连续两次请求都返回 `NotFound`

### 3. model cache 测试

验证：

- `DProgramModel.FindOne` 能走 Redis cache
- `DProgramGroupModel.FindOne` 能走 Redis cache
- `FindFirstByProgramId` 自定义 key 能命中 Redis cache
- 更新或删除后能删除对应 key

## 风险

首期主要风险包括：

- `d_ticket_category.FindByProgramId` 仍可能在 L1 miss 时成为残余热点
- 后台修改链路不统一时，主动失效能力有限
- 如果 TTL 配置过长，会放大最终一致窗口
- 如果 L1 直接缓存可变对象引用，可能出现缓存污染

这些风险都可通过“短 L1 TTL + 限定范围 + 序列化缓存值”控制在可接受范围内。

## 验收标准

首期验收标准建议收敛为：

- `GetProgramDetail` 热点重复访问时，不再重复执行整条多表查询链路
- 不存在的 `programId` 不会持续打库
- Redis 不可用时服务仍可回退到 DB 查询
- 在允许的 TTL 窗口内接受短时间旧数据，但能最终收敛

## 后续演进

如果首期上线后仍观察到：

- `ticket_category` 成为明显热点
- 多节点 L1 一致性窗口不可接受
- 后台修改频率提升

再按顺序考虑：

1. 为 `FindByProgramId` 增加 Redis 列表缓存
2. 为后台修改链路接入主动失效
3. 引入 Redis Stream 做跨节点 L1 失效广播
4. 仅在无效 `programId` 流量明显时再补 Bloom
