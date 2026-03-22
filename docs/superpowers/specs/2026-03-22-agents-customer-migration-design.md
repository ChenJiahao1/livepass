# damai-go agents 迁移 customer agent team 设计

## 背景

当前 `/home/chenjiahao/code/project/damai-go/agents` 已经具备独立 Python 组件、FastAPI 入口、Redis 会话和部分 RPC tools 外壳，但内部实现仍然是第一版确定性编排：

- `Coordinator` 仅做关键词识别
- `Supervisor` 仅做意图到 specialist 的硬路由
- specialist 直接绑定本地 `StructuredTool`
- 会话状态是自定义 `ConversationSession`

该实现与 `/home/chenjiahao/code/project/customer` 中已经成型的 agent team 只有外部形态接近，核心能力并未迁入：

- 没有 `LangGraph` 运行时状态流转
- 没有 `Coordinator -> Supervisor -> Specialist` 的 LLM 决策链
- 没有 `MCPToolRegistry + MultiServerMCPClient` 的正式工具边界
- 没有 `Knowledge Agent`

本次工作的目标不是继续修补第一版壳，而是把 `customer` 的 agent 实现正式迁入 `damai-go/agents`，并按 `damai-go` 的服务边界改造成可对接真实业务服务的版本。

## 目标

重建 `damai-go/agents`，使其满足以下要求：

1. 整体控制流与 `customer` 的 agent team 一致
2. 对外通过 FastAPI 暴露 `/agent/chat`
3. 会话状态由 Redis 持久化
4. `activity/order/refund/handoff` 工具统一通过 MCP 暴露
5. MCP server 内部通过真实 `gRPC/rpc` 客户端调用 `damai-go` 服务
6. `knowledge agent` 保持对 `LightRAG` 的 HTTP 接入，不纳入 MCP 改造

## 不在本次范围内

- `knowledge` 改造成 MCP
- 新增 gateway 联调
- 引入真实人工服务
- 增加长期记忆、用户画像、埋点分析

## 总体架构

新实现拆成三层：

### 1. FastAPI 入口层

职责：

- 暴露 `/agent/chat`
- 处理请求参数与用户身份头
- 读取和保存 Redis 会话状态
- 调用 graph 并返回标准响应

非职责：

- 不进行业务路由
- 不直接调用 RPC
- 不在 route 内拼装 specialist 回复

### 2. Agent Runtime 层

采用 `customer` 的核心运行结构：

- `Coordinator`
- `Supervisor`
- `ActivityAgent`
- `OrderAgent`
- `RefundAgent`
- `HandoffAgent`
- `KnowledgeAgent`

技术方案：

- `LangGraph`
- `PromptRenderer`
- `LLM structured output`
- `Tool-calling agents`

其中：

- `Coordinator` 与 `Supervisor` 负责决策
- specialist agent 负责调用工具与组织回复
- `KnowledgeAgent` 继续直接访问 `LightRAG`

### 3. Tool 接入层

`activity/order/refund/handoff` 统一经由 MCP 暴露。

边界规则：

- agent runtime 只接触 `MCPToolRegistry`
- MCP server 内部适配 `damai-go` RPC client
- RPC client 不暴露给 FastAPI 或 graph

这样可以保留 `customer` 的正式工具边界，也便于未来替换 mock/real 或独立调试工具能力。

## 推荐目录结构

```text
agents/
├── app/
│   ├── api/
│   │   ├── routes.py
│   │   └── schemas.py
│   ├── agents/
│   │   ├── base.py
│   │   ├── coordinator.py
│   │   ├── supervisor.py
│   │   ├── activity.py
│   │   ├── order.py
│   │   ├── refund.py
│   │   ├── handoff.py
│   │   └── knowledge.py
│   ├── llm/
│   │   ├── client.py
│   │   └── schemas.py
│   ├── mcp_client/
│   │   ├── registry.py
│   │   └── tracing.py
│   ├── mcp_server/
│   │   ├── server.py
│   │   ├── toolsets.py
│   │   └── tools/
│   │       ├── activity.py
│   │       ├── order.py
│   │       ├── refund.py
│   │       └── handoff.py
│   ├── rpc/
│   │   ├── channel.py
│   │   ├── order_client.py
│   │   ├── program_client.py
│   │   ├── user_client.py
│   │   └── generated/
│   ├── session/
│   │   └── store.py
│   ├── config.py
│   ├── graph.py
│   ├── main.py
│   ├── prompts.py
│   ├── router.py
│   └── state.py
├── prompts/
│   ├── activity/system.md
│   ├── coordinator/system.md
│   ├── handoff/system.md
│   ├── order/system.md
│   ├── refund/system.md
│   └── supervisor/system.md
└── tests/
```

## 模块映射

### 从 customer 迁入的核心模块

