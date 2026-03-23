# Program Detail Local Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `program-rpc` 的 `GetProgramDetail` 落地“L1 本地缓存 + go-zero model L2 Redis cache”分层缓存，减少热点详情页重复打库与重复组装。

**Architecture:** 在 `program-rpc` 内新增 `internal/programcache` 服务内缓存包，缓存最终 `pb.ProgramDetailInfo` 视图；同时为 `d_program`、`d_program_group`、`d_program_show_time` 接入 go-zero `sqlc.CachedConn`，让 L1 miss 时优先回 Redis。`d_program_category` 采用本地快照缓存，`d_ticket_category` 首期保持 L1 miss 时直查 DB。

**Tech Stack:** Go, go-zero, gRPC, `core/collection.Cache`, `core/stores/sqlc`, Redis, MySQL

---

### Task 1: 补配置与 ServiceContext 红灯测试

**Files:**
- Modify: `services/program-rpc/internal/config/config.go`
- Modify: `services/program-rpc/etc/program-rpc.yaml`
- Modify: `services/program-rpc/tests/config/config_test.go`
- Modify: `services/program-rpc/tests/integration/service_context_redis_test.go`

- [ ] 在 `services/program-rpc/tests/config/config_test.go` 为 `Cache` 与本地缓存参数补配置加载断言，先让测试表达新配置缺口。
- [ ] 在 `services/program-rpc/tests/integration/service_context_redis_test.go` 为 `ProgramDetailCache`、分类快照缓存和 cached model 注入补失败测试，明确 `StoreRedis` 配置存在时这些依赖会被正确装配。
- [ ] 在 `services/program-rpc/internal/config/config.go` 增加 `cache.CacheConf` 与本地缓存配置结构，字段至少覆盖 `DetailTTL`、`DetailNotFoundTTL`、`DetailCacheLimit`、`CategorySnapshotTTL`。
- [ ] 在 `services/program-rpc/etc/program-rpc.yaml` 补默认 `Cache` 节点和本地缓存参数，保持默认值与 spec 一致。
- [ ] 运行 `go test ./services/program-rpc/tests/config ./services/program-rpc/tests/integration -run 'TestLoadProgramRPCConfigUsesDedicatedListenPort|TestProgramServiceContextRedis'`，确认新增断言先红后绿。

### Task 2: 为单行查询接入 go-zero L2 Redis cache

**Files:**
- Modify: `services/program-rpc/internal/model/d_program_model.go`
- Modify: `services/program-rpc/internal/model/d_program_model_gen.go`
- Modify: `services/program-rpc/internal/model/d_program_group_model.go`
- Modify: `services/program-rpc/internal/model/d_program_group_model_gen.go`
- Modify: `services/program-rpc/internal/model/d_program_show_time_model.go`
- Modify: `services/program-rpc/internal/model/d_program_show_time_model_gen.go`
- Modify: `services/program-rpc/internal/svc/service_context.go`
- Test: `services/program-rpc/tests/integration/program_query_logic_test.go`

- [ ] 先用临时目录执行 `goctl model mysql ddl --src sql/program/d_program.sql --dir <tmp> --cache --style go_zero` 和对应的 `d_program_group`、`d_program_show_time` 生成命令，只把输出作为结构参考，不直接覆盖仓库文件。
- [ ] 将 `d_program_model` 和 `d_program_group_model` 的 `default*Model` 改为嵌入 `sqlc.CachedConn`，保留现有接口与自定义语义不变。
- [ ] 在 `d_program_show_time_model.go` 中为 `FindFirstByProgramId` 手工接入 `sqlc.CachedConn.QueryRowCtx`，缓存 key 固定为 `cache:dProgramShowTime:first:programId:<id>`。
- [ ] 在 `services/program-rpc/internal/svc/service_context.go` 中改造 model 构造，按 `StoreRedis` 与 `Cache` 配置初始化 cached model；当 Redis 未启用时回退到无缓存 model，避免开发环境直接 panic。
- [ ] 在 `services/program-rpc/tests/integration/program_query_logic_test.go` 或独立集成测试中补 L2 行为断言，至少覆盖“重复读取不改变结果”和“节目不存在仍返回 NotFound”。

