# Program Domain Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first read-only `program` domain slice in `damai-go`, covering program category list, home list, page list, detail, and ticket-category-by-program endpoints with Java-compatible HTTP paths.

**Architecture:** Keep the existing go-zero split: `program-api` handles HTTP request/response binding only, while `program-rpc` owns MySQL-backed query composition. Phase 1 stays single-database, single-table, and intentionally excludes write APIs, Redis/local cache, Elasticsearch, seat logic, and cross-service area enrichment so `order` can depend on a stable `program` contract next.

**Tech Stack:** Go, go-zero, gRPC, REST, MySQL, protobuf, goctl, sqlx

---

## Scope

In scope:

- `POST /program/category/select/all`
- `POST /program/home/list`
- `POST /program/page`
- `POST /program/detail`
- `POST /ticket/category/select/list/by/program`

Out of scope for this phase:

- `program/search`
- `program/recommend/list`
- `program/add`
- `program/invalid`
- `program/local/detail`
- `program/show/time/add`
- `ticket/category/add`
- `ticket/category/detail`
- `seat/*`
- Redis cache, local cache, ES search, and base-data RPC

Behavior decisions for Phase 1:

- Keep Java DB-fallback semantics for `timeType` and page sorting.
- Return `areaId`, but leave `areaName` empty until `base-data` exists in Go.
- Return `programGroupVo` in detail by parsing `d_program_group.program_json`.
- Use MySQL as the only read source in this phase.

## Planned File Structure

Infrastructure and SQL:

- `damai-go/deploy/mysql/init/01-create-databases.sql`: create `damai_user` and `damai_program` on first MySQL boot.
- `damai-go/deploy/mysql/docker-compose.yml`: mount init SQL into MySQL container.
- `damai-go/deploy/docker-compose/docker-compose.infrastructure.yml`: keep the all-in-one infra bootstrap aligned with MySQL init script mount.
- `damai-go/sql/program/d_program.sql`: single-table program schema for queryable program detail fields.
- `damai-go/sql/program/d_program_category.sql`: category schema used by category list and name mapping.
- `damai-go/sql/program/d_program_group.sql`: group schema for detail aggregation.
- `damai-go/sql/program/d_program_show_time.sql`: show-time schema for time filters and detail fields.
- `damai-go/sql/program/d_ticket_category.sql`: ticket-category schema for min/max prices and detail lists.
- `damai-go/sql/program/dev_seed.sql`: minimal local seed data for manual verification.

`program-rpc`:

- `damai-go/services/program-rpc/program.proto`: internal RPC contract for the five read endpoints.
- `damai-go/services/program-rpc/program.go`: RPC service bootstrap entrypoint.
- `damai-go/services/program-rpc/etc/program-rpc.yaml`: local RPC config pointing at `damai_program`.
- `damai-go/services/program-rpc/internal/config/config.go`: RPC config definition.
- `damai-go/services/program-rpc/internal/svc/service_context.go`: MySQL model wiring.
- `damai-go/services/program-rpc/internal/server/program_rpc_server.go`: generated server adapter plus method registration.
- `damai-go/services/program-rpc/internal/model/*.go`: goctl-generated models and custom query methods.
- `damai-go/services/program-rpc/internal/logic/*.go`: read-only query logic and helpers.
- `damai-go/services/program-rpc/internal/logic/program_query_logic_test.go`: integration-style logic tests against local MySQL.
- `damai-go/services/program-rpc/internal/logic/program_test_helpers_test.go`: reusable DB reset and seed helpers.
- `damai-go/services/program-rpc/pb/*.go`: generated protobuf code.
- `damai-go/services/program-rpc/programrpc/program_rpc.go`: generated RPC client wrapper.

`program-api`:

