# Order Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first end-to-end `order` domain slice in `damai-go`, covering create/list/get/cancel plus scheduled close of unpaid orders with frozen-seat release.

**Architecture:** Keep the current go-zero split: `order-api` handles HTTP binding plus auth-derived `userId`, `order-rpc` owns order rules and synchronous orchestration across `program-rpc` and `user-rpc`, and `jobs/order-close` periodically triggers expired-order closure. Align Java business semantics for `d_order` and `d_order_ticket_user`, but intentionally keep the Go implementation synchronous and single-database.

**Tech Stack:** Go, go-zero, gRPC, REST, MySQL, sqlx, protobuf, goctl

---

## Scope

In scope:

- create `damai_order` bootstrap SQL and single-table order schemas
- add `order-rpc` with create/list/get/cancel/close/count RPC methods
- add `order-api` with protected HTTP routes for create/list/get/cancel
- add shared JWT parsing support for protected order routes
- implement scheduled order close job under `jobs/order-close/`
- persist order snapshot data and release seat freezes on cancel/close
- extend `README.md` with order bootstrap and manual verification

Out of scope for this phase:

- payment and pay callback
- refund and after-sales
- gateway aggregation
- order creation idempotency
- Kafka, delayed messages, or Redis order cache
- manual seat selection

## Planned File Structure

Infrastructure and docs:

- `damai-go/deploy/mysql/init/01-create-databases.sql`: create `damai_order` alongside existing databases.
- `damai-go/sql/order/d_order.sql`: single-table main order schema.
- `damai-go/sql/order/d_order_ticket_user.sql`: single-table order ticket-user snapshot schema.
- `damai-go/README.md`: add order SQL import, startup commands, auth header usage, and curl verification.

Shared auth and errors:

- `damai-go/pkg/xmiddleware/auth.go`: shared JWT parsing and `userId` context helpers for protected APIs.
- `damai-go/pkg/xmiddleware/auth_test.go`: unit tests for bearer-token and channel-code validation.
- `damai-go/pkg/xerr/errors.go`: add order-domain business errors.

`order-rpc`:

- `damai-go/services/order-rpc/order.proto`: internal order RPC contract.
- `damai-go/services/order-rpc/order.go`: RPC bootstrap entrypoint.
- `damai-go/services/order-rpc/etc/order-rpc.yaml`: local RPC config for `damai_order`.
- `damai-go/services/order-rpc/internal/config/config.go`: RPC config with MySQL, `program-rpc`, `user-rpc`, and close-after settings.
- `damai-go/services/order-rpc/internal/svc/service_context.go`: inject MySQL models plus downstream RPC clients.
- `damai-go/services/order-rpc/internal/server/order_rpc_server.go`: generated server wiring.
- `damai-go/services/order-rpc/internal/model/d_order_model.go`: custom order queries.
- `damai-go/services/order-rpc/internal/model/d_order_model_gen.go`: generated order model base.
- `damai-go/services/order-rpc/internal/model/d_order_ticket_user_model.go`: custom order detail queries and batch insert helper.
- `damai-go/services/order-rpc/internal/model/d_order_ticket_user_model_gen.go`: generated order detail model base.
- `damai-go/services/order-rpc/internal/model/vars.go`: model shared helpers.
- `damai-go/services/order-rpc/internal/logic/order_domain_helper.go`: order status constants, snapshot builders, validation helpers.
- `damai-go/services/order-rpc/internal/logic/createorderlogic.go`: create flow orchestration.
- `damai-go/services/order-rpc/internal/logic/listorderslogic.go`: list flow.
- `damai-go/services/order-rpc/internal/logic/getorderlogic.go`: detail flow.
- `damai-go/services/order-rpc/internal/logic/cancelorderlogic.go`: active user cancel flow.
- `damai-go/services/order-rpc/internal/logic/closeexpiredorderslogic.go`: scheduled close flow.
- `damai-go/services/order-rpc/internal/logic/order_test_helpers_test.go`: DB reset and fake dependency helpers.
- `damai-go/services/order-rpc/internal/logic/create_order_logic_test.go`: create-order tests.
- `damai-go/services/order-rpc/internal/logic/query_order_logic_test.go`: list/get tests.
- `damai-go/services/order-rpc/internal/logic/cancel_order_logic_test.go`: cancel tests.
- `damai-go/services/order-rpc/internal/logic/close_expired_orders_logic_test.go`: scheduled close tests.
- `damai-go/services/order-rpc/pb/order.pb.go`: generated protobuf types.
- `damai-go/services/order-rpc/pb/order_grpc.pb.go`: generated gRPC stubs.
- `damai-go/services/order-rpc/orderrpc/order_rpc.go`: generated order RPC client wrapper.

