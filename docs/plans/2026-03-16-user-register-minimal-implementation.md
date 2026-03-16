# User Register Minimal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 `damai-go` 中打通首条真实写链路，实现 `/user/register -> user-rpc.Register -> MySQL d_user` 的最小注册落库。

**Architecture:** `user-api` 负责 HTTP 请求解析与 RPC 转发，`user-rpc` 负责最小校验、密码摘要和 MySQL 写入。底层使用 goctl 生成的 model 访问 `d_user` 单表，不实现分片和辅助索引表。

**Tech Stack:** Go, go-zero, goctl, MySQL, sqlx, gRPC

---

### Task 1: 准备 `d_user` 单表 DDL

**Files:**
- Create: `sql/user/d_user.sql`

**Step 1: Write the failing check**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f sql/user/d_user.sql`
Expected: FAIL

**Step 2: Write the DDL**

DDL 至少包含：

```sql
CREATE TABLE IF NOT EXISTS d_user (
  id BIGINT NOT NULL,
  name VARCHAR(256) DEFAULT NULL,
  rel_name VARCHAR(256) DEFAULT NULL,
  mobile VARCHAR(512) NOT NULL,
  gender INT NOT NULL DEFAULT 1,
  password VARCHAR(512) DEFAULT NULL,
  email_status TINYINT(1) NOT NULL DEFAULT 0,
  email VARCHAR(256) DEFAULT NULL,
  rel_authentication_status TINYINT(1) NOT NULL DEFAULT 0,
  id_number VARCHAR(512) DEFAULT NULL,
  address VARCHAR(256) DEFAULT NULL,
  create_time DATETIME DEFAULT NULL,
  edit_time DATETIME DEFAULT NULL,
  status TINYINT(1) NOT NULL DEFAULT 1,
  PRIMARY KEY (id)
);
```

**Step 3: Apply the DDL**

Run: `docker exec -i mysql-mysql-1 mysql -uroot -p123456 damai_user < sql/user/d_user.sql`
Expected: PASS

**Step 4: Verify the table**

Run: `docker exec mysql-mysql-1 mysql -uroot -p123456 damai_user -e "SHOW TABLES LIKE 'd_user';"`
Expected: PASS with `d_user`

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add sql/user/d_user.sql
git commit -m "feat: add d_user ddl"
```

### Task 2: 生成并扩展 `d_user` model

**Files:**
- Create: `services/user-rpc/internal/model/dusermodel.go`
- Create: `services/user-rpc/internal/model/dusermodel_gen.go`
- Modify: `services/user-rpc/internal/model/dusermodel.go`

**Step 1: Write the failing check**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f services/user-rpc/internal/model/dusermodel.go`
Expected: FAIL

**Step 2: Generate the model**

Run: `cd /home/chenjiahao/code/project/damai-go && $(go env GOPATH)/bin/goctl model mysql ddl -src sql/user/d_user.sql -dir services/user-rpc/internal/model --style go_zero`
Expected: PASS

**Step 3: Add a custom query**

为 model 增加：

```go
FindOneByMobile(ctx context.Context, mobile string) (*DUser, error)
```

并只查询 `status = 1` 的记录。

**Step 4: Verify the model package**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/internal/model/...`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-rpc/internal/model
git commit -m "feat: add d_user model"
```

### Task 3: 接入 `user-rpc` ServiceContext

**Files:**
- Modify: `services/user-rpc/internal/config/config.go`
- Modify: `services/user-rpc/internal/svc/service_context.go`

**Step 1: Write the failing check**

Run: `cd /home/chenjiahao/code/project/damai-go && rg "DUserModel|sqlx.NewMysql" services/user-rpc/internal`
Expected: FAIL

**Step 2: Initialize the model**

在 `ServiceContext` 中初始化 MySQL 连接与 `DUserModel`。

**Step 3: Keep config compatible**

保持 `services/user-rpc/etc/user-rpc.yaml` 中 `MySQL.DataSource` 可直接用于连接。

**Step 4: Verify compile**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/internal/svc ./services/user-rpc/internal/config`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-rpc/internal/config services/user-rpc/internal/svc
git commit -m "chore: wire d_user model into service context"
```

### Task 4: 先写 `user-rpc` 注册测试

**Files:**
- Create: `services/user-rpc/internal/logic/register_logic_test.go`

**Step 1: Write the failing test**

至少包含：

- `TestRegisterInsertsUser`
- `TestRegisterRejectsDuplicateMobile`

**Step 2: Run tests to verify failure**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/internal/logic -run 'TestRegister(InsertsUser|RejectsDuplicateMobile)' -v`
Expected: FAIL

