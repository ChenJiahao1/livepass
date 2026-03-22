# Agents Chat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `damai-go` 中落地正式的 `/agent/chat` 客服能力，对外通过 `gateway-api` 暴露 HTTP 接口，对内由根级 `agents` Python 服务完成多轮会话编排并直连真实 RPC 服务。

**Architecture:** 先补齐 `order-rpc` 的客服视图与退款预检契约，避免把订单规则泄漏到 AI 服务。随后新建根级 `agents/` 服务，用 FastAPI 承接 HTTP 入口、Redis 存会话、LangGraph 编排 specialist agents，并通过 Python gRPC client 访问 `user-rpc`、`program-rpc`、`order-rpc`。最后由 `gateway-api` 统一鉴权、透传用户上下文并转发 `/agent/chat` 到 `agents`。

**Tech Stack:** Go、go-zero、Protocol Buffers、gRPC、FastAPI、LangGraph、LangChain、Redis、uv、pytest

---

### Task 1: 扩展 `order-rpc` 契约，补客服视图与退款预检

**Files:**
- Modify: `services/order-rpc/order.proto`
- Modify: `services/order-rpc/internal/server/order_rpc_server.go`
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go`
- Modify: `services/order-rpc/pb/order.pb.go`（生成）
- Modify: `services/order-rpc/pb/order_grpc.pb.go`（生成）
- Modify: `services/order-rpc/orderrpc/order_rpc.go`（生成）
- Create: `services/order-rpc/internal/logic/get_order_service_view_logic.go`
- Create: `services/order-rpc/internal/logic/preview_refund_order_logic.go`
- Test: `services/order-rpc/tests/integration/agent_order_service_logic_test.go`

- [ ] **Step 1: 阅读现有订单查询与退款逻辑，确认可复用的状态映射与退款规则入口**

Run: `sed -n '1,260p' services/order-rpc/order.proto && sed -n '1,260p' services/order-rpc/internal/logic/get_order_logic.go && sed -n '1,260p' services/order-rpc/internal/logic/refund_order_logic.go && sed -n '1,260p' services/order-rpc/internal/logic/order_domain_helper.go`
Expected: 看清现有 `GetOrder`、`RefundOrder`、`EvaluateRefundRule` 的输入输出和状态常量

- [ ] **Step 2: 先写失败的订单客服视图/退款预检集成测试**

Edit: 在 `services/order-rpc/tests/integration/agent_order_service_logic_test.go` 新增至少 4 个测试：
- `TestGetOrderServiceViewReturnsPayAndRefundHintsForOwner`
- `TestGetOrderServiceViewReturnsNotFoundForAnotherUser`
- `TestPreviewRefundOrderReturnsPreviewForRefundablePaidOrder`
- `TestPreviewRefundOrderRejectsNonRefundableOrder`

测试要求：
- 复用现有 `newOrderTestServiceContext` / seed helpers
- 断言 `payStatus`、`ticketStatus`、`canRefund`、`refundBlockedReason`
- 断言退款预检只返回 preview，不修改订单

- [ ] **Step 3: 运行新测试，确认当前实现无法通过**

Run: `go test ./services/order-rpc/tests/integration -run 'Test(GetOrderServiceView|PreviewRefundOrder)' -count=1`
Expected: FAIL，原因是 proto / server / logic 尚未定义新 RPC 或实现缺失

- [ ] **Step 4: 扩展 `order.proto` 并重新生成 Go 代码**

Edit: 在 `services/order-rpc/order.proto` 新增：
- `GetOrderServiceViewReq`
- `OrderServiceViewResp`
- `PreviewRefundOrderReq`
- `PreviewRefundOrderResp`
- `rpc GetOrderServiceView`
- `rpc PreviewRefundOrder`

Run:

```bash
cd services/order-rpc
goctl rpc protoc order.proto --go_out=. --go-grpc_out=. --zrpc_out=.
```

Expected: `pb/order.pb.go`、`pb/order_grpc.pb.go`、`orderrpc/order_rpc.go`、`internal/server/order_rpc_server.go` 刷新，包含新 RPC 签名

- [ ] **Step 5: 实现最小逻辑让测试通过**

Edit:
- `services/order-rpc/internal/logic/get_order_service_view_logic.go`
- `services/order-rpc/internal/logic/preview_refund_order_logic.go`
- `services/order-rpc/internal/logic/order_domain_helper.go`
- `services/order-rpc/internal/server/order_rpc_server.go`

实现约束：
- `GetOrderServiceView` 只做只读聚合，不改订单状态
- `PreviewRefundOrder` 复用订单域现有退款规则，不直接调用真正退款写操作
- 票券状态、支付状态、退款提示都在订单域内完成映射
- 权限校验仍以 `userId + orderNumber` 为准

- [ ] **Step 6: 重新运行目标测试并确认通过**

Run: `go test ./services/order-rpc/tests/integration -run 'Test(GetOrderServiceView|PreviewRefundOrder)' -count=1`
Expected: PASS

- [ ] **Step 7: 提交订单域补口**

Run:

```bash
git add services/order-rpc/order.proto services/order-rpc/internal/server/order_rpc_server.go services/order-rpc/internal/logic/order_domain_helper.go services/order-rpc/internal/logic/get_order_service_view_logic.go services/order-rpc/internal/logic/preview_refund_order_logic.go services/order-rpc/pb/order.pb.go services/order-rpc/pb/order_grpc.pb.go services/order-rpc/orderrpc/order_rpc.go services/order-rpc/tests/integration/agent_order_service_logic_test.go
git commit -m "feat(order-rpc): add agent service view and refund preview"
```

### Task 2: 创建根级 `agents` 服务骨架与 Python gRPC 生成链路

**Files:**
- Create: `agents/pyproject.toml`
- Create: `agents/README.md`
- Create: `agents/app/__init__.py`
- Create: `agents/app/main.py`
- Create: `agents/app/api/__init__.py`
- Create: `agents/app/api/routes.py`
- Create: `agents/app/api/schemas.py`
- Create: `agents/app/config.py`
- Create: `agents/app/clients/__init__.py`
- Create: `agents/app/clients/rpc/__init__.py`
- Create: `agents/app/clients/rpc/generated/__init__.py`
- Create: `agents/app/clients/rpc/generated/order_pb2.py`（生成）
- Create: `agents/app/clients/rpc/generated/order_pb2_grpc.py`（生成）
- Create: `agents/app/clients/rpc/generated/program_pb2.py`（生成）
- Create: `agents/app/clients/rpc/generated/program_pb2_grpc.py`（生成）
- Create: `agents/app/clients/rpc/generated/user_pb2.py`（生成）
- Create: `agents/app/clients/rpc/generated/user_pb2_grpc.py`（生成）
- Create: `agents/scripts/generate_proto_stubs.sh`
- Test: `agents/tests/test_api.py`

Reference:
- `/home/chenjiahao/code/project/customer/app/cli.py`
- `/home/chenjiahao/code/project/customer/app/config.py`

- [ ] **Step 1: 先写失败的 API 契约测试**

Edit: 在 `agents/tests/test_api.py` 新增至少 2 个测试：
- 缺少 `X-User-Id` 时返回 401/400
- 合法请求返回 JSON，且包含 `conversationId`、`reply`、`status`

测试先用 FastAPI `TestClient` + fake chat service，目标只锁定 HTTP 契约，不依赖真实 LangGraph

- [ ] **Step 2: 运行测试，确认当前目录下没有实现**

Run: `cd agents && uv run pytest tests/test_api.py -v`
Expected: FAIL，原因是 `agents` 包或 FastAPI app 尚不存在

- [ ] **Step 3: 创建最小 FastAPI 服务骨架**

Edit:
- `agents/pyproject.toml`
- `agents/app/main.py`
- `agents/app/api/routes.py`
- `agents/app/api/schemas.py`
- `agents/app/config.py`

依赖至少包含：
- `fastapi`
- `uvicorn`
- `langgraph`
- `langchain`
- `langchain-openai`
- `redis`
- `grpcio`
- `grpcio-tools`
- `pytest`
- `httpx`

骨架要求：
- 提供 `POST /agent/chat`
- 从 header 读取 `X-User-Id`
- 请求体使用显式 schema
- 返回先用占位 fake service，保证接口先可测

- [ ] **Step 4: 添加 Python gRPC stub 生成脚本**

Edit: `agents/scripts/generate_proto_stubs.sh`

脚本要求：
- 从 `services/order-rpc/order.proto`、`services/program-rpc/program.proto`、`services/user-rpc/user.proto` 生成 Python stubs
- 输出到 `agents/app/clients/rpc/generated/`
- 不手改生成文件
- 兼容重复执行

Run:

```bash
cd agents
bash scripts/generate_proto_stubs.sh
```

Expected: 生成 `order_pb2.py`、`order_pb2_grpc.py`、`program_pb2.py`、`program_pb2_grpc.py`、`user_pb2.py`、`user_pb2_grpc.py`

- [ ] **Step 5: 重新运行 API 测试，确认骨架通过**

Run: `cd agents && uv run pytest tests/test_api.py -v`
Expected: PASS

- [ ] **Step 6: 提交 `agents` 服务骨架**

Run:

```bash
git add agents/pyproject.toml agents/README.md agents/app/__init__.py agents/app/main.py agents/app/api/__init__.py agents/app/api/routes.py agents/app/api/schemas.py agents/app/config.py agents/app/clients/__init__.py agents/app/clients/rpc/__init__.py agents/app/clients/rpc/generated/__init__.py agents/app/clients/rpc/generated/order_pb2.py agents/app/clients/rpc/generated/order_pb2_grpc.py agents/app/clients/rpc/generated/program_pb2.py agents/app/clients/rpc/generated/program_pb2_grpc.py agents/app/clients/rpc/generated/user_pb2.py agents/app/clients/rpc/generated/user_pb2_grpc.py agents/scripts/generate_proto_stubs.sh agents/tests/test_api.py
git commit -m "feat(agents): scaffold chat api service"
```

### Task 3: 实现 Redis 会话存储与聊天服务编排壳层

**Files:**
- Create: `agents/app/session/__init__.py`
- Create: `agents/app/session/models.py`
- Create: `agents/app/session/store.py`
- Create: `agents/app/orchestrator/__init__.py`
- Create: `agents/app/orchestrator/state.py`
- Create: `agents/app/orchestrator/service.py`
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/config.py`
- Test: `agents/tests/test_session_store.py`
- Test: `agents/tests/test_chat_service.py`