- `damai-go/services/program-api/program.api`: HTTP contract using Java-compatible routes.
- `damai-go/services/program-api/program.go`: API service bootstrap entrypoint.
- `damai-go/services/program-api/etc/program-api.yaml`: local API config with RPC dependency.
- `damai-go/services/program-api/internal/config/config.go`: API config definition.
- `damai-go/services/program-api/internal/svc/service_context.go`: RPC client wiring.
- `damai-go/services/program-api/internal/types/types.go`: generated HTTP request/response types.
- `damai-go/services/program-api/internal/handler/*.go`: generated HTTP handlers.
- `damai-go/services/program-api/internal/logic/*.go`: RPC adapter logic only.
- `damai-go/services/program-api/internal/logic/program_rpc_fake_test.go`: fake RPC client used by API unit tests.
- `damai-go/services/program-api/internal/logic/program_logic_test.go`: route-level mapping tests for the five endpoints.

Docs:

- `damai-go/README.md`: add `program` bootstrap SQL, startup commands, and curl verification.

### Task 1: Bootstrap `damai_program` and Phase 1 SQL

**Files:**
- Create: `damai-go/deploy/mysql/init/01-create-databases.sql`
- Modify: `damai-go/deploy/mysql/docker-compose.yml`
- Modify: `damai-go/deploy/docker-compose/docker-compose.infrastructure.yml`
- Create: `damai-go/sql/program/d_program.sql`
- Create: `damai-go/sql/program/d_program_category.sql`
- Create: `damai-go/sql/program/d_program_group.sql`
- Create: `damai-go/sql/program/d_program_show_time.sql`
- Create: `damai-go/sql/program/d_ticket_category.sql`
- Create: `damai-go/sql/program/dev_seed.sql`

- [ ] **Step 1: Write the failing precheck**

Run: `find damai-go/sql/program damai-go/deploy/mysql/init -maxdepth 2 -type f | sort`

Expected: `find` reports missing paths because neither `sql/program` nor `deploy/mysql/init` exists yet.

- [ ] **Step 2: Add MySQL init bootstrap**

Write `damai-go/deploy/mysql/init/01-create-databases.sql` with:

