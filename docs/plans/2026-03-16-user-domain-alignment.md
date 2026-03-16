# User Domain Alignment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 `damai-go` 中补齐单库单表用户域数据库、RPC、API、Redis 登录态与渠道 token 配置，使用户接口和核心行为尽量对齐原 Java 实现，验证码除外。

**Architecture:** 继续使用 go-zero 的 API -> RPC -> Model 三层结构。`user-api` 只做 HTTP 与 RPC 适配，`user-rpc` 承担业务逻辑，MySQL 和 Redis 依赖统一经 `ServiceContext` 注入，token 与错误处理放到 `pkg/` 公共层。

**Tech Stack:** Go, go-zero, gRPC, REST, MySQL, Redis, protobuf, goctl-generated model

---

### Task 1: 补齐用户域 SQL

**Files:**
- Create: `damai-go/sql/user/d_user_mobile.sql`
- Create: `damai-go/sql/user/d_user_email.sql`
- Create: `damai-go/sql/user/d_ticket_user.sql`
- Modify: `damai-go/sql/user/d_user.sql`

**Step 1: 写失败前置检查**

检查 `sql/user/` 下缺失文件，确认当前只有 `d_user.sql`。

Run: `find damai-go/sql/user -maxdepth 1 -type f | sort`
Expected: 只看到 `d_user.sql` 和占位文件

**Step 2: 写最小 SQL**

按 Java 单表结构写入 4 张表定义，包含主键、必要字段和索引。

**Step 3: 校验 SQL 内容**

Run: `sed -n '1,240p' damai-go/sql/user/d_user_mobile.sql`
Expected: 表名、字段名、索引名与 Java 单表语义一致

**Step 4: 提交**

```bash
git -C damai-go add sql/user
git -C damai-go commit -m "feat: add user domain sql schema"
```

### Task 2: 生成并补齐用户域 model

**Files:**
- Create: `damai-go/services/user-rpc/internal/model/d_user_mobile_model.go`
- Create: `damai-go/services/user-rpc/internal/model/d_user_mobile_model_gen.go`
- Create: `damai-go/services/user-rpc/internal/model/d_user_email_model.go`
- Create: `damai-go/services/user-rpc/internal/model/d_user_email_model_gen.go`
- Create: `damai-go/services/user-rpc/internal/model/d_ticket_user_model.go`
- Create: `damai-go/services/user-rpc/internal/model/d_ticket_user_model_gen.go`
- Modify: `damai-go/services/user-rpc/internal/model/d_user_model.go`
- Modify: `damai-go/services/user-rpc/internal/model/vars.go`

**Step 1: 写失败前置检查**

确认当前只有 `d_user` model。

Run: `find damai-go/services/user-rpc/internal/model -maxdepth 1 -type f | sort`
Expected: 不存在 `d_user_mobile`、`d_user_email`、`d_ticket_user` model

**Step 2: 生成或手写最小 model**

补齐 3 张新表 model，并在自定义 model 中加入这些查询：

- `d_user_mobile.FindOneByMobile`
- `d_user_email.FindOneByEmail`
- `d_ticket_user.FindByUserId`
- `d_ticket_user.FindOneByUserIdAndIdTypeAndIdNumber`

**Step 3: 运行编译验证**

Run: `go test ./services/user-rpc/internal/model/...`
Expected: PASS 或无测试文件且可正常编译

**Step 4: 提交**

```bash
git -C damai-go add services/user-rpc/internal/model
git -C damai-go commit -m "feat: add user domain models"
```

### Task 3: 扩展公共配置与依赖

**Files:**
- Modify: `damai-go/pkg/xjwt/jwt.go`
- Modify: `damai-go/pkg/xredis/redis.go`
- Modify: `damai-go/pkg/xerr/errors.go`
- Modify: `damai-go/services/user-rpc/internal/config/config.go`
- Modify: `damai-go/services/user-rpc/internal/svc/service_context.go`
- Modify: `damai-go/services/user-rpc/etc/user-rpc.yaml`

**Step 1: 写失败测试或检查**

确认当前 `user-rpc` 配置中没有 Redis 和渠道配置，`xjwt` 也没有 token 创建与解析逻辑。

Run: `sed -n '1,240p' damai-go/services/user-rpc/internal/config/config.go`
Expected: 只包含 RPCServerConf 和 MySQL

**Step 2: 写最小实现**

补齐：

- Redis 配置与客户端初始化
- 渠道配置 `code -> tokenSecret`
- token 过期时间
- 登录失败阈值
- `xjwt` 的创建与解析函数
- `xerr` 的用户域错误

**Step 3: 编译验证**

Run: `go test ./pkg/... ./services/user-rpc/internal/svc/...`
Expected: PASS

**Step 4: 提交**

```bash
git -C damai-go add pkg services/user-rpc/internal/config services/user-rpc/internal/svc services/user-rpc/etc
git -C damai-go commit -m "feat: add user rpc infrastructure config"
```

