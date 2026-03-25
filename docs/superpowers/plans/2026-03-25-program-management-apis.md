# Program Management APIs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 program 后台管理主链路接口，覆盖节目、分类、场次、票档、座位、失效与重置能力。

**Architecture:** 继续复用 `services/program-api` 作为 HTTP 入口，新增接口只做参数映射和 RPC 调用；`services/program-rpc` 承担全部写事务、校验、缓存失效和座位账本清理。对已有 `CreateProgram/UpdateProgram` 直接复用，对其余后台能力通过新增 proto + logic + model 自定义方法落地。

**Tech Stack:** Go, go-zero, goctl `--style go_zero`, gRPC, MySQL, Redis, Go integration tests

---

## File Map

### 设计文档

- Reference: `docs/superpowers/specs/2026-03-25-program-management-apis-design.md`

### program-api

- Modify: `services/program-api/program.api`
- Modify: `services/program-api/internal/types/types.go`
- Modify: `services/program-api/internal/handler/routes.go`
- Modify: `services/program-api/internal/logic/mapper.go`
- Modify: `services/program-api/tests/integration/program_logic_test.go`
- Modify: `services/program-api/tests/integration/program_rpc_fake_test.go`
- Create: `services/program-api/internal/handler/add_program_handler.go`
- Create: `services/program-api/internal/handler/update_program_handler.go`
- Create: `services/program-api/internal/handler/invalid_program_handler.go`
- Create: `services/program-api/internal/handler/list_program_categories_by_type_handler.go`
- Create: `services/program-api/internal/handler/list_program_categories_by_parent_handler.go`
- Create: `services/program-api/internal/handler/batch_create_program_categories_handler.go`
- Create: `services/program-api/internal/handler/create_program_show_time_handler.go`
- Create: `services/program-api/internal/handler/create_ticket_category_handler.go`
- Create: `services/program-api/internal/handler/get_ticket_category_detail_handler.go`
- Create: `services/program-api/internal/handler/create_seat_handler.go`
- Create: `services/program-api/internal/handler/batch_create_seats_handler.go`
- Create: `services/program-api/internal/handler/get_seat_relate_info_handler.go`
- Create: `services/program-api/internal/handler/reset_program_handler.go`
- Create: `services/program-api/internal/logic/add_program_logic.go`
- Create: `services/program-api/internal/logic/update_program_logic.go`
- Create: `services/program-api/internal/logic/invalid_program_logic.go`
- Create: `services/program-api/internal/logic/list_program_categories_by_type_logic.go`
- Create: `services/program-api/internal/logic/list_program_categories_by_parent_logic.go`
- Create: `services/program-api/internal/logic/batch_create_program_categories_logic.go`
- Create: `services/program-api/internal/logic/create_program_show_time_logic.go`
- Create: `services/program-api/internal/logic/create_ticket_category_logic.go`
- Create: `services/program-api/internal/logic/get_ticket_category_detail_logic.go`
- Create: `services/program-api/internal/logic/create_seat_logic.go`
- Create: `services/program-api/internal/logic/batch_create_seats_logic.go`
- Create: `services/program-api/internal/logic/get_seat_relate_info_logic.go`
- Create: `services/program-api/internal/logic/reset_program_logic.go`

### program-rpc

- Modify: `services/program-rpc/program.proto`
- Modify: `services/program-rpc/programrpc/program_rpc.go`
- Modify: `services/program-rpc/pb/program.pb.go`
- Modify: `services/program-rpc/pb/program_grpc.pb.go`
- Modify: `services/program-rpc/internal/server/program_rpc_server.go`
- Modify: `services/program-rpc/internal/programcache/cache_invalidator.go`
- Modify: `services/program-rpc/internal/svc/service_context.go`
- Modify: `services/program-rpc/tests/integration/program_write_logic_test.go`
- Modify: `services/program-rpc/tests/integration/program_query_logic_test.go`
- Modify: `services/program-rpc/tests/integration/program_test_helpers_test.go`
- Modify: `services/program-rpc/internal/model/d_program_category_model.go`
- Modify: `services/program-rpc/internal/model/d_program_group_model.go`
- Modify: `services/program-rpc/internal/model/d_program_show_time_model.go`
- Modify: `services/program-rpc/internal/model/d_ticket_category_model.go`
- Modify: `services/program-rpc/internal/model/d_seat_model.go`
- Create: `services/program-rpc/internal/logic/invalid_program_logic.go`
- Create: `services/program-rpc/internal/logic/reset_program_logic.go`
- Create: `services/program-rpc/internal/logic/list_program_categories_by_type_logic.go`
- Create: `services/program-rpc/internal/logic/list_program_categories_by_parent_logic.go`
- Create: `services/program-rpc/internal/logic/batch_create_program_categories_logic.go`
- Create: `services/program-rpc/internal/logic/create_program_show_time_logic.go`
- Create: `services/program-rpc/internal/logic/program_show_time_write_helper.go`
- Create: `services/program-rpc/internal/logic/create_ticket_category_logic.go`
- Create: `services/program-rpc/internal/logic/get_ticket_category_detail_logic.go`
- Create: `services/program-rpc/internal/logic/create_seat_logic.go`
- Create: `services/program-rpc/internal/logic/batch_create_seats_logic.go`
- Create: `services/program-rpc/internal/logic/get_seat_relate_info_logic.go`