```sql
CREATE DATABASE IF NOT EXISTS damai_user CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS damai_program CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

Mount `./init:/docker-entrypoint-initdb.d:ro` in both MySQL compose files so fresh containers create both databases automatically.

- [ ] **Step 3: Add single-table DDL for the minimum read model**

Create single-table DDL derived from Java sharded tables, but only for:

- `d_program`
- `d_program_category`
- `d_program_group`
- `d_program_show_time`
- `d_ticket_category`

Keep only fields needed by the five phase-1 endpoints plus `create_time`, `edit_time`, and `status`. Remove shard suffixes (`_0`, `_1`) and do not carry `undo_log`.

- [ ] **Step 4: Add a minimal local seed**

Seed:

- 2 root categories and 2 child categories
- 1 `d_program_group` row with valid `program_json`
- 1 `d_program` row with `prime = 1` and `program_status = 1`
- 1 `d_program_show_time` row
- 2 `d_ticket_category` rows with distinct prices

Use stable IDs so README curl examples and tests can reuse them.

- [ ] **Step 5: Validate SQL locally**

Run:

```bash
docker compose -f damai-go/deploy/docker-compose/docker-compose.infrastructure.yml up -d mysql
for f in damai-go/sql/program/d_program_category.sql damai-go/sql/program/d_program_group.sql damai-go/sql/program/d_program.sql damai-go/sql/program/d_program_show_time.sql damai-go/sql/program/d_ticket_category.sql damai-go/sql/program/dev_seed.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_program < "$f"
done
```

Expected: each import exits successfully with no SQL error output.

- [ ] **Step 6: Commit**

```bash
git -C damai-go add deploy/mysql deploy/docker-compose sql/program
git -C damai-go commit -m "feat: add program domain bootstrap sql"
```

### Task 2: Scaffold `program-rpc` contract and service shell

**Files:**
- Create: `damai-go/services/program-rpc/program.proto`
- Create: `damai-go/services/program-rpc/program.go`
- Create: `damai-go/services/program-rpc/etc/program-rpc.yaml`
- Create: `damai-go/services/program-rpc/internal/config/config.go`
- Create: `damai-go/services/program-rpc/internal/server/program_rpc_server.go`
- Create: `damai-go/services/program-rpc/internal/svc/service_context.go`
- Create: `damai-go/services/program-rpc/pb/program.pb.go`
- Create: `damai-go/services/program-rpc/pb/program_grpc.pb.go`
- Create: `damai-go/services/program-rpc/programrpc/program_rpc.go`

- [ ] **Step 1: Write the failing precheck**

Run: `find damai-go/services/program-rpc -maxdepth 3 -type f | sort`

Expected: no files found because `program-rpc` does not exist yet.

- [ ] **Step 2: Define the RPC contract**

Add `program.proto` with requests/responses for:

- `ListProgramCategories(Empty) returns (ProgramCategoryListResp)`
- `ListHomePrograms(ListHomeProgramsReq) returns (ProgramHomeListResp)`
- `PagePrograms(PageProgramsReq) returns (ProgramPageResp)`
- `GetProgramDetail(GetProgramDetailReq) returns (ProgramDetailInfo)`
- `ListTicketCategoriesByProgram(ListTicketCategoriesByProgramReq) returns (TicketCategoryDetailListResp)`

Model the response payloads after Java VOs:

- `ProgramCategoryInfo`
- `ProgramListInfo`
- `ProgramHomeSection`
- `ProgramGroupInfo`
- `ProgramSimpleInfo`
- `TicketCategoryInfo`
- `TicketCategoryDetailInfo`

- [ ] **Step 3: Generate the RPC scaffold**

Run:

```bash
goctl rpc protoc services/program-rpc/program.proto --go_out=services/program-rpc --go-grpc_out=services/program-rpc --zrpc_out=services/program-rpc
```

Expected: generated `pb/`, `internal/`, and `programrpc/` files appear under `services/program-rpc`.

- [ ] **Step 4: Fill config and service context**

Keep config minimal:

- `zrpc.RpcServerConf`
- `xmysql.Config`

Point local DSN at `damai_program`, for example:

```yaml
MySQL:
  DataSource: root:123456@tcp(127.0.0.1:3306)/damai_program?parseTime=true