Reference:
- `/home/chenjiahao/code/project/customer/app/graph.py`
- `/home/chenjiahao/code/project/customer/app/state.py`

- [ ] **Step 1: 写失败的会话与聊天服务测试**

Edit:
- `agents/tests/test_session_store.py`
- `agents/tests/test_chat_service.py`

覆盖场景：
- 首轮无 `conversationId` 时自动生成
- 同一 `conversationId` 必须绑定同一 `userId`
- 会话续写会追加消息并续 TTL
- service 能把 HTTP 请求映射为会话读写 + orchestrator 调用

建议使用 `fakeredis` 或自定义 in-memory fake，避免测试依赖真实 Redis

- [ ] **Step 2: 运行测试，确认当前会话实现缺失**

Run: `cd agents && uv run pytest tests/test_session_store.py tests/test_chat_service.py -v`
Expected: FAIL，原因是 session store / chat service 尚未定义

- [ ] **Step 3: 实现会话模型与 Redis store**

Edit:
- `agents/app/session/models.py`
- `agents/app/session/store.py`
- `agents/app/config.py`

实现要求：
- Redis key 含 `conversationId`
- value 含最近消息、摘要、槽位状态、handoff 信息
- 每次写入续 TTL
- 读会话时校验 `userId`

- [ ] **Step 4: 实现最小聊天服务壳层**