### Task 4: 扩展 RPC 契约

**Files:**
- Modify: `damai-go/services/user-rpc/user.proto`
- Modify: `damai-go/services/user-rpc/pb/user.pb.go`
- Modify: `damai-go/services/user-rpc/pb/user_grpc.pb.go`
- Modify: `damai-go/services/user-rpc/userrpc/user_rpc.go`

**Step 1: 写失败前置检查**

确认当前 proto 缺少 Java 对齐接口和字段。

Run: `sed -n '1,260p' damai-go/services/user-rpc/user.proto`
Expected: 缺失 logout、getByMobile、updatePassword、updateEmail、updateMobile、authentication、deleteTicketUser、getUserAndTicketUserList

**Step 2: 修改 proto**

补齐请求、响应和服务方法，字段命名尽量与 Java DTO/VO 一致。

**Step 3: 重新生成代码**

Run: `goctl rpc protoc services/user-rpc/user.proto --go_out=services/user-rpc --go-grpc_out=services/user-rpc --zrpc_out=services/user-rpc`
Expected: 生成文件成功

**Step 4: 编译验证**

Run: `go test ./services/user-rpc/...`
Expected: 编译通过，旧调用点按新契约修复

**Step 5: 提交**

```bash
git -C damai-go add services/user-rpc
git -C damai-go commit -m "feat: expand user rpc contract"
```

### Task 5: 先写用户 RPC 逻辑测试

**Files:**
- Modify: `damai-go/services/user-rpc/internal/logic/register_logic_test.go`
- Create: `damai-go/services/user-rpc/internal/logic/login_logic_test.go`
- Create: `damai-go/services/user-rpc/internal/logic/logout_logic_test.go`
- Create: `damai-go/services/user-rpc/internal/logic/user_query_logic_test.go`
- Create: `damai-go/services/user-rpc/internal/logic/update_user_logic_test.go`
- Create: `damai-go/services/user-rpc/internal/logic/ticket_user_logic_test.go`

**Step 1: 写失败测试**

覆盖至少这些场景：

- 注册成功
- 重复手机号注册失败
- 确认密码不一致失败
- 手机号登录成功
- 邮箱登录成功
- 密码错误失败
- 登出成功
- 按 ID、手机号查询成功
- 修改手机号和邮箱的唯一性校验
- 实名认证成功
- 购票人新增、删除、列表、聚合查询

**Step 2: 运行测试确认失败**

Run: `go test ./services/user-rpc/internal/logic/...`
Expected: FAIL，提示缺少实现或行为不符

**Step 3: 提交**

```bash
git -C damai-go add services/user-rpc/internal/logic
git -C damai-go commit -m "test: add user rpc logic coverage"
```

### Task 6: 实现用户 RPC 核心逻辑

**Files:**
- Modify: `damai-go/services/user-rpc/internal/logic/register_logic.go`
- Modify: `damai-go/services/user-rpc/internal/logic/login_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/logout_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/get_user_by_mobile_logic.go`
- Modify: `damai-go/services/user-rpc/internal/logic/get_user_by_id_logic.go`
- Modify: `damai-go/services/user-rpc/internal/logic/update_user_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/update_password_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/update_email_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/update_mobile_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/authentication_logic.go`
- Modify: `damai-go/services/user-rpc/internal/logic/list_ticket_users_logic.go`
- Modify: `damai-go/services/user-rpc/internal/logic/add_ticket_user_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/delete_ticket_user_logic.go`
- Create: `damai-go/services/user-rpc/internal/logic/get_user_and_ticket_user_list_logic.go`
- Modify: `damai-go/services/user-rpc/internal/server/user_rpc_server.go`

**Step 1: 让单测逐项通过**

逐个实现最小逻辑，不要跳过测试。

**Step 2: 跑 RPC 逻辑测试**

Run: `go test ./services/user-rpc/internal/logic/...`
Expected: PASS

**Step 3: 跑 RPC 全量测试**

Run: `go test ./services/user-rpc/...`
Expected: PASS

**Step 4: 提交**

```bash
git -C damai-go add services/user-rpc
git -C damai-go commit -m "feat: implement user rpc logic"
```

### Task 7: 更新 user-api 契约

**Files:**
- Modify: `damai-go/services/user-api/desc/user.api`
- Modify: `damai-go/services/user-api/desc/ticket_user.api`
- Modify: `damai-go/services/user-api/user.api`
- Modify: `damai-go/services/user-api/internal/types/types.go`
- Modify: `damai-go/services/user-api/internal/handler/routes.go`

**Step 1: 写失败前置检查**

确认当前 HTTP 方法和路径与 Java controller 不一致。

Run: `sed -n '1,260p' damai-go/services/user-api/user.api`
Expected: 仍存在 `GET /user/get/id` 和验证码接口

**Step 2: 修改 API 契约**

统一为 Java controller 路径，去掉验证码接口，补齐缺失接口。