## Task 1: 固化 spec 并锁定 API 层映射测试

**Files:**
- Modify: `services/program-api/tests/integration/program_logic_test.go`
- Modify: `services/program-api/tests/integration/program_rpc_fake_test.go`

- [ ] **Step 1: 先写失败测试，覆盖新增管理接口请求映射**

补以下测试：

```go
func TestAddProgramMapsRequestAndResponse(t *testing.T) {}
func TestUpdateProgramMapsRequestAndResponse(t *testing.T) {}
func TestInvalidProgramMapsRequestAndResponse(t *testing.T) {}
func TestListProgramCategoriesByTypeMapsResponse(t *testing.T) {}
func TestListProgramCategoriesByParentMapsResponse(t *testing.T) {}
func TestBatchCreateProgramCategoriesMapsRequest(t *testing.T) {}
func TestCreateProgramShowTimeMapsRequestAndResponse(t *testing.T) {}
func TestCreateTicketCategoryMapsRequestAndResponse(t *testing.T) {}
func TestGetTicketCategoryDetailMapsResponse(t *testing.T) {}
func TestCreateSeatMapsRequestAndResponse(t *testing.T) {}
func TestBatchCreateSeatsMapsRequestAndResponse(t *testing.T) {}
func TestGetSeatRelateInfoMapsResponse(t *testing.T) {}
func TestResetProgramMapsRequestAndResponse(t *testing.T) {}
```

- [ ] **Step 2: 运行 API 测试，确认因缺少实现而失败**

Run: `go test ./services/program-api/tests/integration -count=1`
Expected: FAIL，提示 fake RPC 或 logic/handler/types 尚未补齐。

## Task 2: 先补 program-api 的最小实现骨架

**Files:**
- Modify: `services/program-api/program.api`
- Modify: `services/program-api/internal/types/types.go`
- Modify: `services/program-api/internal/handler/routes.go`
- Modify: `services/program-api/internal/logic/mapper.go`
- Create: `services/program-api/internal/handler/*.go`
- Create: `services/program-api/internal/logic/*.go`

- [ ] **Step 1: 在 `program.api` 中声明新增请求与返回类型**

新增类型至少包括：

- `ProgramAddReq`
- `ProgramUpdateReq`
- `ProgramInvalidReq`
- `ProgramCategoryTypeReq`
- `ParentProgramCategoryReq`
- `ProgramCategoryBatchSaveReq`
- `ProgramShowTimeAddReq`
- `TicketCategoryAddReq`
- `TicketCategoryReq`
- `SeatAddReq`
- `SeatBatchAddReq`
- `SeatBatchRelateInfoAddReq`
- `SeatListReq`
- `SeatRelateInfoResp`
- `PriceSeatGroup`
- `IdResp`
- `BoolResp`

- [ ] **Step 2: 生成或补齐 handler/routes/types 骨架**

优先使用：

Run: `cd services/program-api && goctl api go -api program.api -dir . --style go_zero`

如果生成结果会覆盖已有自定义逻辑，则只保留新增 handler/routes/types 变更，不回退已有实现。

- [ ] **Step 3: 为每个新增接口写最小 logic，实现 RPC 透传与映射**

API logic 只负责：

- 构造 RPC request
- 调用 `svcCtx.ProgramRpc`
- 返回 `map...` 结果