Edit:
- `agents/app/orchestrator/state.py`
- `agents/app/orchestrator/service.py`
- `agents/app/api/routes.py`

约束：
- 路由层只做 header / body 解析
- service 负责会话读写、调用 orchestrator、组装响应
- 暂时允许 orchestrator 返回固定 reply，先把服务壳层定稳

- [ ] **Step 5: 重新运行会话与服务测试**

Run: `cd agents && uv run pytest tests/test_session_store.py tests/test_chat_service.py -v`
Expected: PASS

- [ ] **Step 6: 提交会话层**

Run:

```bash
git add agents/app/session/__init__.py agents/app/session/models.py agents/app/session/store.py agents/app/orchestrator/__init__.py agents/app/orchestrator/state.py agents/app/orchestrator/service.py agents/app/api/routes.py agents/app/config.py agents/tests/test_session_store.py agents/tests/test_chat_service.py
git commit -m "feat(agents): add session-backed chat service"
```

### Task 4: 迁移多 Agent 编排，并替换为真实 RPC tools

**Files:**
- Create: `agents/app/orchestrator/graph.py`
- Create: `agents/app/orchestrator/agents/__init__.py`
- Create: `agents/app/orchestrator/agents/base.py`
- Create: `agents/app/orchestrator/agents/coordinator.py`
- Create: `agents/app/orchestrator/agents/supervisor.py`
- Create: `agents/app/orchestrator/agents/activity.py`
- Create: `agents/app/orchestrator/agents/order.py`
- Create: `agents/app/orchestrator/agents/refund.py`
- Create: `agents/app/orchestrator/agents/handoff.py`
- Create: `agents/app/tools/__init__.py`
- Create: `agents/app/tools/activity.py`
- Create: `agents/app/tools/order.py`
- Create: `agents/app/tools/refund.py`
- Create: `agents/app/tools/handoff.py`
- Create: `agents/app/clients/rpc/channel.py`
- Create: `agents/app/clients/rpc/order_client.py`
- Create: `agents/app/clients/rpc/program_client.py`
- Create: `agents/app/clients/rpc/user_client.py`
- Create: `agents/app/prompts/coordinator/system.md`
- Create: `agents/app/prompts/supervisor/system.md`
- Create: `agents/app/prompts/activity/system.md`
- Create: `agents/app/prompts/order/system.md`
- Create: `agents/app/prompts/refund/system.md`
- Create: `agents/app/prompts/handoff/system.md`
- Modify: `agents/app/orchestrator/service.py`
- Test: `agents/tests/test_activity_tools.py`
- Test: `agents/tests/test_order_tools.py`
- Test: `agents/tests/test_refund_tools.py`
- Test: `agents/tests/test_graph_flow.py`