**Step 3: 重新生成或修正 types/handler**

Run: `goctl api go -api services/user-api/user.api -dir services/user-api`
Expected: 生成成功，必要时保留自定义逻辑文件

**Step 4: 编译验证**

Run: `go test ./services/user-api/...`
Expected: 编译通过或进入下一步待实现

**Step 5: 提交**

```bash
git -C damai-go add services/user-api
git -C damai-go commit -m "feat: align user api contract"
```

### Task 8: 先写 user-api 逻辑测试

**Files:**
- Modify: `damai-go/services/user-api/internal/logic/register_logic_test.go`
- Create: `damai-go/services/user-api/internal/logic/login_logic_test.go`
- Create: `damai-go/services/user-api/internal/logic/logout_logic_test.go`
- Create: `damai-go/services/user-api/internal/logic/user_query_logic_test.go`
- Create: `damai-go/services/user-api/internal/logic/update_user_logic_test.go`
- Create: `damai-go/services/user-api/internal/logic/ticket_user_logic_test.go`

**Step 1: 写失败测试**

验证 API logic 能正确调用 RPC，并完成请求字段和响应字段转换。

**Step 2: 运行测试确认失败**

Run: `go test ./services/user-api/internal/logic/...`
Expected: FAIL，提示缺少逻辑实现或接口不匹配

**Step 3: 提交**

```bash
git -C damai-go add services/user-api/internal/logic
git -C damai-go commit -m "test: add user api logic coverage"
```

### Task 9: 实现 user-api 逻辑

**Files:**
- Modify: `damai-go/services/user-api/internal/logic/register_logic.go`
- Modify: `damai-go/services/user-api/internal/logic/login_logic.go`
- Create: `damai-go/services/user-api/internal/logic/logout_logic.go`
- Create: `damai-go/services/user-api/internal/logic/get_user_by_mobile_logic.go`
- Modify: `damai-go/services/user-api/internal/logic/get_user_by_i_d_logic.go`
- Modify: `damai-go/services/user-api/internal/logic/update_user_logic.go`
- Create: `damai-go/services/user-api/internal/logic/update_password_logic.go`
- Create: `damai-go/services/user-api/internal/logic/update_email_logic.go`
- Create: `damai-go/services/user-api/internal/logic/update_mobile_logic.go`
- Create: `damai-go/services/user-api/internal/logic/authentication_logic.go`
- Modify: `damai-go/services/user-api/internal/logic/list_ticket_users_logic.go`
- Modify: `damai-go/services/user-api/internal/logic/add_ticket_user_logic.go`
- Create: `damai-go/services/user-api/internal/logic/delete_ticket_user_logic.go`
- Create: `damai-go/services/user-api/internal/logic/get_user_and_ticket_user_list_logic.go`
- Modify: `damai-go/services/user-api/internal/handler/*.go`

**Step 1: 完成所有 API logic**

每个 logic 只做 RPC 调用与字段映射，不落业务逻辑。

**Step 2: 跑 API logic 测试**

Run: `go test ./services/user-api/internal/logic/...`
Expected: PASS

**Step 3: 跑 API 全量测试**

Run: `go test ./services/user-api/...`
Expected: PASS

**Step 4: 提交**

```bash
git -C damai-go add services/user-api
git -C damai-go commit -m "feat: implement user api logic"
```

### Task 10: 端到端验证

**Files:**
- Modify: `damai-go/README.md`
- Modify: `damai-go/services/user-api/etc/user-api.yaml`
- Modify: `damai-go/services/user-rpc/etc/user-rpc.yaml`

**Step 1: 启动依赖**

Run: `docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d`
Expected: MySQL、Redis、etcd 启动成功

**Step 2: 跑完整测试**

Run: `go test ./...`
Expected: PASS

**Step 3: 本地启动服务**

Run: `go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml`
Expected: user-rpc 正常监听

Run: `go run services/user-api/user.go -f services/user-api/etc/user-api.yaml`
Expected: user-api 正常监听

**Step 4: 手工验证关键接口**

Run: `curl -X POST http://127.0.0.1:8888/user/register -H 'Content-Type: application/json' -d '{"mobile":"13800000003","password":"123456","confirmPassword":"123456"}'`
Expected: 注册成功

Run: `curl -X POST http://127.0.0.1:8888/user/login -H 'Content-Type: application/json' -d '{"code":"0001","mobile":"13800000003","password":"123456"}'`
Expected: 返回 `userId` 和 `token`

Run: `curl -X POST http://127.0.0.1:8888/user/get/id -H 'Content-Type: application/json' -d '{"id":<userId>}'`
Expected: 返回脱敏用户信息

**Step 5: 更新文档**

补充 README 中的启动与验证示例。

**Step 6: 提交**

```bash
git -C damai-go add README.md services/user-api/etc services/user-rpc/etc
git -C damai-go commit -m "docs: update user service usage"
```