```

- [ ] **Step 5: Compile the empty shell**

Run: `go test ./services/program-rpc/...`

Expected: build failure or unimplemented-method failure before models and logic are added.

- [ ] **Step 6: Commit**

```bash
git -C damai-go add services/program-rpc
git -C damai-go commit -m "feat: scaffold program rpc service"
```

### Task 3: Generate program models and wire MySQL access

**Files:**
- Create: `damai-go/services/program-rpc/internal/model/d_program_model.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_model_gen.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_category_model.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_category_model_gen.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_group_model.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_group_model_gen.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_show_time_model.go`
- Create: `damai-go/services/program-rpc/internal/model/d_program_show_time_model_gen.go`
- Create: `damai-go/services/program-rpc/internal/model/d_ticket_category_model.go`
- Create: `damai-go/services/program-rpc/internal/model/d_ticket_category_model_gen.go`
- Create: `damai-go/services/program-rpc/internal/model/vars.go`
- Modify: `damai-go/services/program-rpc/internal/svc/service_context.go`

- [ ] **Step 1: Write the failing precheck**

Run: `find damai-go/services/program-rpc/internal/model -maxdepth 1 -type f | sort`

Expected: model directory is missing or empty.

- [ ] **Step 2: Generate base models from MySQL**

Run:

```bash
goctl model mysql datasource --url "root:123456@tcp(127.0.0.1:3306)/damai_program?parseTime=true" --table d_program,d_program_category,d_program_group,d_program_show_time,d_ticket_category --dir services/program-rpc/internal/model
```

Expected: generated `*_model.go`, `*_model_gen.go`, and `vars.go`.

- [ ] **Step 3: Add custom query methods**

Add focused custom model methods instead of writing SQL inside logic:

- `DProgramCategoryModel.FindAll`
- `DProgramModel.FindHomeList`
- `DProgramModel.CountPageList`
- `DProgramModel.FindPageList`
- `DProgramGroupModel.FindOne`
- `DProgramShowTimeModel.FindByProgramIds`
- `DProgramShowTimeModel.FindFirstByProgramId`
- `DTicketCategoryModel.FindByProgramIds`
- `DTicketCategoryModel.FindByProgramId`
- `DTicketCategoryModel.FindPriceAggregateByProgramIds`

Keep filtering semantics aligned with Java DB fallback:

- `prime = 1` when `areaId` is absent
- `status = 1` and `program_status = 1`
- `timeType` date window applies on `show_day_time`
- sort `type = 2` by `high_heat desc`
- sort `type = 3` by `show_time asc`
- sort `type = 4` by `issue_time asc`

- [ ] **Step 4: Wire models into `ServiceContext`**

Inject all five models so logic files never open raw SQL connections directly.

- [ ] **Step 5: Compile the model layer**

Run: `go test ./services/program-rpc/internal/model/... ./services/program-rpc/internal/svc/...`

Expected: PASS or packages compile with `[no test files]`.

- [ ] **Step 6: Commit**

```bash
git -C damai-go add services/program-rpc/internal/model services/program-rpc/internal/svc
git -C damai-go commit -m "feat: add program domain models"
```

### Task 4: Write failing `program-rpc` query tests first

**Files:**
- Create: `damai-go/services/program-rpc/internal/logic/program_query_logic_test.go`
- Create: `damai-go/services/program-rpc/internal/logic/program_test_helpers_test.go`

- [ ] **Step 1: Add reusable test helpers**

Mirror the `user-rpc` testing style:

- local DSN constant for `damai_program`
- `newProgramTestServiceContext(t)`
- `resetProgramDomainState(t)` that replays all `sql/program/*.sql`
- `seedProgramFixtures(t, svcCtx)` for extra per-test inserts

- [ ] **Step 2: Add failing tests for the five RPC entry points**

Cover at least:

- `ListProgramCategories` returns seeded categories
- `ListHomePrograms` groups by requested parent category IDs and preserves request order
- `PagePrograms` applies `timeType`, `programCategoryId`, and sort type
- `GetProgramDetail` returns show time, category names, group info, and ticket category summary
- `ListTicketCategoriesByProgram` returns both rows with `remainNumber`
- `GetProgramDetail` returns `NotFound` for unknown program ID
- `PagePrograms` returns `InvalidArgument` when `timeType = 5` but date range is incomplete

- [ ] **Step 3: Run the tests and confirm failure**

Run: `go test ./services/program-rpc/internal/logic/...`

Expected: FAIL because methods are unimplemented or response mapping is incomplete.

- [ ] **Step 4: Commit**

```bash
git -C damai-go add services/program-rpc/internal/logic
git -C damai-go commit -m "test: add program rpc query coverage"
```

### Task 5: Implement `program-rpc` read logic

**Files:**
- Create: `damai-go/services/program-rpc/internal/logic/list_program_categories_logic.go`
- Create: `damai-go/services/program-rpc/internal/logic/list_home_programs_logic.go`
- Create: `damai-go/services/program-rpc/internal/logic/page_programs_logic.go`
- Create: `damai-go/services/program-rpc/internal/logic/get_program_detail_logic.go`
- Create: `damai-go/services/program-rpc/internal/logic/list_ticket_categories_by_program_logic.go`
- Create: `damai-go/services/program-rpc/internal/logic/program_domain_helper.go`
- Modify: `damai-go/services/program-rpc/internal/server/program_rpc_server.go`

- [ ] **Step 1: Implement shared helper functions**

Put shared, domain-local helpers in `program_domain_helper.go`:

- `applyPageTimeRange`
- `mapCategoryNameMap`
- `mapTicketPriceRange`
- `parseProgramGroupJSON`
- `toProgramListInfo`
- `toProgramDetailInfo`

Do not move these helpers into `pkg/`; they are specific to the `program` domain.

- [ ] **Step 2: Implement the simplest RPC first**

Start with `ListProgramCategories`, make its test pass, and verify names and IDs map correctly.

- [ ] **Step 3: Implement home list and page list**

Rules:

- home list groups by `parentProgramCategoryIds`
- each group uses category name from `d_program_category`
- page list returns `pageNum`, `pageSize`, `totalSize`, and `list`
- `minPrice` / `maxPrice` come from ticket-category aggregation
- `areaName` is set to `""` in phase 1

- [ ] **Step 4: Implement detail and ticket-category list**

Rules:

- detail loads `d_program`, first show time, group row, category names, and ticket-category summary
- detail returns `NotFound` if either program or first show time is missing
- `programGroupVo.programSimpleInfoVoList` is parsed from `program_json`
- by-program ticket-category list includes `totalNumber` and `remainNumber`

- [ ] **Step 5: Run focused logic tests**

Run: `go test ./services/program-rpc/internal/logic/...`

Expected: PASS.

- [ ] **Step 6: Run full RPC tests**

Run: `go test ./services/program-rpc/...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git -C damai-go add services/program-rpc
git -C damai-go commit -m "feat: implement program rpc queries"
```

### Task 6: Scaffold `program-api` HTTP contract

**Files:**
- Create: `damai-go/services/program-api/program.api`
- Create: `damai-go/services/program-api/program.go`
- Create: `damai-go/services/program-api/etc/program-api.yaml`
- Create: `damai-go/services/program-api/internal/config/config.go`
- Create: `damai-go/services/program-api/internal/svc/service_context.go`
- Create: `damai-go/services/program-api/internal/types/types.go`
- Create: `damai-go/services/program-api/internal/handler/routes.go`
- Create: `damai-go/services/program-api/internal/handler/list_program_categories_handler.go`
- Create: `damai-go/services/program-api/internal/handler/list_home_programs_handler.go`
- Create: `damai-go/services/program-api/internal/handler/page_programs_handler.go`
- Create: `damai-go/services/program-api/internal/handler/get_program_detail_handler.go`
- Create: `damai-go/services/program-api/internal/handler/list_ticket_categories_by_program_handler.go`
- Create: `damai-go/services/program-api/internal/logic/list_program_categories_logic.go`
- Create: `damai-go/services/program-api/internal/logic/list_home_programs_logic.go`
- Create: `damai-go/services/program-api/internal/logic/page_programs_logic.go`
- Create: `damai-go/services/program-api/internal/logic/get_program_detail_logic.go`
- Create: `damai-go/services/program-api/internal/logic/list_ticket_categories_by_program_logic.go`

- [ ] **Step 1: Write the failing precheck**

Run: `find damai-go/services/program-api -maxdepth 3 -type f | sort`

Expected: no files found because `program-api` does not exist yet.

- [ ] **Step 2: Define the HTTP contract**

Add routes:

- `post /program/category/select/all`
- `post /program/home/list`
- `post /program/page`
- `post /program/detail`
- `post /ticket/category/select/list/by/program`

Match request JSON fields to Java DTO names:

- `areaId`
- `parentProgramCategoryIds`
- `parentProgramCategoryId`
- `programCategoryId`
- `timeType`
- `startDateTime`
- `endDateTime`
- `pageNumber`
- `pageSize`
- `type`
- `id`
- `programId`

- [ ] **Step 3: Generate the API scaffold**

Run:

```bash
goctl api go --api services/program-api/program.api --dir services/program-api
```

Expected: generated `internal/handler`, `internal/logic`, `internal/types`, and `program.go`.

- [ ] **Step 4: Wire the RPC dependency**

Set `program-api` config to depend on `program-rpc` via etcd, matching the same local registry pattern already used by `user-api`.

- [ ] **Step 5: Compile the empty shell**

Run: `go test ./services/program-api/...`

Expected: build failure or mapping failure before logic tests and response adapters are added.

- [ ] **Step 6: Commit**

```bash
git -C damai-go add services/program-api
git -C damai-go commit -m "feat: scaffold program api service"
```

### Task 7: Write failing `program-api` mapping tests

**Files:**
- Create: `damai-go/services/program-api/internal/logic/program_rpc_fake_test.go`
- Create: `damai-go/services/program-api/internal/logic/program_logic_test.go`

- [ ] **Step 1: Add a fake RPC client**

Mirror `user-api` test style with a fake implementation that captures the last request and returns seeded protobuf responses.

- [ ] **Step 2: Add one mapping test per endpoint**

Cover at least:

- category list maps repeated RPC entries to HTTP JSON list
- home list request forwards `parentProgramCategoryIds`
- page list forwards `pageNumber`, `pageSize`, `timeType`, and maps pagination response
- detail forwards `id` and maps nested `programGroupVo` and `ticketCategoryVoList`
- ticket-category-by-program forwards `programId` and maps numeric fields correctly

- [ ] **Step 3: Run tests and confirm failure**

Run: `go test ./services/program-api/internal/logic/...`

Expected: FAIL because mapper code is not implemented yet.

- [ ] **Step 4: Commit**

```bash
git -C damai-go add services/program-api/internal/logic
git -C damai-go commit -m "test: add program api mapping coverage"
```

### Task 8: Implement `program-api` adapters, docs, and final verification

**Files:**
- Modify: `damai-go/services/program-api/internal/logic/list_program_categories_logic.go`
- Modify: `damai-go/services/program-api/internal/logic/list_home_programs_logic.go`
- Modify: `damai-go/services/program-api/internal/logic/page_programs_logic.go`
- Modify: `damai-go/services/program-api/internal/logic/get_program_detail_logic.go`
- Modify: `damai-go/services/program-api/internal/logic/list_ticket_categories_by_program_logic.go`
- Modify: `damai-go/services/program-api/internal/handler/routes.go`
- Modify: `damai-go/README.md`

- [ ] **Step 1: Implement the five API logic adapters**

Rules:

- API logic only calls RPC and maps fields
- no business rules in handlers or API logic
- keep route paths exactly aligned to the Java controller paths

- [ ] **Step 2: Add README bootstrap instructions**

Extend `README.md` with:

- MySQL bootstrap for `damai_program`
- SQL import sequence for all five program tables plus seed file
- `program-rpc` start command
- `program-api` start command
- curl examples for all five endpoints

- [ ] **Step 3: Run API logic tests**

Run: `go test ./services/program-api/internal/logic/...`

Expected: PASS.

- [ ] **Step 4: Run full project tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 5: Run manual smoke verification**

With infra plus both services running, check:

```bash
curl -X POST http://127.0.0.1:8889/program/category/select/all -H 'Content-Type: application/json' -d '{}'
curl -X POST http://127.0.0.1:8889/program/home/list -H 'Content-Type: application/json' -d '{"parentProgramCategoryIds":[1]}'
curl -X POST http://127.0.0.1:8889/program/page -H 'Content-Type: application/json' -d '{"parentProgramCategoryId":1,"timeType":0,"pageNumber":1,"pageSize":10,"type":1}'
curl -X POST http://127.0.0.1:8889/program/detail -H 'Content-Type: application/json' -d '{"id":10001}'
curl -X POST http://127.0.0.1:8889/ticket/category/select/list/by/program -H 'Content-Type: application/json' -d '{"programId":10001}'
```

Expected: all responses return HTTP 200 with non-empty seeded payloads.

- [ ] **Step 6: Commit**

```bash
git -C damai-go add services/program-api README.md
git -C damai-go commit -m "feat: add phase 1 program api"
```

## Exit Criteria

The plan is complete when all of the following are true:

- `damai_program` boots automatically in local MySQL setup
- all five phase-1 endpoints exist in both API and RPC contracts
- `program-rpc` tests pass against local MySQL
- `program-api` tests pass with fake RPC
- `go test ./...` passes at repo root
- README contains reproducible bootstrap and curl verification steps
