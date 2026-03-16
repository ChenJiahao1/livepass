# User Service Bootstrap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 `damai-go` 中先完成用户服务首轮工程初始化，落地工作区目录、基础配置、共享包占位，以及 `user-api` / `user-rpc` 的 go-zero 空服务骨架，为后续用户业务实现提供稳定起点。

**Architecture:** 先按 go-zero 规范初始化工程根目录与基础设施模板，再用最小可生成的 `.api` 与 `.proto` 契约生成 `user-api` 和 `user-rpc` 骨架。`user-api` 负责对外 HTTP 入口，`user-rpc` 负责内部统一业务入口，本轮仅打通项目结构、配置入口和依赖接线位，不实现具体用户业务。

**Tech Stack:** Go, go-zero, goctl, gRPC, etcd, MySQL, Redis, protobuf, Docker Compose

---

### Task 1: 初始化 Go 工作区

**Files:**
- Create: `go.mod`
- Create: `go.work`
- Create: `README.md`

**Step 1: 写一个失败检查**

```bash
cd /home/chenjiahao/code/project/damai-go
test -f go.mod
```

Expected: FAIL，因为仓库尚未初始化 Go 模块。

**Step 2: 初始化模块**

```bash
cd /home/chenjiahao/code/project/damai-go
go mod init damai-go
go work init .
```

**Step 3: 写最小 README**

```markdown
# damai-go

基于 Go 与 go-zero 的大麦业务总线重建项目。

当前阶段：用户服务工程初始化。
```

**Step 4: 验证文件已生成**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f go.mod && test -f go.work && test -f README.md`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add go.mod go.work README.md
git commit -m "chore: initialize damai-go workspace"
```

### Task 2: 初始化根级目录骨架

**Files:**
- Create: `docs/architecture/.gitkeep`
- Create: `docs/api/.gitkeep`
- Create: `docs/migration/.gitkeep`
- Create: `deploy/docker-compose/.gitkeep`
- Create: `deploy/etcd/.gitkeep`
- Create: `deploy/mysql/.gitkeep`
- Create: `deploy/redis/.gitkeep`
- Create: `scripts/build/.gitkeep`
- Create: `scripts/deploy/.gitkeep`
- Create: `scripts/goctl/.gitkeep`
- Create: `sql/user/.gitkeep`
- Create: `services/user-api/.gitkeep`
- Create: `services/user-rpc/.gitkeep`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -d deploy/etcd`
Expected: FAIL，因为目录尚不存在。

**Step 2: 创建目录**

```bash
cd /home/chenjiahao/code/project/damai-go
mkdir -p docs/architecture docs/api docs/migration
mkdir -p deploy/docker-compose deploy/etcd deploy/mysql deploy/redis
mkdir -p scripts/build scripts/deploy scripts/goctl
mkdir -p sql/user services/user-api services/user-rpc
```

**Step 3: 写入占位文件**

```bash
cd /home/chenjiahao/code/project/damai-go
touch docs/architecture/.gitkeep docs/api/.gitkeep docs/migration/.gitkeep
touch deploy/docker-compose/.gitkeep deploy/etcd/.gitkeep deploy/mysql/.gitkeep deploy/redis/.gitkeep
touch scripts/build/.gitkeep scripts/deploy/.gitkeep scripts/goctl/.gitkeep
touch sql/user/.gitkeep services/user-api/.gitkeep services/user-rpc/.gitkeep
```

**Step 4: 验证目录结构**

Run: `cd /home/chenjiahao/code/project/damai-go && find docs deploy scripts sql services -maxdepth 2 -type d | sort`
Expected: 输出包含上述目录，且不包含 `agents`

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add docs deploy scripts sql services
git commit -m "chore: add bootstrap directory layout"
```

### Task 3: 初始化共享基础包

**Files:**
- Create: `pkg/xerr/errors.go`
- Create: `pkg/xresponse/response.go`
- Create: `pkg/xjwt/jwt.go`
- Create: `pkg/xmysql/mysql.go`
- Create: `pkg/xredis/redis.go`
- Create: `pkg/xetcd/etcd.go`
- Create: `pkg/xmiddleware/auth.go`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f pkg/xerr/errors.go`
Expected: FAIL，因为共享包尚不存在。

**Step 2: 创建包目录**

```bash
cd /home/chenjiahao/code/project/damai-go
mkdir -p pkg/xerr pkg/xresponse pkg/xjwt pkg/xmysql pkg/xredis pkg/xetcd pkg/xmiddleware
```

**Step 3: 写最小占位代码**

```go
package xerr