Reference:
- `/home/chenjiahao/code/project/customer/app/graph.py`
- `/home/chenjiahao/code/project/customer/app/agents/*.py`
- `/home/chenjiahao/code/project/customer/prompts/*`

- [ ] **Step 1: 先写失败的 tool 与 graph 测试**

Edit:
- `agents/tests/test_activity_tools.py`
- `agents/tests/test_order_tools.py`
- `agents/tests/test_refund_tools.py`
- `agents/tests/test_graph_flow.py`

覆盖场景：
- `activity` tools 能调 `PagePrograms` / `GetProgramDetail`
- `order` tools 能调 `ListOrders` / `GetOrderServiceView`
- `refund` tools 能调 `PreviewRefundOrder` / `RefundOrder`
- graph 能在活动咨询、订单查询、退款、转人工之间完成路由

测试要求：
- 用 fake RPC client，不依赖真实服务
- 不再引用 `mock_mcp_server`
- 订单号使用真实 `int64` 语义，不再依赖 `ORD-xxxx`

- [ ] **Step 2: 运行测试，确认 orchestrator/tools 仍为空**

Run: `cd agents && uv run pytest tests/test_activity_tools.py tests/test_order_tools.py tests/test_refund_tools.py tests/test_graph_flow.py -v`
Expected: FAIL，原因是 RPC clients / tools / graph 尚未定义