- [ ] **Step 4: 运行 API 测试**

Run: `go test ./services/program-api/tests/integration -count=1`
Expected: PASS

## Task 3: 先补 program-rpc 失败测试，锁定后台写链路行为

**Files:**
- Modify: `services/program-rpc/tests/integration/program_write_logic_test.go`
- Modify: `services/program-rpc/tests/integration/program_query_logic_test.go`
- Modify: `services/program-rpc/tests/integration/program_test_helpers_test.go`

- [ ] **Step 1: 写失败测试，锁定新增 RPC 行为**

补以下测试：

```go
func TestInvalidProgramMarksProgramOffShelfAndInvalidatesCache(t *testing.T) {}
func TestResetProgramRestoresSeatStatusAndRemainNumber(t *testing.T) {}
func TestListProgramCategoriesByTypeReturnsMatchedRows(t *testing.T) {}
func TestListProgramCategoriesByParentReturnsMatchedRows(t *testing.T) {}
func TestBatchCreateProgramCategoriesRejectsDuplicateEntries(t *testing.T) {}
func TestCreateProgramShowTimePersistsRecordAndRefreshesGroupRecentShowTime(t *testing.T) {}
func TestCreateTicketCategoryPersistsRecordAndInvalidatesProgramDetailCache(t *testing.T) {}
func TestGetTicketCategoryDetailReturnsRecord(t *testing.T) {}
func TestCreateSeatRejectsDuplicateSeatCoordinate(t *testing.T) {}
func TestBatchCreateSeatsGeneratesExpectedSeatRows(t *testing.T) {}
func TestGetSeatRelateInfoGroupsSeatsByPrice(t *testing.T) {}
```

- [ ] **Step 2: 运行 program-rpc 集成测试，确认先失败**

Run: `go test ./services/program-rpc/tests/integration -run 'TestInvalidProgram|TestResetProgram|TestListProgramCategoriesByType|TestListProgramCategoriesByParent|TestBatchCreateProgramCategories|TestCreateProgramShowTime|TestCreateTicketCategory|TestGetTicketCategoryDetail|TestCreateSeat|TestBatchCreateSeats|TestGetSeatRelateInfo' -count=1`
Expected: FAIL，提示 RPC、logic 或 model 能力缺失。

## Task 4: 扩展 proto 与 server/client 生成物

**Files:**
- Modify: `services/program-rpc/program.proto`
- Modify: `services/program-rpc/programrpc/program_rpc.go`
- Modify: `services/program-rpc/pb/program.pb.go`
- Modify: `services/program-rpc/pb/program_grpc.pb.go`
- Modify: `services/program-rpc/internal/server/program_rpc_server.go`

- [ ] **Step 1: 在 `program.proto` 中新增后台管理消息与 RPC**

至少补齐：

- `IdResp`
- `ProgramInvalidReq`
- `ProgramCategoryTypeReq`
- `ParentProgramCategoryReq`
- `ProgramCategoryBatchItem`
- `ProgramCategoryBatchSaveReq`
- `ProgramShowTimeAddReq`
- `TicketCategoryAddReq`
- `TicketCategoryReq`
- `SeatAddReq`
- `SeatBatchRelateInfoAddReq`
- `SeatBatchAddReq`
- `SeatListReq`
- `SeatRelateInfo`
- `PriceSeatGroup`

- [ ] **Step 2: 重新生成 program-rpc 代码**

Run: `cd services/program-rpc && goctl rpc protoc program.proto --go_out=. --go-grpc_out=. --zrpc_out=. --style go_zero`

Expected: `pb/`、`programrpc/`、`internal/server/` 更新完成，保留已有 logic 文件。

- [ ] **Step 3: 编译一次，确认 proto 生成层无语法错误**

Run: `go test ./services/program-rpc/... -run TestDoesNotExist -count=1`
Expected: PASS 或只报告无匹配测试，不应有编译错误。

## Task 5: 实现 program-rpc 的 model 扩展与公共辅助

**Files:**
- Modify: `services/program-rpc/internal/model/d_program_category_model.go`
- Modify: `services/program-rpc/internal/model/d_program_group_model.go`
- Modify: `services/program-rpc/internal/model/d_program_show_time_model.go`
- Modify: `services/program-rpc/internal/model/d_ticket_category_model.go`
- Modify: `services/program-rpc/internal/model/d_seat_model.go`
- Modify: `services/program-rpc/internal/programcache/cache_invalidator.go`
- Modify: `services/program-rpc/internal/svc/service_context.go`
- Create: `services/program-rpc/internal/logic/program_show_time_write_helper.go`