import "errors"

var (
	ErrInvalidParam = errors.New("invalid param")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal error")
)
```

```go
package xresponse

type Response[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}
```

其余包保持最小可编译的初始化辅助函数或结构体占位。

**Step 4: 验证共享包可编译**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./pkg/...`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add pkg
git commit -m "chore: add shared bootstrap packages"
```

### Task 4: 增加基础设施配置模板

**Files:**
- Create: `deploy/etcd/docker-compose.yml`
- Create: `deploy/mysql/docker-compose.yml`
- Create: `deploy/redis/docker-compose.yml`
- Create: `deploy/docker-compose/docker-compose.infrastructure.yml`
- Modify: `README.md`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f deploy/etcd/docker-compose.yml`
Expected: FAIL，因为部署模板尚不存在。

**Step 2: 写本地开发模板**

```yaml
services:
  etcd:
    image: bitnami/etcd:latest
    ports:
      - "2379:2379"
```

MySQL 与 Redis 模板使用等价最小结构，聚合编排文件只负责编排三者。

**Step 3: 补充使用说明**

在 `README.md` 中增加本地基础设施启动说明：

```bash
docker compose -f deploy/docker-compose/docker-compose.infrastructure.yml up -d
```

**Step 4: 验证模板路径**

Run: `cd /home/chenjiahao/code/project/damai-go && find deploy -maxdepth 2 -type f | sort`
Expected: 输出 4 个 compose 文件

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add deploy README.md
git commit -m "chore: add local infrastructure templates"
```

### Task 5: 定义 `user-api` 最小契约

**Files:**
- Create: `services/user-api/user.api`
- Create: `services/user-api/desc/user.api`
- Create: `services/user-api/desc/ticket_user.api`
- Create: `services/user-api/desc/captcha.api`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f services/user-api/user.api`
Expected: FAIL，因为 API 契约文件不存在。

**Step 2: 写聚合入口**

```api
import "desc/user.api"
import "desc/ticket_user.api"
import "desc/captcha.api"
```

**Step 3: 写最小可生成的接口定义**

至少包含这些路径分组：

```text
/user/register
/user/login
/user/get/id
/user/update
/ticket/user/list
/ticket/user/add
/user/captcha/get
/user/captcha/verify
```

请求与响应结构先使用最小字段集合，只保证 `goctl api validate` 与后续生成通过。

**Step 4: 验证 API 契约**