- `customer/app/graph.py` -> `agents/app/graph.py`
- `customer/app/router.py` -> `agents/app/router.py`
- `customer/app/state.py` -> `agents/app/state.py`
- `customer/app/agents/*` -> `agents/app/agents/*`
- `customer/app/llm/*` -> `agents/app/llm/*`
- `customer/app/mcp_client/*` -> `agents/app/mcp_client/*`
- `customer/prompts/*` -> `agents/prompts/*`

### damai-go 现有能力的归位方式

- 现有 `agents/app/clients/rpc/*` 下沉到 `agents/app/rpc/*`
- 新增 `agents/app/mcp_server/*` 作为真实工具接入层

### 退役模块

- `agents/app/orchestrator/*`
- `agents/app/tools/*`
- `agents/app/session/models.py`

这些模块不应与新实现双轨长期共存，否则会继续造成“接口看起来一样，内部完全不同”的问题。

## 术语与命名收口

`customer` 里存在 demo 时期命名，例如 `event`。迁入 `damai-go` 后必须收口到正式领域术语。

建议统一：

- `selected_event_id` -> `selected_program_id`
- `list_events` / `get_event_detail` 语义收口到 `program`

原则：

- 服务边界以 `damai-go` 的 `program/order/pay/customize/user` 为准
- prompt、tool 名、状态字段、测试文案同步收口
- 不把 demo 命名残留到正式仓库

## 会话状态设计

不再保留第一版 `ConversationSession` 业务模型，改为以 `ConversationState` 为中心存储。

建议核心字段：

- `messages`
- `route`
- `last_intent`
- `coordinator_action`
- `next_agent`
- `business_ready`
- `delegated`
- `selected_program_id`
- `selected_order_id`
- `current_user_id`
- `specialist_result`
- `need_handoff`
- `trace`
- `current_agent`
- `final_reply`

Redis 只保存：

- `conversation_id`
- `user_id`
- graph state

不再保存旧版 `summary/slots/handoff/currentAgent` 等第一版壳字段。

## /agent/chat 请求流

### 请求

- Header: `X-User-Id`
- Body:
  - `message`
  - `conversationId` 可选

### 处理流程

1. FastAPI 读取会话状态；若不存在则创建新会话
2. 将当前用户消息追加到 `messages`
3. 注入 graph runtime context：
   - `current_user_id`
   - `registry`
   - `llm`
4. 调用 graph `ainvoke`
5. 保存新的 graph state 到 Redis
6. 返回标准响应

### 响应

继续保持最小对外契约：

```json
{
  "conversationId": "xxx",
  "reply": "xxx",
  "status": "completed"
}
```

`status` 规则：

- `need_handoff == true` 时返回 `handoff`
- 否则返回 `completed`

不向正式接口暴露 `trace`、`messages`、`current_agent` 等内部调试字段。

## Graph 与多轮会话

`customer` CLI 通过 `thread_id` 使用 LangGraph 内存 checkpoint。迁入服务端后不再将其作为主持久化方案。

服务端策略：

- graph 保持无状态可调用
- Redis 持久化 `ConversationState`
- 多轮上下文由 Redis 里的 state 提供

理由：

- 更适合 API 服务部署
- 可横向扩展
- TTL 与会话归属更可控

## MCP 设计

### MCP 的正式角色

MCP 不是临时过渡层，而是正式工具边界：

- 对上提供稳定 toolset
- 对下适配真实 RPC
- 屏蔽 proto 细节

### toolset 划分

- `activity`
- `order`
- `refund`
- `handoff`

`knowledge` 不进入 MCP。

### MCP server 启动模式

保持与 `customer` 一致的 toolset 启动方式：

- `uv run damai-mcp-server --toolset activity`
- `uv run damai-mcp-server --toolset order`
- `uv run damai-mcp-server --toolset refund`
- `uv run damai-mcp-server --toolset handoff`

这样可直接配合 `MultiServerMCPClient` 的按 toolset 加载方式。

## Tool 命名与 RPC 映射

建议使用正式业务语义命名，而不是照搬 `customer` 的 demo 命名。

### activity toolset

- `list_programs` -> `program-rpc.page_programs`
- `get_program_detail` -> `program-rpc.get_program_detail`

### order toolset

- `list_user_orders` -> `order-rpc.list_orders`
- `get_order_detail_for_service` -> `order-rpc.get_order_service_view`

### refund toolset

- `preview_refund_order` -> `order-rpc.preview_refund_order`
- `submit_refund_order` -> `order-rpc.refund_order`

### handoff toolset

第一阶段先在本地 MCP tool 内完成：

- 生成接管摘要
- 生成接管单号

待未来出现真实人工服务后再替换工具内部实现，但保留 tool 名和返回契约稳定。

## Tool 返回值规范