**Step 3: Use real database setup**

测试里执行清表与建表，确保可重复运行。

**Step 4: Keep scope minimal**

不要在测试里顺带验证登录或其他接口。

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-rpc/internal/logic/register_logic_test.go
git commit -m "test: add register rpc tests"
```

### Task 5: 实现 `user-rpc` 注册落库

**Files:**
- Modify: `services/user-rpc/internal/logic/register_logic.go`
- Modify: `services/user-rpc/user.proto`
- Modify: `services/user-rpc/pb/...`

**Step 1: Keep the tests red**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/internal/logic -run 'TestRegister(InsertsUser|RejectsDuplicateMobile)' -v`
Expected: FAIL

**Step 2: Implement minimal logic**

逻辑至少包含：

- 校验 `mobile` / `password` 非空
- 查询是否已存在手机号
- 生成 `id`
- `md5` 摘要密码
- 插入 `d_user`
- 返回 `BoolResp{Success:true}`

**Step 3: Re-run tests**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/internal/logic -run 'TestRegister(InsertsUser|RejectsDuplicateMobile)' -v`
Expected: PASS

**Step 4: Run full rpc tests**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/...`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-rpc
git commit -m "feat: implement register rpc persistence"
```

### Task 6: 先写 `user-api` 注册测试

**Files:**
- Create: `services/user-api/internal/logic/register_logic_test.go`

**Step 1: Write the failing test**

测试 `Register` 会调用 `UserRpc.Register` 并在成功时返回 `BoolResp{Success:true}`。

**Step 2: Run test to verify failure**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-api/internal/logic -run TestRegisterCallsRpc -v`
Expected: FAIL

**Step 3: Mock only RPC boundary**

测试内只 fake `UserRpc`，不引入 HTTP server。

**Step 4: Keep it narrow**

不要同时测试 handler 层。

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-api/internal/logic/register_logic_test.go
git commit -m "test: add register api logic test"
```

### Task 7: 实现 `user-api` 注册转发

**Files:**
- Modify: `services/user-api/internal/logic/register_logic.go`
- Modify: `services/user-api/internal/svc/service_context.go`

**Step 1: Keep the test red**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-api/internal/logic -run TestRegisterCallsRpc -v`
Expected: FAIL

**Step 2: Implement the forwarding**

将 `types.UserRegisterReq` 映射到 `userrpc.RegisterReq` 并调用 `svcCtx.UserRpc.Register(...)`。

**Step 3: Re-run the focused test**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-api/internal/logic -run TestRegisterCallsRpc -v`
Expected: PASS

**Step 4: Run full api tests**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-api/...`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-api
git commit -m "feat: wire register api to rpc"
```

### Task 8: 端到端验证注册

**Files:**
- Modify: `README.md`

**Step 1: Run final verification**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-api/... ./services/user-rpc/... && go build ./...`
Expected: PASS

**Step 2: Start the services**

Run:

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
```

**Step 3: Verify HTTP register**

Run:

```bash
curl -X POST http://127.0.0.1:8888/user/register \
  -H 'Content-Type: application/json' \
  -d '{"mobile":"13800000000","password":"123456"}'
```

Expected: 返回成功 JSON

**Step 4: Document the behavior**

在 `README.md` 中补充注册验证方式。

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add README.md services sql go.mod go.sum
git commit -m "feat: complete minimal register flow"
```