- [ ] **Step 3: 实现 gRPC channel 与 client 适配**

Edit:
- `agents/app/clients/rpc/channel.py`
- `agents/app/clients/rpc/order_client.py`
- `agents/app/clients/rpc/program_client.py`
- `agents/app/clients/rpc/user_client.py`

实现要求：
- target 由配置驱动
- 每个 client 封装请求超时
- 与 Python 生成 stub 解耦
- 暴露面向工具层的薄包装方法

- [ ] **Step 4: 实现真实 tool 层**

Edit:
- `agents/app/tools/activity.py`
- `agents/app/tools/order.py`
- `agents/app/tools/refund.py`
- `agents/app/tools/handoff.py`

约束：
- 不再通过 stdio 拉起 MCP 子进程
- 直接在进程内注册 LangChain tools
- `refund` 只能通过 `PreviewRefundOrder` / `RefundOrder` 访问订单域，不直连 `pay-rpc`

- [ ] **Step 5: 迁移 coordinator/supervisor/specialist 编排**

Edit:
- `agents/app/orchestrator/graph.py`
- `agents/app/orchestrator/agents/*.py`
- `agents/app/prompts/*`
- `agents/app/orchestrator/service.py`

实现要求：
- 删除知识问答分支
- 首期只保留 `activity`、`order`、`refund`、`handoff`
- 维持多轮槽位提问能力
- graph 输出统一的 `reply/status/intent/currentAgent/needHandoff`

- [ ] **Step 6: 重新运行 orchestrator 相关测试**

Run: `cd agents && uv run pytest tests/test_activity_tools.py tests/test_order_tools.py tests/test_refund_tools.py tests/test_graph_flow.py -v`
Expected: PASS

- [ ] **Step 7: 提交编排与真实工具接入**

Run:

```bash
git add agents/app/orchestrator/graph.py agents/app/orchestrator/agents agents/app/tools agents/app/clients/rpc/channel.py agents/app/clients/rpc/order_client.py agents/app/clients/rpc/program_client.py agents/app/clients/rpc/user_client.py agents/app/prompts agents/app/orchestrator/service.py agents/tests/test_activity_tools.py agents/tests/test_order_tools.py agents/tests/test_refund_tools.py agents/tests/test_graph_flow.py
git commit -m "feat(agents): connect chat flow to rpc-backed tools"
```

### Task 5: 接入 `gateway-api`，鉴权并透传用户上下文

**Files:**
- Modify: `services/gateway-api/etc/gateway-api.yaml`
- Modify: `services/gateway-api/internal/middleware/auth_helper.go`
- Modify: `services/gateway-api/internal/middleware/auth_middleware.go`
- Modify: `services/gateway-api/tests/testkit/gateway.go`
- Modify: `services/gateway-api/tests/integration/auth_middleware_test.go`
- Modify: `services/gateway-api/tests/integration/gateway_integration_test.go`

- [ ] **Step 1: 写失败的网关测试，覆盖 `/agent/chat` 鉴权与头透传**

Edit:
- `services/gateway-api/tests/integration/auth_middleware_test.go`
- `services/gateway-api/tests/integration/gateway_integration_test.go`
- `services/gateway-api/tests/testkit/gateway.go`

至少覆盖：
- 未带 JWT 访问 `/agent/chat` 被拦截
- 已鉴权请求会被转发到 `agents`
- `X-User-Id` 会被网关注入到上游请求头

- [ ] **Step 2: 运行网关测试，确认当前实现不支持 `/agent/chat`**

Run: `go test ./services/gateway-api/tests/... -run 'TestGateway.*Agent|TestAuthMiddleware.*Agent' -count=1`
Expected: FAIL，原因是路由未映射、`/agent/` 未鉴权或 `X-User-Id` 未透传

- [ ] **Step 3: 修改网关配置与中间件**

Edit:
- `services/gateway-api/etc/gateway-api.yaml`
- `services/gateway-api/internal/middleware/auth_helper.go`
- `services/gateway-api/internal/middleware/auth_middleware.go`