`order-api`:

- `damai-go/services/order-api/order.api`: protected HTTP contract with auth middleware.
- `damai-go/services/order-api/order.go`: API bootstrap entrypoint.
- `damai-go/services/order-api/etc/order-api.yaml`: local API config with RPC dependency and auth channel map.
- `damai-go/services/order-api/internal/config/config.go`: API config definition.
- `damai-go/services/order-api/internal/svc/service_context.go`: inject order RPC client.
- `damai-go/services/order-api/internal/middleware/authmiddleware.go`: generated middleware wrapper calling shared auth helper.
- `damai-go/services/order-api/internal/types/types.go`: generated HTTP request/response types.
- `damai-go/services/order-api/internal/handler/routes.go`: generated route wiring with middleware.
- `damai-go/services/order-api/internal/handler/createorderhandler.go`: generated create handler.
- `damai-go/services/order-api/internal/handler/listordershandler.go`: generated list handler.
- `damai-go/services/order-api/internal/handler/getorderhandler.go`: generated detail handler.
- `damai-go/services/order-api/internal/handler/cancelorderhandler.go`: generated cancel handler.
- `damai-go/services/order-api/internal/logic/mapper.go`: RPC-to-HTTP mapping helpers.
- `damai-go/services/order-api/internal/logic/createorderlogic.go`: extract `userId` from context and call RPC.
- `damai-go/services/order-api/internal/logic/listorderslogic.go`: list mapping logic.
- `damai-go/services/order-api/internal/logic/getorderlogic.go`: detail mapping logic.
- `damai-go/services/order-api/internal/logic/cancelorderlogic.go`: cancel mapping logic.
- `damai-go/services/order-api/internal/logic/order_logic_test.go`: API-side logic tests.
- `damai-go/services/order-api/internal/logic/order_rpc_fake_test.go`: fake RPC client for API tests.

`jobs/order-close`:

- `damai-go/jobs/order-close/order_close.go`: job bootstrap entrypoint.
- `damai-go/jobs/order-close/etc/order-close.yaml`: interval and batch-size config.
- `damai-go/jobs/order-close/internal/config/config.go`: job config definition.
- `damai-go/jobs/order-close/internal/svc/service_context.go`: inject `order-rpc` client.
- `damai-go/jobs/order-close/internal/logic/closeexpiredorderslogic.go`: single-run close executor.
- `damai-go/jobs/order-close/internal/logic/closeexpiredorderslogic_test.go`: job logic tests.

## Contract Notes

Key HTTP routes:

- `POST /order/create`
- `POST /order/select/list`
- `POST /order/get`
- `POST /order/cancel`

All four routes should be protected. Phase 1 should require:

- `Authorization: Bearer <jwt>`
- `X-Channel-Code: 0001`

This keeps JWT validation compatible with the current user-domain implementation, where token secrets are resolved from `UserAuth.ChannelMap` by channel code.

Recommended `order-rpc` methods:

- `CreateOrder(CreateOrderReq) returns (CreateOrderResp)`
- `ListOrders(ListOrdersReq) returns (ListOrdersResp)`
- `GetOrder(GetOrderReq) returns (OrderDetailInfo)`
- `CancelOrder(CancelOrderReq) returns (BoolResp)`
- `CloseExpiredOrders(CloseExpiredOrdersReq) returns (CloseExpiredOrdersResp)`
- `CountActiveTicketsByUserProgram(CountActiveTicketsByUserProgramReq) returns (CountActiveTicketsByUserProgramResp)`

## Task 1: Bootstrap `damai_order` SQL and local docs

**Files:**
- Modify: `damai-go/deploy/mysql/init/01-create-databases.sql`
- Create: `damai-go/sql/order/d_order.sql`
- Create: `damai-go/sql/order/d_order_ticket_user.sql`
- Modify: `damai-go/README.md`

- [ ] **Step 1: Write the failing precheck**

Run:

```bash
find sql/order deploy/mysql/init -maxdepth 2 -type f | sort | rg 'd_order|d_order_ticket_user|01-create-databases.sql'
```

Expected: `sql/order` files are missing, and `01-create-databases.sql` does not yet mention `damai_order`.

- [ ] **Step 2: Add `damai_order` bootstrap**

