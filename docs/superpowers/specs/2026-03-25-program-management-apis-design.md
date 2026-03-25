# Program 管理接口补全设计

- 日期：`2026-03-25`
- 状态：`approved`
- 适用范围：`services/program-api`、`services/program-rpc`、`sql/program`
- 参考来源：原 Java 项目 `damai-program-service` 的 `ProgramController`、`ProgramCategoryController`、`ProgramShowTimeController`、`TicketCategoryController`、`SeatController`、`ProgramResetController`

## 1. 背景

当前 Go 侧 `program-api` 已经覆盖 C 端节目查询、预下单详情和系统配座锁座能力，但后台管理能力仍缺失。`program-rpc` 已经具备部分节目写链路，例如 `CreateProgram` 和 `UpdateProgram`，但 HTTP 管理入口、分类管理、票档管理、场次管理、座位管理和节目重置仍未落地。

Java 侧后台接口已经形成一套完整的节目建档与运维面。本次目标不是机械照搬全部 Java 接口，而是在保持当前 Go 服务边界不扩张的前提下，补齐“可直接支撑节目建档与运维”的最小闭环。

## 2. 目标与非目标

### 2.1 目标

1. 在 `services/program-api` 中补齐后台管理 HTTP 接口，路径尽量对齐 Java 现有接口。
2. 在 `services/program-rpc` 中补齐对应 RPC 能力，避免 API 层直接操作数据库。
3. 保持 `go-zero` 的 Handler -> Logic -> RPC 三层结构，不把业务规则塞入 `pkg/`。
4. 复用现有节目详情缓存、票档缓存和座位账本失效语义，避免新增接口引入脏缓存。
5. 为节目建档提供最小可用闭环：节目、分类、场次、票档、座位、失效、重置。

### 2.2 非目标

1. 不补 `program/search`、`program/recommend/list`，因为 Java 侧依赖 ES，当前 Go 侧没有等价搜索基础设施。
2. 不补 `program/detail/v1`、`program/detail/v2`、`program/local/detail`，这些属于 Java 侧缓存/兼容查询面，不是本轮后台主链路必需能力。
3. 不新增独立 `program-admin-api` 服务，本轮继续复用 `services/program-api`。
4. 不引入用户手动选座。项目约束明确 `program` 仅保留系统安排座位。

## 3. 接口范围

### 3.1 保留现有接口

- `/program/category/select/all`
- `/program/home/list`
- `/program/page`
- `/program/detail`
- `/program/preorder/detail`
- `/ticket/category/select/list/by/program`
- `/program/seat/freeze`

### 3.2 本轮新增 HTTP 接口

#### 节目管理

- `/program/add`
- `/program/update`
- `/program/invalid`

#### 分类管理

- `/program/category/selectByType`
- `/program/category/selectByParentProgramCategoryId`
- `/program/category/save/batch`

#### 演出时间管理

- `/program/show/time/add`

#### 票档管理

- `/ticket/category/add`
- `/ticket/category/detail`

#### 座位管理

- `/seat/add`
- `/seat/batch/add`
- `/seat/relate/info`

#### 节目运维

- `/program/reset/execute`

## 4. 总体架构

### 4.1 API 层

继续使用 `services/program-api/program.api` 统一声明接口类型和路由，再由 `goctl --style go_zero` 生成 `types`、`handler`、`routes` 骨架。每个新增接口在 `internal/logic/` 下保留单独逻辑文件，职责只包括：

- 参数映射
- 调用 `program-rpc`
- 将 RPC 返回值映射到 API 返回体

### 4.2 RPC 层

`services/program-rpc/program.proto` 新增后台管理 RPC。RPC 层统一负责：

- 参数校验
- 单库单表写入或更新
- 事务边界
- 缓存和座位账本失效

### 4.3 数据层

继续基于现有模型：

- `d_program`
- `d_program_category`
- `d_program_group`
- `d_program_show_time`
- `d_ticket_category`
- `d_seat`

必要时只扩展 `internal/model/*.go` 的自定义方法，不改 SQL 表结构。

## 5. RPC 设计

### 5.1 直接复用的 RPC

- `CreateProgram`
- `UpdateProgram`
- `ListProgramCategories`
- `GetProgramDetail`
- `ListTicketCategoriesByProgram`

### 5.2 本轮新增 RPC

- `InvalidProgram`
- `ResetProgram`
- `ListProgramCategoriesByType`
- `ListProgramCategoriesByParent`
- `BatchCreateProgramCategories`
- `CreateProgramShowTime`
- `CreateTicketCategory`
- `GetTicketCategoryDetail`
- `CreateSeat`
- `BatchCreateSeats`
- `GetSeatRelateInfo`

### 5.3 返回语义

- 新增写接口统一返回 `BoolResp` 或 `IdResp`
- 列表查询沿用现有 `ProgramCategoryListResp`
- 座位关联信息新增专用返回结构，返回节目、场次、价格分组和座位集合