实现要求：
- 在 `requiresAuth` 中纳入 `/agent/`
- 鉴权成功后，在转发请求头中写入 `X-User-Id`
- 新增 `agents` upstream，映射 `/agent/chat`

- [ ] **Step 4: 更新网关测试辅助**

Edit: `services/gateway-api/tests/testkit/gateway.go`

要求：
- `NewTestConfig` 支持 agents upstream target
- 启动测试网关时能同时挂载 user/program/order/agents 四个 upstream

- [ ] **Step 5: 重新运行网关测试**

Run: `go test ./services/gateway-api/tests/... -run 'TestGateway.*Agent|TestAuthMiddleware.*Agent' -count=1`
Expected: PASS

- [ ] **Step 6: 提交网关接入**

Run:

```bash
git add services/gateway-api/etc/gateway-api.yaml services/gateway-api/internal/middleware/auth_helper.go services/gateway-api/internal/middleware/auth_middleware.go services/gateway-api/tests/testkit/gateway.go services/gateway-api/tests/integration/auth_middleware_test.go services/gateway-api/tests/integration/gateway_integration_test.go
git commit -m "feat(gateway): forward authenticated agent chat requests"
```

### Task 6: 补运行文档、启动脚本与跨服务验收

**Files:**
- Modify: `README.md`
- Create: `agents/.env.example`
- Create: `agents/tests/test_e2e_contract.py`
- Create: `scripts/acceptance/agent_chat.sh`
- Create: `scripts/acceptance/agent_chat_cases.sh`

- [ ] **Step 1: 先写失败的端到端契约测试或验收脚本**

Edit:
- `agents/tests/test_e2e_contract.py`
- `scripts/acceptance/agent_chat.sh`
- `scripts/acceptance/agent_chat_cases.sh`

覆盖场景：
- 活动咨询
- 当前用户订单查询
- 退款预检
- 退款发起
- 系统失败转人工

其中：
- Python 侧 `test_e2e_contract.py` 可使用 fake app / fake rpc client 验证 JSON 契约
- shell 验收脚本用于真实服务联调

- [ ] **Step 2: 运行当前验收测试，确认脚本或文档尚未具备**

Run:

```bash
cd agents && uv run pytest tests/test_e2e_contract.py -v
bash scripts/acceptance/agent_chat.sh
```

Expected: pytest 初始 FAIL；shell 脚本因服务未完整接通而失败或提示缺少依赖

- [ ] **Step 3: 补充运行文档与环境示例**

Edit:
- `README.md`
- `agents/.env.example`

文档至少写清：
- `agents` 服务启动命令
- Redis / OpenAI / RPC target 配置
- `gateway-api` 暴露 `/agent/chat`
- 本地联调依赖顺序

- [ ] **Step 4: 完善验收脚本与契约测试**

Edit:
- `agents/tests/test_e2e_contract.py`
- `scripts/acceptance/agent_chat.sh`
- `scripts/acceptance/agent_chat_cases.sh`

要求：
- shell 验收脚本通过 `gateway-api` 调 `/agent/chat`
- 支持复用登录 JWT
- 至少包含一个多轮会话用例

- [ ] **Step 5: 执行最终验证**

Run:

```bash
go test ./services/order-rpc/tests/... ./services/gateway-api/tests/... -count=1
cd agents && uv run pytest tests/test_api.py tests/test_session_store.py tests/test_chat_service.py tests/test_activity_tools.py tests/test_order_tools.py tests/test_refund_tools.py tests/test_graph_flow.py tests/test_e2e_contract.py -v
git diff --stat
git status --short
```

Expected:
- Go 测试 PASS
- Python 测试 PASS
- diff 只覆盖本计划定义的文件

- [ ] **Step 6: 提交文档与验收脚本**

Run:

```bash
git add README.md agents/.env.example agents/tests/test_e2e_contract.py scripts/acceptance/agent_chat.sh scripts/acceptance/agent_chat_cases.sh
git commit -m "docs: add agent chat runbook and acceptance scripts"
```