Update `deploy/mysql/init/01-create-databases.sql` so fresh MySQL startup creates:

- `damai_user`
- `damai_program`
- `damai_order`

Use the same `utf8mb4` charset and collation style as the existing databases.

- [ ] **Step 3: Add `d_order` DDL**

Create `sql/order/d_order.sql` with at minimum these columns:

- `id bigint not null`
- `order_number bigint not null`
- `program_id bigint not null`
- `program_title varchar(256) not null`
- `program_item_picture varchar(512) default ''`
- `program_place varchar(256) not null`
- `program_show_time datetime not null`
- `program_permit_choose_seat tinyint not null`
- `user_id bigint not null`
- `distribution_mode varchar(64) default ''`
- `take_ticket_mode varchar(64) default ''`
- `ticket_count int not null`
- `order_price decimal(10,0) not null`
- `order_status tinyint not null`
- `freeze_token varchar(64) not null`
- `order_expire_time datetime not null`
- `create_order_time datetime not null`
- `cancel_order_time datetime default null`
- `create_time datetime not null`
- `edit_time datetime not null`
- `status tinyint(1) not null default 1`

Add these indexes:

- `uk_order_number(order_number)`
- `idx_user_status_time(user_id, order_status, create_order_time)`
- `idx_program_user_status(program_id, user_id, order_status)`
- `idx_close_scan(order_status, order_expire_time)`

- [ ] **Step 4: Add `d_order_ticket_user` DDL**

Create `sql/order/d_order_ticket_user.sql` with at minimum these columns:

- `id bigint not null`
- `order_number bigint not null`
- `user_id bigint not null`
- `ticket_user_id bigint not null`
- `ticket_user_name varchar(128) not null`
- `ticket_user_id_number varchar(64) not null`
- `ticket_category_id bigint not null`
- `ticket_category_name varchar(128) not null`
- `ticket_price decimal(10,0) not null`
- `seat_id bigint not null`
- `seat_row int not null`
- `seat_col int not null`
- `seat_price decimal(10,0) not null`
- `order_status tinyint not null`
- `create_order_time datetime not null`
- `create_time datetime not null`
- `edit_time datetime not null`
- `status tinyint(1) not null default 1`

Add these indexes:

- `idx_order_number(order_number)`
- `idx_user_ticket_user(user_id, ticket_user_id)`
- `idx_create_order_time(create_order_time)`

- [ ] **Step 5: Validate the new SQL**

Run:

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d mysql
for f in sql/order/d_order.sql sql/order/d_order_ticket_user.sql; do
  docker exec -i docker-compose-mysql-1 mysql -uroot -p123456 damai_order < "$f"