Run: `cd /home/chenjiahao/code/project/damai-go && goctl api validate -api services/user-api/user.api`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-api/*.api services/user-api/desc
git commit -m "feat: add minimal user api contracts"
```

### Task 6: 生成 `user-api` go-zero 骨架

**Files:**
- Create: `services/user-api/etc/user-api.yaml`
- Create: `services/user-api/user.go`
- Create: `services/user-api/internal/config/config.go`
- Create: `services/user-api/internal/handler/...`
- Create: `services/user-api/internal/logic/...`
- Create: `services/user-api/internal/svc/servicecontext.go`
- Create: `services/user-api/internal/types/types.go`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f services/user-api/internal/config/config.go`
Expected: FAIL，因为尚未生成服务骨架。

**Step 2: 运行 goctl 生成 API 服务**

```bash
cd /home/chenjiahao/code/project/damai-go
goctl api go -api services/user-api/user.api -dir services/user-api --style go_zero
```

**Step 3: 补 API 配置模板**

在 `services/user-api/etc/user-api.yaml` 中补最小配置，至少包含：

```yaml
Name: user-api
Host: 0.0.0.0
Port: 8888
UserRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:2379
    Key: user.rpc
```

**Step 4: 验证 API 骨架**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-api/...`
Expected: PASS，或仅因 `user-rpc` 依赖尚未补齐而产生可预期错误

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-api
git commit -m "feat: generate user api scaffold"
```

### Task 7: 定义 `user-rpc` 最小契约

**Files:**
- Create: `services/user-rpc/user.proto`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f services/user-rpc/user.proto`
Expected: FAIL，因为 RPC 契约文件不存在。

**Step 2: 写最小 proto 结构**

```proto
syntax = "proto3";

package user;

option go_package = "damai-go/services/user-rpc/pb";
```

**Step 3: 定义最小服务与消息**

服务至少包含：

```text
Register
Login
GetUserById
UpdateUser
ListTicketUsers
GetCaptcha
VerifyCaptcha
```

消息体先保留最小字段，只以生成成功为目标。

**Step 4: 验证 proto 文件存在**

Run: `cd /home/chenjiahao/code/project/damai-go && test -s services/user-rpc/user.proto`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-rpc/user.proto
git commit -m "feat: add minimal user rpc contract"
```

### Task 8: 生成 `user-rpc` go-zero 骨架

**Files:**
- Create: `services/user-rpc/etc/user-rpc.yaml`
- Create: `services/user-rpc/user.go`
- Create: `services/user-rpc/internal/config/config.go`
- Create: `services/user-rpc/internal/logic/...`
- Create: `services/user-rpc/internal/server/...`
- Create: `services/user-rpc/internal/svc/servicecontext.go`
- Create: `services/user-rpc/pb/...`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && test -f services/user-rpc/internal/config/config.go`
Expected: FAIL，因为尚未生成 RPC 骨架。

**Step 2: 运行 goctl 生成 RPC 服务**

```bash
cd /home/chenjiahao/code/project/damai-go
goctl rpc protoc services/user-rpc/user.proto --go_out=./services/user-rpc --go-grpc_out=./services/user-rpc --zrpc_out=./services/user-rpc --style go_zero
```

**Step 3: 补 RPC 配置模板**

在 `services/user-rpc/etc/user-rpc.yaml` 中补最小配置，至少包含：

```yaml
Name: user.rpc
ListenOn: 0.0.0.0:8080
Etcd:
  Hosts:
    - 127.0.0.1:2379
  Key: user.rpc
MySQL:
  DataSource: root:123456@tcp(127.0.0.1:3306)/damai_user?parseTime=true
Redis:
  Host: 127.0.0.1:6379
```

**Step 4: 验证 RPC 骨架**

Run: `cd /home/chenjiahao/code/project/damai-go && go test ./services/user-rpc/...`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-rpc
git commit -m "feat: generate user rpc scaffold"
```

### Task 9: 补齐 API/RPC 配置接线位

**Files:**
- Modify: `services/user-api/internal/config/config.go`
- Modify: `services/user-api/internal/svc/servicecontext.go`
- Modify: `services/user-rpc/internal/config/config.go`
- Modify: `services/user-rpc/internal/svc/servicecontext.go`

**Step 1: 写一个失败检查**

Run: `cd /home/chenjiahao/code/project/damai-go && rg "UserRpc|MySQL|Redis|Etcd" services/user-api services/user-rpc`
Expected: 配置项不完整或接线位缺失。

**Step 2: 扩展 API 配置结构**

在 `user-api` 配置中增加 RPC 客户端配置：

```go
type Config struct {
	rest.RestConf
	UserRpc zrpc.RpcClientConf
}
```

**Step 3: 扩展 RPC 配置结构与依赖入口**

在 `user-rpc` 配置与 `ServiceContext` 中增加：

- MySQL 配置
- Redis 配置
- JWT/Auth 配置
- 后续模型注入预留位

**Step 4: 验证依赖接线**

Run: `cd /home/chenjiahao/code/project/damai-go && go build ./services/user-api ./services/user-rpc`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add services/user-api services/user-rpc
git commit -m "chore: wire bootstrap configs for user services"
```

### Task 10: 收尾验证与依赖整理

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `README.md`

**Step 1: 整理依赖**

Run: `cd /home/chenjiahao/code/project/damai-go && go mod tidy`
Expected: PASS

**Step 2: 运行全局构建验证**

Run: `cd /home/chenjiahao/code/project/damai-go && go build ./...`
Expected: PASS

**Step 3: 记录当前启动方式**

在 `README.md` 中补最小说明：

```bash
go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml
go run services/user-api/user.go -f services/user-api/etc/user-api.yaml
```

**Step 4: 最终检查目录和文档**

Run: `cd /home/chenjiahao/code/project/damai-go && find . -maxdepth 3 -type f | sort`
Expected: 输出包含工作区文件、部署模板、共享包和两个服务骨架

**Step 5: Commit**

```bash
cd /home/chenjiahao/code/project/damai-go
git add go.mod go.sum README.md services pkg deploy docs scripts sql
git commit -m "chore: finish user service bootstrap scaffolding"
```

Plan complete and saved to `docs/plans/2026-03-16-user-service-implementation.md`. Two execution options:

**1. Subagent-Driven (this session)** - 我在当前会话按任务逐步执行、每步校验后再推进

**2. Parallel Session (separate)** - 你开一个新会话，使用 `executing-plans` 按该计划分批执行

Which approach?