- [ ] **Step 1: 为分类、场次、票档、座位补自定义查询/写入方法**

需要的最小能力包括：

- 分类按 `type`、`parent_id` 查询
- 分类按 `(parent_id, name, type)` 批量查重
- 场次按 `program_id` 列表查询与最早场次更新辅助
- 票档按 `id` 查询详情
- 座位按 `program_id` 查询全部票档 ID、按 `program_id + row + col` 查重

- [ ] **Step 2: 给缓存失效器补“按节目清理所有座位账本”的辅助入口**

要求：

- 通过查询该节目下所有票档或座位票档集合清理 ledger
- 不把 Redis key 拼接散落到各个 logic 文件里

- [ ] **Step 3: 运行相关编译与白盒测试**

Run: `go test ./services/program-rpc/internal/... -count=1`
Expected: PASS

## Task 6: 实现 program-rpc 后台管理逻辑

**Files:**
- Create: `services/program-rpc/internal/logic/invalid_program_logic.go`
- Create: `services/program-rpc/internal/logic/reset_program_logic.go`
- Create: `services/program-rpc/internal/logic/list_program_categories_by_type_logic.go`
- Create: `services/program-rpc/internal/logic/list_program_categories_by_parent_logic.go`
- Create: `services/program-rpc/internal/logic/batch_create_program_categories_logic.go`
- Create: `services/program-rpc/internal/logic/create_program_show_time_logic.go`
- Create: `services/program-rpc/internal/logic/create_ticket_category_logic.go`
- Create: `services/program-rpc/internal/logic/get_ticket_category_detail_logic.go`
- Create: `services/program-rpc/internal/logic/create_seat_logic.go`
- Create: `services/program-rpc/internal/logic/batch_create_seats_logic.go`
- Create: `services/program-rpc/internal/logic/get_seat_relate_info_logic.go`

- [ ] **Step 1: 先实现只够让一个测试通过的最小逻辑**

顺序建议：

1. 分类查询
2. 分类批量新增
3. 场次新增
4. 票档新增/详情
5. 座位新增/批量新增
6. 节目失效
7. 节目重置
8. 座位关联信息

- [ ] **Step 2: 每完成一组逻辑就运行对应测试，不要一次堆完**

示例：

Run: `go test ./services/program-rpc/tests/integration -run 'TestCreateProgramShowTime' -count=1`
Expected: PASS

- [ ] **Step 3: 全量运行新增 program-rpc 集成测试**

Run: `go test ./services/program-rpc/tests/integration -run 'TestInvalidProgram|TestResetProgram|TestListProgramCategoriesByType|TestListProgramCategoriesByParent|TestBatchCreateProgramCategories|TestCreateProgramShowTime|TestCreateTicketCategory|TestGetTicketCategoryDetail|TestCreateSeat|TestBatchCreateSeats|TestGetSeatRelateInfo' -count=1`
Expected: PASS

## Task 7: 联调 program-api 与 program-rpc，完成最终验证

**Files:**
- Modify: `services/program-api/internal/logic/*.go`
- Modify: `services/program-api/internal/logic/mapper.go`
- Modify: `services/program-api/tests/integration/program_logic_test.go`

- [ ] **Step 1: 补齐 API 层新增映射对象**

确保：

- `IdResp`、`BoolResp` 正确映射
- `SeatRelateInfoResp` 和价格分组结构稳定

- [ ] **Step 2: 运行 program-api 与 program-rpc 相关测试**

Run: `go test ./services/program-api/tests/integration ./services/program-rpc/tests/integration -count=1`
Expected: PASS

- [ ] **Step 3: 做一次格式化和编译校验**

Run: `gofmt -w services/program-api services/program-rpc`

Run: `go test ./services/program-api/... ./services/program-rpc/... -count=1`
Expected: PASS

- [ ] **Step 4: 核对变更边界**

确认：

- 未引入 ES 搜索相关改动
- 未新增独立 admin 服务
- 文件命名保持下划线风格
- 测试仍在 `services/<service>/tests/`