done
```

Expected: both imports exit successfully with no SQL error output.

- [ ] **Step 6: Update local bootstrap and manual verification docs**

Update `README.md` to add:

- order SQL import commands
- `order-rpc`, `order-api`, and `jobs/order-close` startup commands
- curl examples for create/list/get/cancel using `Authorization` and `X-Channel-Code`

- [ ] **Step 7: Commit**

```bash
git add deploy/mysql/init/01-create-databases.sql sql/order README.md
git commit -m "feat: add order domain bootstrap sql"
```

## Task 2: Scaffold `order-rpc` contract and service shell

**Files:**
- Create: `damai-go/services/order-rpc/order.proto`
- Create: `damai-go/services/order-rpc/order.go`
- Create: `damai-go/services/order-rpc/etc/order-rpc.yaml`
- Create: `damai-go/services/order-rpc/internal/config/config.go`
- Create: `damai-go/services/order-rpc/internal/svc/service_context.go`
- Create: `damai-go/services/order-rpc/internal/server/order_rpc_server.go`
- Create: `damai-go/services/order-rpc/pb/order.pb.go`
- Create: `damai-go/services/order-rpc/pb/order_grpc.pb.go`
- Create: `damai-go/services/order-rpc/orderrpc/order_rpc.go`

- [ ] **Step 1: Write the failing precheck**

Run:

```bash
find services/order-rpc -maxdepth 3 -type f | sort
```

Expected: `services/order-rpc` does not exist yet.

- [ ] **Step 2: Define the RPC contract**

Create `services/order-rpc/order.proto` with:

- common messages: `BoolResp`
- create flow: `CreateOrderReq`, `CreateOrderResp`
- list flow: `ListOrdersReq`, `ListOrdersResp`, `OrderListInfo`
- detail flow: `GetOrderReq`, `OrderDetailInfo`, `OrderTicketInfo`
- cancel flow: `CancelOrderReq`
- scheduled close flow: `CloseExpiredOrdersReq`, `CloseExpiredOrdersResp`
- limit helper: `CountActiveTicketsByUserProgramReq`, `CountActiveTicketsByUserProgramResp`

Field requirements:

- `CreateOrderReq`: `userId`, `programId`, `ticketCategoryId`, `ticketUserIds`, `distributionMode`, `takeTicketMode`
- `ListOrdersReq`: `userId`, `pageNumber`, `pageSize`, `orderStatus`
- `GetOrderReq`: `userId`, `orderNumber`
- `CancelOrderReq`: `userId`, `orderNumber`
- `CloseExpiredOrdersReq`: `limit`

`OrderDetailInfo` should include:

- main order snapshot fields
- `orderExpireTime`
- repeated `orderTicketInfoVoList`

`OrderTicketInfo` should include:

- `ticketUserId`
- `ticketUserName`
- `ticketUserIdNumber`
- `ticketCategoryId`
- `ticketCategoryName`
- `ticketPrice`
- `seatId`
- `seatRow`
- `seatCol`
- `seatPrice`

- [ ] **Step 3: Generate the RPC scaffold**

Run:

```bash
goctl rpc protoc services/order-rpc/order.proto --go_out=services/order-rpc --go-grpc_out=services/order-rpc --zrpc_out=services/order-rpc
```

Expected: generated `pb/`, `internal/server/`, and `orderrpc/` files appear.

- [ ] **Step 4: Fill config and service context**

Implement `internal/config/config.go` with:

- `zrpc.RpcServerConf`
- `xmysql.Config`
- `ProgramRpc zrpc.RpcClientConf`
- `UserRpc zrpc.RpcClientConf`
- `OrderConfig` with `CloseAfter time.Duration` defaulting to `15m`

Implement `etc/order-rpc.yaml` with:

- `Name: order.rpc`
- `ListenOn: 0.0.0.0:8082`
- `Etcd.Key: order.rpc`
- `MySQL.DataSource` pointing at `damai_order`
- downstream `ProgramRpc` and `UserRpc` etcd clients
- `Order.CloseAfter: 15m`

Implement `internal/svc/service_context.go` so production wiring includes:

- `DOrderModel`
- `DOrderTicketUserModel`
- `ProgramRpc`
- `UserRpc`
- `SqlConn`

Type `ProgramRpc` and `UserRpc` in the service context as interfaces, not concrete structs, so tests can inject fakes without network calls.

- [ ] **Step 5: Compile the empty shell**

Run:

```bash
go test ./services/order-rpc/...
```

Expected: build fails because models and logic do not exist yet.

- [ ] **Step 6: Commit**

```bash
git add services/order-rpc
git commit -m "feat: scaffold order rpc service"
```

## Task 3: Generate order models and custom query helpers

**Files:**
- Create: `damai-go/services/order-rpc/internal/model/d_order_model.go`
- Create: `damai-go/services/order-rpc/internal/model/d_order_model_gen.go`
- Create: `damai-go/services/order-rpc/internal/model/d_order_ticket_user_model.go`
- Create: `damai-go/services/order-rpc/internal/model/d_order_ticket_user_model_gen.go`
- Create: `damai-go/services/order-rpc/internal/model/vars.go`
- Modify: `damai-go/services/order-rpc/internal/svc/service_context.go`

- [ ] **Step 1: Write the failing precheck**

Run:

```bash
find services/order-rpc/internal/model -maxdepth 1 -type f | sort
```

Expected: model files are missing.

- [ ] **Step 2: Generate base models from DDL**

Run:

```bash
goctl model mysql ddl -src sql/order/d_order.sql -dir services/order-rpc/internal/model
goctl model mysql ddl -src sql/order/d_order_ticket_user.sql -dir services/order-rpc/internal/model
```

Expected: generated `*_model.go`, `*_model_gen.go`, and `vars.go` appear.

- [ ] **Step 3: Add order-model helpers**

Extend `d_order_model.go` with:

- `FindOneByOrderNumber(ctx, orderNumber)`
- `FindOneByOrderNumberForUpdate(ctx, session, orderNumber)`
- `FindPageByUserAndStatus(ctx, userId, orderStatus, pageNumber, pageSize)`
- `CountByUserProgramAndStatus(ctx, userId, programId, orderStatus)`
- `FindExpiredUnpaid(ctx, before time.Time, limit int64)`
- `UpdateCancelStatus(ctx, session, orderNumber, cancelTime)`

Keep the implementation narrow to Phase 1 fields. Do not build a generic query builder.

- [ ] **Step 4: Add order-detail model helpers**

Extend `d_order_ticket_user_model.go` with:

- `FindByOrderNumber(ctx, orderNumber)`
- `InsertBatch(ctx, session, rows []*DOrderTicketUser)`
- `UpdateCancelStatusByOrderNumber(ctx, session, orderNumber, cancelTime)`

Use one narrow batch insert helper instead of repeated single-row inserts.

- [ ] **Step 5: Wire models into service context**

Instantiate:

- `DOrderModel`
- `DOrderTicketUserModel`

from the same `sqlx.SqlConn` used by `order-rpc`.

- [ ] **Step 6: Compile model generation**

Run:

```bash
go test ./services/order-rpc/internal/model ./services/order-rpc/internal/svc
```

Expected: packages build successfully before logic is added.

- [ ] **Step 7: Commit**

```bash
git add services/order-rpc/internal/model services/order-rpc/internal/svc/service_context.go
git commit -m "feat: add order rpc models"
```

## Task 4: Write failing `order-rpc` tests first

**Files:**
- Create: `damai-go/services/order-rpc/internal/logic/order_test_helpers_test.go`
- Create: `damai-go/services/order-rpc/internal/logic/create_order_logic_test.go`
- Create: `damai-go/services/order-rpc/internal/logic/query_order_logic_test.go`
- Create: `damai-go/services/order-rpc/internal/logic/cancel_order_logic_test.go`
- Create: `damai-go/services/order-rpc/internal/logic/close_expired_orders_logic_test.go`

- [ ] **Step 1: Add order-domain test helpers**

Create `order_test_helpers_test.go` with:

- `testOrderMySQLDataSource = "root:123456@tcp(127.0.0.1:3306)/damai_order?parseTime=true"`
- DB reset helper that replays `sql/order/d_order.sql` and `sql/order/d_order_ticket_user.sql`
- seed helpers for unpaid/cancelled orders and order ticket-user rows
- fake `ProgramRpc` and `UserRpc` implementations capturing request params and configurable responses

Keep tests isolated from real RPC servers. Reuse the existing MySQL integration-test style already used in `program-rpc`.

- [ ] **Step 2: Add create-order failing tests**

Cover at least:

- create succeeds and returns a new `orderNumber`
- selected ticket users not owned by current user returns business error
- request count exceeds `perOrderLimitPurchaseCount`
- active unpaid tickets plus current request exceed `perAccountLimitPurchaseCount`
- freeze failure leaves `d_order` and `d_order_ticket_user` empty
- insert failure triggers exactly one `ReleaseSeatFreeze` compensation call

- [ ] **Step 3: Add query and cancel failing tests**

Cover at least:

- list returns only current user orders and supports `orderStatus` filter
- get returns detail rows for owner
- get of another user order returns not found
- cancel succeeds for owner unpaid order
- repeat cancel follows the chosen Phase 1 semantic consistently

For Phase 1, choose one semantic and encode it into tests before implementation:

- recommended: repeat cancel returns success and leaves state unchanged

- [ ] **Step 4: Add scheduled-close failing tests**

Cover at least:

- only `order_status = 1` and `order_expire_time <= now` rows are closed
- each closed order triggers one freeze release call
- non-expired rows remain unchanged
- the response returns the closed count

- [ ] **Step 5: Run focused tests to verify failure**

Run:

```bash
go test ./services/order-rpc/internal/logic -run 'TestCreateOrder|TestListOrders|TestGetOrder|TestCancelOrder|TestCloseExpiredOrders' -count=1
```

Expected: FAIL because the order logic files do not exist yet.

- [ ] **Step 6: Commit**

```bash
git add services/order-rpc/internal/logic/*_test.go
git commit -m "test: add failing order rpc coverage"
```

## Task 5: Implement `order-rpc` business logic

**Files:**
- Create: `damai-go/services/order-rpc/internal/logic/order_domain_helper.go`
- Create: `damai-go/services/order-rpc/internal/logic/createorderlogic.go`
- Create: `damai-go/services/order-rpc/internal/logic/listorderslogic.go`
- Create: `damai-go/services/order-rpc/internal/logic/getorderlogic.go`
- Create: `damai-go/services/order-rpc/internal/logic/cancelorderlogic.go`
- Create: `damai-go/services/order-rpc/internal/logic/closeexpiredorderslogic.go`
- Modify: `damai-go/services/order-rpc/internal/server/order_rpc_server.go`
- Modify: `damai-go/pkg/xerr/errors.go`
- Modify: `damai-go/services/order-rpc/internal/model/d_order_model.go`
- Modify: `damai-go/services/order-rpc/internal/model/d_order_ticket_user_model.go`

- [ ] **Step 1: Add order-domain errors and helper constants**

Extend `pkg/xerr/errors.go` with:

- `ErrOrderNotFound`
- `ErrOrderStatusInvalid`
- `ErrOrderTicketUserInvalid`
- `ErrOrderPurchaseLimitExceeded`

Create `order_domain_helper.go` with:

- order status constants `1` unpaid / `2` cancelled
- snapshot builders from preorder and ticket-user RPC responses
- helper that maps gRPC/code errors to the project’s current style

- [ ] **Step 2: Implement create-order logic**

Implement `CreateOrderLogic` with this exact flow:

1. validate request shape and non-empty `ticketUserIds`
2. call `ProgramRpc.GetProgramPreorder`
3. call `UserRpc.GetUserAndTicketUserList`
4. verify each requested `ticketUserId` belongs to `userId`
5. verify the request size does not exceed `perOrderLimitPurchaseCount`
6. call local `CountByUserProgramAndStatus(..., unpaid)` and verify account limit
7. call `ProgramRpc.AutoAssignAndFreezeSeats` with `requestNo` set to `fmt.Sprintf("order-%d", xid.New())`
8. calculate `orderPrice` from preorder ticket price and seat count
9. calculate `orderExpireTime = now + CloseAfter`
10. open DB transaction and insert `d_order`
11. batch insert `d_order_ticket_user`
12. on transaction failure, call `ProgramRpc.ReleaseSeatFreeze` once for compensation

Use `xid.New()` for:

- `order_number`
- table row IDs

- [ ] **Step 3: Implement list and get logic**

`ListOrdersLogic` should:

- require positive `userId`
- default `pageNumber=1` and `pageSize=10` when absent
- filter by `orderStatus` only when non-zero
- return newest orders first by `create_order_time desc`

`GetOrderLogic` should:

- query by `orderNumber`
- treat non-owner access as not found
- return detail rows ordered by `id asc`

- [ ] **Step 4: Implement cancel and scheduled-close logic**

`CancelOrderLogic` should:

- lock the order row with `FindOneByOrderNumberForUpdate`
- verify owner and unpaid status
- call `ProgramRpc.ReleaseSeatFreeze`
- update the main order and detail rows to cancelled in the same transaction
- preserve idempotent repeat-cancel semantics chosen in Task 4

`CloseExpiredOrdersLogic` should:

- fetch a limited batch via `FindExpiredUnpaid(now, limit)`
- close rows one by one through a shared internal cancel helper
- skip already-cancelled rows safely
- return `closedCount`

Holding the DB transaction only around one order at a time is acceptable for Phase 1. Do not wrap the whole batch in a single transaction.

- [ ] **Step 5: Run focused RPC tests**

Run:

```bash
go test ./services/order-rpc/internal/logic -run 'TestCreateOrder|TestListOrders|TestGetOrder|TestCancelOrder|TestCloseExpiredOrders' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run full `order-rpc` tests**

Run:

```bash
go test ./services/order-rpc/... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/xerr/errors.go services/order-rpc/internal/logic services/order-rpc/internal/model services/order-rpc/internal/server/order_rpc_server.go
git commit -m "feat: implement order rpc flows"
```

## Task 6: Scaffold protected `order-api` and shared auth parsing

**Files:**
- Modify: `damai-go/pkg/xmiddleware/auth.go`
- Create: `damai-go/pkg/xmiddleware/auth_test.go`
- Create: `damai-go/services/order-api/order.api`
- Create: `damai-go/services/order-api/order.go`
- Create: `damai-go/services/order-api/etc/order-api.yaml`
- Create: `damai-go/services/order-api/internal/config/config.go`
- Create: `damai-go/services/order-api/internal/svc/service_context.go`
- Create: `damai-go/services/order-api/internal/middleware/authmiddleware.go`
- Create: `damai-go/services/order-api/internal/types/types.go`
- Create: `damai-go/services/order-api/internal/handler/routes.go`
- Create: `damai-go/services/order-api/internal/handler/createorderhandler.go`
- Create: `damai-go/services/order-api/internal/handler/listordershandler.go`
- Create: `damai-go/services/order-api/internal/handler/getorderhandler.go`
- Create: `damai-go/services/order-api/internal/handler/cancelorderhandler.go`

- [ ] **Step 1: Write the failing precheck**

Run:

```bash
find services/order-api pkg/xmiddleware -maxdepth 3 -type f | sort | rg 'order-api|auth.go|auth_test.go'
```

Expected: `services/order-api` is missing, and `pkg/xmiddleware/auth.go` still contains the no-op middleware.

- [ ] **Step 2: Define the protected HTTP contract**

Create `services/order-api/order.api` using one `@server` block with:

- `middleware: Auth`

Define:

- `CreateOrderReq` with `programId`, `ticketCategoryId`, `ticketUserIds`, `distributionMode`, `takeTicketMode`
- `CreateOrderResp` with `orderNumber`
- `ListOrdersReq` with `pageNumber`, `pageSize`, `orderStatus`
- `ListOrdersResp` with `pageNum`, `pageSize`, `totalSize`, `list`
- `GetOrderReq` with `orderNumber`
- `OrderDetailInfo`
- `OrderTicketInfo`
- `CancelOrderReq` with `orderNumber`
- `BoolResp`

Add routes:

- `post /order/create`
- `post /order/select/list`
- `post /order/get`
- `post /order/cancel`

- [ ] **Step 3: Generate the API scaffold**

Run:

```bash
goctl api go -api services/order-api/order.api -dir services/order-api
```

Expected: generated `internal/types`, `internal/handler`, and `internal/middleware/authmiddleware.go` appear.

- [ ] **Step 4: Implement shared auth parsing**

Replace the no-op logic in `pkg/xmiddleware/auth.go` with helper functions:

- `Authenticate(r *http.Request, channelHeader string, channelMap map[string]string) (int64, error)`
- `WithUserID(ctx context.Context, userID int64) context.Context`
- `UserIDFromContext(ctx context.Context) (int64, bool)`

Behavior:

- require `Authorization: Bearer <token>`
- require `X-Channel-Code` header by default
- resolve JWT secret from the configured channel map
- parse token via `xjwt.ParseToken`
- return `userId`

Keep the shared package independent of `order-api` types.

- [ ] **Step 5: Implement generated middleware and config**

Update `services/order-api/internal/config/config.go` to include:

- `rest.RestConf`
- `OrderRpc zrpc.RpcClientConf`
- `AuthConfig` with `ChannelCodeHeader` defaulting to `X-Channel-Code`
- `AuthConfig.ChannelMap`

Update `services/order-api/etc/order-api.yaml` with:

- `Name: order-api`
- `Port: 8890`
- `OrderRpc.Etcd.Key: order.rpc`
- `Auth.ChannelCodeHeader: X-Channel-Code`
- `Auth.ChannelMap["0001"]: local-user-secret-0001`

Modify the generated `internal/middleware/authmiddleware.go` so it:

- calls `xmiddleware.Authenticate(...)`
- injects `userId` into `r.Context()`
- fails with `httpx.ErrorCtx` when auth headers are missing or invalid

- [ ] **Step 6: Run package-level auth tests**

Run:

```bash
go test ./pkg/xmiddleware ./services/order-api/internal/middleware -count=1
```

Expected: PASS for the shared auth helper and middleware package.

- [ ] **Step 7: Commit**

```bash
git add pkg/xmiddleware services/order-api
git commit -m "feat: scaffold order api with auth middleware"
```

## Task 7: Implement `order-api` logic and tests

**Files:**
- Create: `damai-go/services/order-api/internal/logic/mapper.go`
- Create: `damai-go/services/order-api/internal/logic/createorderlogic.go`
- Create: `damai-go/services/order-api/internal/logic/listorderslogic.go`
- Create: `damai-go/services/order-api/internal/logic/getorderlogic.go`
- Create: `damai-go/services/order-api/internal/logic/cancelorderlogic.go`
- Create: `damai-go/services/order-api/internal/logic/order_logic_test.go`
- Create: `damai-go/services/order-api/internal/logic/order_rpc_fake_test.go`

- [ ] **Step 1: Add API-side fake RPC client**

Create `order_rpc_fake_test.go` implementing the generated `orderrpc.OrderRpc` interface for:

- `CreateOrder`
- `ListOrders`
- `GetOrder`
- `CancelOrder`

Capture the last request so tests can assert `userId` injection from middleware context.

- [ ] **Step 2: Add failing API logic tests**

Cover at least:

- create forwards `userId` from context, not request body
- list uses default page values when omitted
- get forwards `orderNumber`
- cancel forwards `userId` and `orderNumber`
- missing `userId` in context returns unauthorized

- [ ] **Step 3: Implement API logic**

Each logic file should:

- read `userId` using `xmiddleware.UserIDFromContext`
- validate it exists
- build the RPC request
- call `svcCtx.OrderRpc`
- map the RPC response to generated HTTP types using `mapper.go`

Do not duplicate business validation already enforced in `order-rpc`.

- [ ] **Step 4: Run focused API tests**

Run:

```bash
go test ./services/order-api/internal/logic -count=1
```

Expected: PASS.

- [ ] **Step 5: Run full `order-api` tests**

Run:

```bash
go test ./services/order-api/... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/order-api/internal/logic
git commit -m "feat: implement order api flows"
```

## Task 8: Add `jobs/order-close` scheduled runner

**Files:**
- Create: `damai-go/jobs/order-close/order_close.go`
- Create: `damai-go/jobs/order-close/etc/order-close.yaml`
- Create: `damai-go/jobs/order-close/internal/config/config.go`
- Create: `damai-go/jobs/order-close/internal/svc/service_context.go`
- Create: `damai-go/jobs/order-close/internal/logic/closeexpiredorderslogic.go`
- Create: `damai-go/jobs/order-close/internal/logic/closeexpiredorderslogic_test.go`

- [ ] **Step 1: Write the failing precheck**

Run:

```bash
find jobs/order-close -maxdepth 3 -type f | sort
```

Expected: `jobs/order-close` does not exist yet.

- [ ] **Step 2: Create the job shell**

Add:

- `order_close.go` main entrypoint
- config with `Interval time.Duration` and `BatchSize int64`
- service context with `orderrpc.OrderRpc`

Use:

- `Interval: 1m`
- `BatchSize: 100`

as initial local defaults in `etc/order-close.yaml`.

- [ ] **Step 3: Implement one-shot close logic**

Implement `CloseExpiredOrdersLogic.RunOnce()` so it:

- calls `OrderRpc.CloseExpiredOrders(ctx, &pb.CloseExpiredOrdersReq{Limit: batchSize})`
- logs `closedCount`
- returns any RPC error to the caller

Keep scheduling separate from business logic so the logic package is easy to unit test.

- [ ] **Step 4: Add job loop and tests**

In `order_close.go`, create a simple ticker loop:

- run `RunOnce()` immediately at startup
- rerun on each interval tick
- stop cleanly on process context cancellation

Test `internal/logic/closeexpiredorderslogic_test.go` for:

- correct `limit` forwarding
- success path logging/return
- RPC failure propagation

- [ ] **Step 5: Run job tests**

Run:

```bash
go test ./jobs/order-close/... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add jobs/order-close
git commit -m "feat: add order close job"
```

## Task 9: Final integration verification and docs polish

**Files:**
- Modify: `damai-go/README.md`
- Modify: `damai-go/services/order-api/etc/order-api.yaml`
- Modify: `damai-go/services/order-rpc/etc/order-rpc.yaml`
- Modify: `damai-go/jobs/order-close/etc/order-close.yaml`

- [ ] **Step 1: Verify config examples and startup commands**

Make sure `README.md` includes:

- MySQL import loop for `sql/order/*.sql`
- startup commands for:
  - `go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.yaml`
  - `go run services/order-api/order.go -f services/order-api/etc/order-api.yaml`
  - `go run jobs/order-close/order_close.go -f jobs/order-close/etc/order-close.yaml`
- login example showing how to get a JWT first

- [ ] **Step 2: Add manual order verification examples**

Document curl examples for:

1. login and capture JWT
2. create order
3. list orders
4. get order detail
5. cancel order

Use these headers consistently:

```bash
-H 'Authorization: Bearer <jwt>'
-H 'X-Channel-Code: 0001'
```

- [ ] **Step 3: Run the full repository test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Sanity-check generated services build**

Run:

```bash
go test ./services/order-api/... ./services/order-rpc/... ./jobs/order-close/... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md services/order-api/etc/order-api.yaml services/order-rpc/etc/order-rpc.yaml jobs/order-close/etc/order-close.yaml
git commit -m "docs: add order phase1 verification guide"
```