这样可以让 API 层保持极薄，同时避免在 API 层拼装复杂业务数据。

## 6. 关键业务规则

### 6.1 节目失效

`InvalidProgram` 只做“逻辑下架”，即将 `d_program.program_status` 置为 `0`。执行后需要失效：

- 节目详情缓存
- 首场演出时间缓存
- 节目分组缓存

如果 Redis 座位账本已为该节目加载，也需要清理对应票档的座位账本 key，避免下架节目仍被旧缓存命中。

### 6.2 节目重置

`ResetProgram` 用于压测或演练后的回滚，语义对齐 Java：

1. 将该节目下所有 `seat_status in (2,3)` 的座位重置为 `1`
2. 清空 `freeze_token`、`freeze_expire_time`
3. 将该节目下所有票档的 `remain_number` 恢复为 `total_number`
4. 清理节目详情缓存、分组缓存、首场场次缓存和座位账本

该操作不变更节目基本信息，只恢复库存与座位售卖状态。

### 6.3 节目分类批量新增

`BatchCreateProgramCategories` 允许按 Java 后台方式批量创建分类。考虑到 `d_program_category` 当前没有唯一索引，本轮策略为：

- 允许调用方一次写入多条
- 对单批次内 `(parent_id, name, type)` 做去重校验
- 如果库内已存在同键分类，则直接返回参数错误，避免生成重复字典数据

### 6.4 演出时间新增

`CreateProgramShowTime` 在新增场次后，需要同步刷新所属 `program_group.recent_show_time`。规则如下：

- 如果该组当前没有 `recent_show_time`，直接写新场次时间
- 如果新场次早于当前 `recent_show_time`，则更新为更早时间
- 否则保持不变

### 6.5 票档新增

新增票档前必须校验节目存在。票档写入后需要失效节目详情缓存，因为详情页内嵌票档列表和价格信息。

### 6.6 座位新增与批量新增

项目约束不支持用户手动选座，但系统自动配座依赖真实座位数据，因此后台仍需维护座位。

规则：

- `CreateSeat` 校验 `(program_id, row_code, col_code)` 唯一
- `BatchCreateSeats` 按输入的票档 + 数量批量生成行列号，维持 Java 现有“每行 10 个座位”的默认算法
- 写入后清理该节目对应票档的座位账本与详情缓存

### 6.7 座位关联信息

`GetSeatRelateInfo` 返回：

- 节目 ID
- 场馆
- 场次时间
- 星期信息
- 价格列表
- `price -> []seat` 的映射

同时保留项目约束：如果节目不允许选座，返回业务错误。

## 7. 缓存与账本失效策略

当前 Go 侧已有两类缓存：

1. `go-zero` model cache / 本地详情缓存
2. Redis 座位账本 `seatcache`

本轮新增接口统一遵循以下策略：

- 节目基础信息变更：`InvalidateProgram(programID, groupID...)`
- 场次新增：失效节目详情和首场场次缓存，必要时失效节目组缓存
- 票档新增：失效节目详情缓存
- 座位新增/批量新增：清理对应 `programId + ticketCategoryId` 座位账本
- 节目重置：同时清理详情缓存、场次缓存、分组缓存和所有票档座位账本

为此需要在 `seatcache.SeatStockStore` 之上补一个“按节目清理所有票档账本”的辅助逻辑。

## 8. 测试设计

### 8.1 API 测试

放在 `services/program-api/tests/integration/`，延续当前 fake RPC 模式，覆盖：

- 请求字段是否正确映射到 RPC
- RPC 返回值是否正确映射回 HTTP response type
- 新增列表和布尔返回是否保持一致

### 8.2 RPC 集成测试

放在 `services/program-rpc/tests/integration/`，覆盖：

- 节目失效是否更新 `program_status`
- 节目重置是否恢复余票与座位状态
- 分类批量新增与去重校验
- 场次新增是否同步刷新 `program_group.recent_show_time`
- 票档新增与详情查询
- 单座位新增与批量座位新增
- 座位关联信息查询

### 8.3 验证重点

1. 新增写接口必须先有失败测试，再补实现
2. 涉及缓存的链路要验证数据落库后缓存被清理，而不是只验证数据库
3. `ResetProgram` 需要同时验证 DB 状态和座位账本状态

## 9. 风险与取舍

### 9.1 不实现 ES 搜索

这是刻意取舍。本轮如果把 `search/recommend` 一起补进来，会把 ES 索引写入、删除和测试环境都拉进范围，明显超出“节目管理接口补全”的主目标。

### 9.2 不新建 admin 服务

继续复用 `program-api` 会让 C 端接口和管理接口共存，但当前仓库没有 admin 服务独立部署约束，这样可以最快形成能力闭环，并减少网关与配置改动。

### 9.3 批量接口维持最小语义

Java 后台某些批量接口默认逻辑较粗糙，例如座位批量生成算法固定。本轮保持语义兼容，不额外设计更复杂的后台编排 DSL，避免过度建模。