MCP server 负责把 RPC 结果规整成 agent 友好的业务对象，不把 proto 字段直接暴露给 agent。

推荐对象形状：

### program

- `program_id`
- `title`
- `show_time`
- `venue_name`

### order

- `order_id`
- `status`
- `payment_status`
- `ticket_status`

### refund

- `order_id`
- `eligible`
- `refund_amount`
- `refund_percent`
- `reason`

这样 prompts 和 agent 行为只依赖业务语义，不直接感知底层 proto 命名。

## Knowledge Agent

`KnowledgeAgent` 保持沿用 `customer` 现有方案：

- 继续使用 `LightRAG` HTTP 接口
- 不纳入 MCP
- 不阻塞业务 tool 改造

范围约束：

- 仅支持明星基础百科类问题
- 不支持实时新闻、八卦和最新动态

## 测试策略

建议分五层验证：

### 1. 路由与状态层

覆盖：

- `router.py`
- `CoordinatorAgent`
- `SupervisorAgent`

验证：

- 闲聊
- 活动咨询
- 订单查询
- 退款
- 转人工
- 明星知识问答

### 2. Specialist Agent 层

覆盖：

- `ActivityAgent`
- `OrderAgent`
- `RefundAgent`
- `HandoffAgent`
- `KnowledgeAgent`

其中：

- 业务 specialist 使用 fake registry
- knowledge 使用 fake http client

### 3. MCP server 层

覆盖：

- `mcp_server/tools/*`

验证：

- 入参映射
- RPC 调用
- 返回值归一化
- 异常场景

### 4. API 与会话层

覆盖：

- `/agent/chat`
- `X-User-Id` 校验
- 多轮 `conversationId`
- Redis TTL 刷新
- `handoff` 状态映射

### 5. 端到端合同测试

使用 fake LLM + fake MCP/fake RPC 组合，覆盖至少：

- 活动咨询
- 订单查询
- 无订单号先列单再退款
- 明星知识问答
- 转人工

重点验证多轮 state 连续性，而不是单轮 API 返回。

## 实施顺序

### Step 1. 依赖与目录骨架

- 引入 `jinja2`
- 引入 `langchain-mcp-adapters`
- 引入 `mcp[cli]`
- 引入 `python-dotenv`
- 建立 `llm/mcp_client/mcp_server/rpc` 目录

### Step 2. 迁入 customer 内核

- 迁 `state/router/prompts renderer/llm schemas/agents/graph`
- 同步把 `event` 收口为 `program`
- 先跑不依赖真实 RPC 的单测

### Step 3. 落地真实 MCP server

- 下沉现有 RPC client 到 `app/rpc`
- 实现 `mcp_server/tools/*`
- 打通 fake RPC 与真实配置

### Step 4. 重写 FastAPI + Redis 会话层

- Redis 存 `ConversationState`
- `/agent/chat` 接 graph
- 保持外部响应契约稳定

### Step 5. 重写测试

- 以 `customer` 的 graph/agent/e2e 测试为主体
- 吸收现有 API 契约测试
- 删除仅服务旧 `orchestrator` 的测试

### Step 6. 清理旧实现

- 删除 `app/orchestrator/*`
- 删除 `app/tools/*`
- 删除旧 session model
- 更新 README 与环境变量说明

## 成功标准

本次迁移完成后应满足：

1. `damai-go/agents` 主控制流与 `customer` 一致
2. `activity/order/refund/handoff` 全部通过 MCP 间接访问真实 RPC
3. `knowledge` 能继续独立工作
4. `/agent/chat` 支持 Redis 驱动的多轮会话
5. 测试能证明：
   - 路由正确
   - tool 调用真实存在
   - 列单到退款链路成立
   - 知识问答链路成立
   - API 契约稳定

## 风险与取舍

### 风险 1：customer 术语与 damai-go 领域模型不完全一致

解决方式：

- 在迁入时统一术语
- 由 MCP server 做返回值归一化

### 风险 2：保留旧壳会导致双轨实现长期并存

解决方式：

- 明确退役 `orchestrator/tools/session models`
- 迁移完成后清理旧实现

### 风险 3：知识能力与业务工具改造节奏不同

解决方式：

- 将 `knowledge` 保持独立直连
- 不让其阻塞业务 MCP 改造

## 设计结论

推荐采用“迁 `customer` 内核，但在 `damai-go` 语义上收口”的方案：

- 迁入完整 agent team 控制流
- 对外提供 FastAPI
- 对内通过 MCP 对接真实 RPC
- 用 Redis 驱动多轮状态
- 保留 `KnowledgeAgent` 的 `LightRAG` 直连

这样能真正解决当前 `damai-go/agents` “只是外形一样、实现根本不同”的问题，并为后续接入正式业务服务留下稳定边界。