### Task 3: 实现本地 L1 详情缓存与分类快照缓存

**Files:**
- Create: `services/program-rpc/internal/programcache/detail_cache.go`
- Create: `services/program-rpc/internal/programcache/detail_loader.go`
- Create: `services/program-rpc/internal/programcache/category_snapshot_cache.go`
- Create: `services/program-rpc/internal/programcache/detail_cache_test.go`
- Modify: `services/program-rpc/internal/svc/service_context.go`

- [ ] 在 `services/program-rpc/internal/programcache/detail_cache_test.go` 先写 L1 行为测试，覆盖命中、miss 回源、not-found、失效删除和反序列化保护。
- [ ] 在 `detail_cache.go` 中基于 `collection.NewCache` 实现 `ProgramDetailCache`，缓存值使用序列化字节，避免共享指针对象污染。
- [ ] 在 `category_snapshot_cache.go` 中实现 `CategorySnapshotCache`，固定 key 为 `program:category:snapshot`，只缓存分类全集。
- [ ] 在 `detail_loader.go` 中集中实现详情组装逻辑，依赖 `DProgramModel`、`DProgramShowTimeModel`、`DProgramGroupModel`、`CategorySnapshotCache` 和 `DTicketCategoryModel`。
- [ ] 在 `services/program-rpc/internal/svc/service_context.go` 中注入 `ProgramDetailCache` 与 `CategorySnapshotCache`，并将现有 model 引用传入 loader。
- [ ] 运行 `go test ./services/program-rpc/internal/programcache -run Test -count=1`，确认 L1 行为测试通过。

### Task 4: 将 GetProgramDetail 接入缓存链路

**Files:**
- Modify: `services/program-rpc/internal/logic/get_program_detail_logic.go`
- Modify: `services/program-rpc/tests/integration/program_query_logic_test.go`
- Possibly Create: `services/program-rpc/tests/integration/program_detail_cache_test.go`

- [ ] 先在集成测试里补缓存链路断言，至少覆盖“第二次请求命中 L1 仍返回相同结果”和“`Invalidate(programId)` 后重新回源”。
- [ ] 将 `GetProgramDetailLogic` 从直接多表查询改为调用 `svcCtx.ProgramDetailCache.Get(ctx, programId)`。
- [ ] 为不存在节目场景增加重复访问断言，确认 `NotFound` 在 L1 与 L2 组合下仍保持正确语义。
- [ ] 如现有 `program_query_logic_test.go` 过长，拆出 `program_detail_cache_test.go` 承载缓存专项集成测试，避免继续堆大单文件。
- [ ] 运行 `go test ./services/program-rpc/tests/integration -run 'TestGetProgramDetail|TestProgramServiceContextRedis' -count=1`，确认 `GetProgramDetail` 主路径绿灯。

### Task 5: 端到端验证与文档收尾

**Files:**
- Modify: `services/program-rpc/etc/program-rpc.yaml` if defaults need final adjustment
- Modify: `go.work.sum` only if test execution requires dependency updates

- [ ] 运行 `go test ./services/program-rpc/...`，确认 `program-rpc` 全量测试不回归。
- [ ] 运行 `go test ./services/program-api/...`，确认 API 层透传 `GetProgramDetail` 不受影响。
- [ ] 如需，使用本地 Redis 环境手工验证连续请求的命中路径，记录是否出现缓存序列化或 key 删除问题。
- [ ] 按任务边界拆分提交，建议至少分为“config + cached model”、“programcache + logic wiring”、“tests + cleanup”三次提交，保持回滚粒度清晰。
