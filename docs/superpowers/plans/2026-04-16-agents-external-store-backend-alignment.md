# Agents External Store Backend Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `damai-go` 内的 `agents` 与 `gateway-api` 严格对齐 `/home/chenjiahao/code/project/damai-web/docs/superpowers/specs/2026-04-16-agent-external-store-langgraph-contract-design.md`，把当前旧 `run_* / message_delta / tool_call_*` 契约升级为稳定的后端资源模型、事件日志与 HITL/MCP/LangGraph 长期实现。

**Architecture:** 以 `thread.id == LangGraph thread_id` 为唯一上下文，把一次用户输入收口为一次 `run` 资源，所有执行过程通过 `RunEventStore` 先落库再发布。`agents` 负责资源状态、事件投影、HITL 恢复与 MCP adapter，`gateway-api` 仅做 JWT 鉴权与路由转发；浏览器只消费后端定义的 DTO 与 SSE envelope，不再直接感知 LangGraph chunk 或 MCP 原始协议。

本轮按“最佳重构”落地为两层运行时：

- **外层 LangGraph 工作流**：在 `agents/app/graph.py` 用 LangGraph 编排稳定业务流程、状态流转、HITL 恢复点与工具调用前后收口。
- **内层 Agent 节点**：在工作流的某个节点内使用 LangChain v1 `create_agent(...)` 承载通用 ReAct/tool-calling 能力，而不是把整个系统直接建立在单个 agent executor 之上。
- **弃用旧入口**：不要使用已废弃的 `langgraph.prebuilt.create_react_agent`；新增实现统一以 `create_agent(...)` + LangGraph workflow 为标准。

**Tech Stack:** FastAPI、LangGraph 1.1.6、LangChain v1 `create_agent(...)`、langchain-mcp-adapters、Redis、MySQL、pytest、go-zero gateway、Go integration tests

---

### Task 0: 删除旧兼容层，避免新旧契约共存

**Files:**
- Modify: `agents/app/api/schemas.py`
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/runs/event_models.py`
- Modify: `agents/app/runs/event_projector.py`
- Modify: `agents/app/runs/executor.py`
- Modify: `agents/app/agent_runtime/service.py`
- Modify: `agents/app/agent_runtime/callbacks.py`
- Modify: `agents/app/runs/service.py`
- Modify: `agents/app/runs/repository.py`
- Modify: `agents/app/messages/service.py`
- Modify: `agents/app/threads/service.py`
- Delete: `agents/app/session/store.py`
- Modify: `agents/app/session/__init__.py`
- Modify: `services/gateway-api/etc/gateway-api.yaml`
- Modify: `services/gateway-api/etc/gateway-api.perf.yaml`
- Modify: `services/gateway-api/tests/testkit/gateway.go`
- Modify: `services/gateway-api/tests/integration/agents_run_routes_test.go`
- Modify: `README.md`
- Modify: `agents/README.md`

- [ ] **Step 1: 删除旧 HTTP 契约入口与响应形态**

保留 `POST /agent/threads` 作为显式创建空会话入口，但删除这些旧契约：

- 删除 `/agent/runs/{runId}/stream` 路由、gateway 映射、testkit 映射与文档引用
- 删除 `POST /agent/runs` 的 `messages[]` 请求体
- 删除 `CreateRunResponse(runId, threadId)` 扁平响应
- 删除 `RunDTO.outputMessageIds`

`POST /agent/threads` 只做“创建空线程并返回 `ThreadDTO`”，不创建 message、不创建 run、不触发 LangGraph。

- [ ] **Step 2: 删除旧事件名与旧 SSE envelope**

删除这些旧事件常量与所有引用：

```python
"run_started"
"run_resumed"
"run_paused"
"run_completed"
"run_failed"
"run_cancelled"
"message_delta"
"tool_call_started"
"tool_call_requires_human"
"tool_call_completed"
"tool_call_failed"
```

统一替换为 spec 事件类型：

```python
"run.created"
"run.updated"
"run.completed"
"run.failed"
"run.cancelled"
"message.created"
"message.delta"
"message.updated"
"tool_call.created"
"tool_call.updated"
"tool_call.progress"
"tool_call.completed"
"tool_call.failed"
"run.progress"
```

SSE 只输出：

```text
id: <sequenceNo>
event: agent.run.event
data: <AgentRunEventEnvelope JSON>
```

- [ ] **Step 3: 删除旧 metadata 承载与旧运行结果投影**

删除这些旧承载方式：

- `run.metadata.assistantMessageId`
- `run.metadata.outputMessageIds`
- `tool_call.metadata.request`
- `event.payload.messageId`
- `event.payload.toolCallId`

删除这些旧执行路径：

- 依赖 `final_reply` / `reply` 的一次性最终文本投影
- 依赖 `result.get("tool_call")` 的暂停判断
- 依赖 `result.get("__interrupt__")` 的对外事件判断
- `RunExecutor` 自己根据结果字典决定是否终态

新路径只允许 LangGraph `astream(stream_mode=["messages", "updates", "custom"])` 进入统一 projector。

- [ ] **Step 4: 删除 Redis ThreadOwnershipStore 双事实源**

删除 `ThreadOwnershipStore` 与相关测试，线程归属只通过 MySQL 资源表判断：

- `agent_threads.user_id`
- `agent_runs.user_id`
- `agent_messages.user_id`
- `agent_tool_calls.user_id`

`get_run`、`get_thread`、`list_messages`、`resume`、`cancel` 统一以数据库归属校验为准，避免 Redis TTL 造成会话归属漂移。

- [ ] **Step 5: 删除无目标价值的遗留方法**

删除：

- `RunService.create_running`
- `RunRepository.create_running`
- `HumanToolCallDTO`
- `MessageInputDTO`
- `RunInputMessageDTO`
- 旧错误码 `RUN_STATE_INVALID`
- 旧错误码 `AGENT_RUN_FAILED`

用 spec 错误码替代：

- `ACTIVE_RUN_EXISTS`
- `RUN_NOT_FOUND`
- `THREAD_NOT_FOUND`
- `TOOL_CALL_NOT_FOUND`
- `TOOL_CALL_NOT_WAITING_HUMAN`
- `TOOL_CALL_DECISION_NOT_ALLOWED`
- `LANGGRAPH_RUNTIME_ERROR`
- `UPSTREAM_TOOL_ERROR`
- `RUN_CANCELLED`
- `RUN_NOT_ACTIVE`

- [ ] **Step 6: 跑删除后失败验证**

Run:

```bash
cd agents && uv run pytest \
  tests/test_api.py \
  tests/test_run_contract_api.py \
  tests/test_run_stream_service.py \
  tests/test_run_executor.py \
  tests/test_docs.py -v

cd /home/chenjiahao/code/project/damai-go && go test ./services/gateway-api/tests/integration -run 'Agent' -count=1
```

Expected: FAIL，失败点集中在新 DTO、新事件、新 `/events`、显式线程创建与 LangGraph streaming projector 尚未实现。


### Task 1: 先把后端契约测试改成目标 spec

**Files:**
- Modify: `agents/tests/test_run_contract_api.py`
- Modify: `agents/tests/test_run_resume_cancel_api.py`
- Modify: `agents/tests/test_run_stream_service.py`
- Modify: `agents/tests/test_thread_message_run_services.py`
- Modify: `agents/tests/test_docs.py`
- Modify: `agents/tests/test_e2e_contract.py`
- Modify: `services/gateway-api/tests/integration/agents_run_routes_test.go`
- Modify: `scripts/acceptance/agent_threads.sh`

- [ ] **Step 1: 先写 `POST /agent/runs` 新请求体与新响应体失败用例**

在 `agents/tests/test_run_contract_api.py` 把旧请求：

```json
{
  "messages": [
    { "role": "user", "parts": [{ "type": "text", "text": "帮我查订单" }] }
  ]
}
```

改成：

```json
{
  "threadId": "thr_001",
  "input": {
    "clientMessageId": "cli_msg_001",
    "parts": [{ "type": "text", "text": "帮我查订单" }]
  },
  "metadata": {}
}
```

并断言响应必须包含：

```json
{
  "thread": { "id": "thr_001", "activeRunId": "run_001" },
  "run": {
    "id": "run_001",
    "threadId": "thr_001",
    "assistantMessageId": "msg_asst_001",
    "status": "queued"
  },
  "acceptedMessage": {
    "id": "msg_user_001",
    "runId": "run_001",
    "metadata": { "clientMessageId": "cli_msg_001" }
  },
  "assistantMessage": {
    "id": "msg_asst_001",
    "threadId": "thr_001",
    "runId": "run_001",
    "role": "assistant",
    "status": "in_progress",
    "parts": []
  }
}
```

- [ ] **Step 2: 写 `GET /agent/runs/{runId}/events` 与新 envelope 失败用例**

在 `agents/tests/test_run_contract_api.py` 与 `agents/tests/test_run_stream_service.py` 新增断言：

- 路径从 `/agent/runs/{runId}/stream` 改成 `/agent/runs/{runId}/events`
- SSE `id:` 必须等于事件 `sequenceNo`
- SSE `event:` 固定为 `agent.run.event`
- `GET /agent/runs/{runId}/events?after=12` 只能返回 `sequenceNo > 12` 的事件，不能包含第 12 条
- `data:` 结构必须是：

```json
{
  "schemaVersion": "2026-04-16",
  "sequenceNo": 1,
  "type": "message.delta",
  "runId": "run_001",
  "threadId": "thr_001",
  "messageId": "msg_asst_001",
  "createdAt": "2026-04-16T12:00:03Z",
  "payload": { "delta": "正在帮你查询订单" }
}
```

并断言终态必须出现 `message.updated` + `run.updated(status=...)`，不能只停留在 `message.delta`。

- [ ] **Step 3: 写 `resume/cancel` 与网关映射失败用例**

在 `agents/tests/test_run_resume_cancel_api.py` 与 `services/gateway-api/tests/integration/agents_run_routes_test.go` 新增断言：

- `POST /agent/runs/{runId}/tool-calls/{toolCallId}/resume`
  - 同一 `runId`
  - 同一 `thread_id`
  - 第二次相同恢复请求按幂等处理
- `POST /agent/runs/{runId}/cancel`
  - 只能取消 `queued|running|requires_action`
  - 对 `completed|failed|cancelled` 返回 `409`
  - `error.code == "RUN_NOT_ACTIVE"`
- `GET /agent/runs/{runId}/events?after=12`
  - 网关要保留 query string 原样转发
  - 不再暴露 `/stream`
- `PATCH /agent/threads/{threadId}`
  - 网关必须按原路径转发到 agents
  - 只允许更新 `title` 与 `status`

同时把 `scripts/acceptance/agent_threads.sh` 改成：

```bash
thread_id="$(gateway_curl POST /agent/threads '{"title":"订单咨询"}' | jq -r '.thread.id')"

gateway_curl POST /agent/runs '{
  "threadId":"'"${thread_id}"'",
  "input":{"clientMessageId":"cli_001","parts":[{"type":"text","text":"'"$(agent_case_order)"'"}]},
  "metadata":{}
}'
```

- [ ] **Step 4: 跑定向测试，确认当前实现按新 spec 失败**

Run:

```bash
cd agents && uv run pytest \
  tests/test_run_contract_api.py \
  tests/test_run_stream_service.py \
  tests/test_run_resume_cancel_api.py \
  tests/test_e2e_contract.py -v

cd /home/chenjiahao/code/project/damai-go && go test ./services/gateway-api/tests/integration -run 'Agent' -count=1
```

Expected: FAIL，报旧请求体 `messages`、旧 `/stream` 路由、旧 `run_started/message_delta/run_completed` 事件名与目标 spec 不一致。


### Task 2: 对齐 SQL 与持久化资源模型

**Files:**
- Modify: `sql/agents/agent_messages.sql`
- Modify: `sql/agents/agent_runs.sql`
- Modify: `sql/agents/agent_tool_calls.sql`
- Modify: `sql/agents/agent_run_events.sql`
- Modify: `scripts/import_sql.sh`
- Modify: `agents/app/messages/models.py`
- Modify: `agents/app/runs/models.py`
- Modify: `agents/app/runs/tool_call_models.py`
- Modify: `agents/app/runs/event_models.py`
- Modify: `agents/app/runs/tool_call_repository.py`
- Modify: `agents/app/runs/event_store.py`
- Modify: `agents/app/runs/repository.py`

- [ ] **Step 1: 先写模型/仓储失败用例，锁定新增字段**

在 `agents/tests/test_thread_message_run_repositories.py` 与 `agents/tests/test_thread_message_run_services.py` 新增断言：

- `agent_messages` 有 `updated_at`
- `agent_runs` 显式保存 `assistant_message_id`
- `agent_tool_calls` 显式保存 `message_id`、`request_json`
- `RunEventRecord` 必须能承载 `message_id`、`tool_call_id`
- `RunRecord` 必须暴露 `assistant_message_id`

同时补 4 条“旧承载方式必须退出”的失败断言：

- `assistantMessageId` 不再只存在于 `run.metadata_json`
- `tool_call.request` 不再通过 `metadata_json.request` 持久化
- `messageId` / `toolCallId` 不再只存在于 `agent_run_events.payload_json`
- `scripts/import_sql.sh` 导入 `agents` 域时必须包含 `agent_run_events.sql` 与 `agent_tool_calls.sql`

- [ ] **Step 2: 修改 SQL，把资源字段从 metadata 抽成显式列**

目标表结构至少补齐：

```sql
ALTER TABLE agent_messages
  ADD COLUMN updated_at datetime(3) NOT NULL AFTER created_at;

ALTER TABLE agent_runs
  ADD COLUMN assistant_message_id varchar(64) NOT NULL AFTER trigger_message_id;

ALTER TABLE agent_tool_calls
  ADD COLUMN message_id varchar(64) NOT NULL AFTER run_id,
  ADD COLUMN request_json json NOT NULL AFTER arguments_json;
```

`agent_run_events` 继续 append-only，但要为 envelope 归属信息留出显式列：

```sql
ALTER TABLE agent_run_events
  ADD COLUMN message_id varchar(64) NULL AFTER thread_id,
  ADD COLUMN tool_call_id varchar(64) NULL AFTER message_id;
```

不要新增线程表里的 `active_run_id` 持久化列；继续通过 active run 查询推导，避免双写漂移。

并明确 3 个清理动作：

- 不删除 `sql/agents/*.sql` 这 5 个表文件本身；删除的是“旧字段承载语义”
- `agent_runs` 里后续不再依赖 `metadata_json.assistantMessageId`
- `agent_tool_calls` 里后续不再依赖 `metadata_json.request`

- [ ] **Step 3: 更新 Python 记录模型与状态常量**

把 `agents/app/messages/models.py`、`agents/app/runs/models.py`、`agents/app/runs/tool_call_models.py`、`agents/app/runs/event_models.py` 调整成显式资源模型，例如：

```python
@dataclass(slots=True)
class RunRecord:
    id: str
    thread_id: str
    user_id: int
    trigger_message_id: str
    assistant_message_id: str
    status: str
    started_at: datetime
    completed_at: datetime | None = None
    error: dict[str, Any] | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
```

```python
@dataclass(slots=True)
class RunEventRecord:
    id: str
    run_id: str
    thread_id: str
    user_id: int
    sequence_no: int
    event_type: str
    message_id: str | None = None
    tool_call_id: str | None = None
    payload: dict[str, Any] = field(default_factory=dict)
    created_at: datetime | None = None
```

同时把仓储持久化方式改成显式列，不再走旧兜底逻辑：

- `agents/app/runs/repository.py`：`assistant_message_id` 直接入库/出库
- `agents/app/runs/tool_call_repository.py`：`request_json`、`message_id` 直接入库/出库，不再把 `request` 塞进 `metadata_json`
- `agents/app/runs/event_store.py`：`message_id`、`tool_call_id` 直接入库/出库，不再只放 `payload_json`

- [ ] **Step 4: 跑仓储定向测试**

Run:

```bash
cd agents && uv run pytest \
  tests/test_thread_message_run_repositories.py \
  tests/test_thread_message_run_services.py -v
```

Expected: PASS，仓储层已经能持久化新的显式资源字段。

- [ ] **Step 5: 更新 SQL 导入清单并清理旧初始化遗漏**

在 `scripts/import_sql.sh` 把：

```bash
AGENTS_SQL_FILES=(
  "sql/agents/agent_threads.sql"
  "sql/agents/agent_messages.sql"
  "sql/agents/agent_runs.sql"
)
```

改成：

```bash
AGENTS_SQL_FILES=(
  "sql/agents/agent_threads.sql"
  "sql/agents/agent_messages.sql"
  "sql/agents/agent_runs.sql"
  "sql/agents/agent_run_events.sql"
  "sql/agents/agent_tool_calls.sql"
)
```

并确认没有额外旧 `agents` SQL 文件需要继续保留兼容导入。

- [ ] **Step 6: 跑 agents SQL 初始化检查**

Run:

```bash
cd /home/chenjiahao/code/project/damai-go && IMPORT_DOMAINS=agents bash scripts/import_sql.sh
```

Expected: `damai_agents` 初始化时包含 `agent_threads`、`agent_messages`、`agent_runs`、`agent_run_events`、`agent_tool_calls` 五张表，不再遗漏后两张。


### Task 3: 重写 API DTO 与 HTTP 路由契约

**Files:**
- Modify: `agents/app/api/schemas.py`
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/common/errors.py`
- Modify: `agents/README.md`
- Modify: `README.md`
- Modify: `agents/tests/test_api.py`
- Modify: `agents/tests/test_run_contract_api.py`
- Modify: `agents/tests/test_docs.py`

- [ ] **Step 1: 先写 DTO 失败用例，锁定新字段名**

断言以下 DTO 必须存在：

```python
class CreateRunRequest(ApiSchemaModel):
    thread_id: str = Field(alias="threadId")
    input: RunInputDTO
    metadata: dict[str, Any] = Field(default_factory=dict)

class RunInputDTO(ApiSchemaModel):
    client_message_id: str | None = Field(default=None, alias="clientMessageId")
    parts: list[TextPartDTO] = Field(min_length=1)
```

`CreateRunResponse` 改成：

```python
class CreateRunResponse(ApiSchemaModel):
    thread: ThreadDTO
    run: RunDTO
    accepted_message: MessageDTO = Field(alias="acceptedMessage")
    assistant_message: MessageDTO = Field(alias="assistantMessage")
```

并给 `RunDTO` 增加：

```python
assistant_message_id: str = Field(alias="assistantMessageId")
```

同时补齐查询/线程侧 DTO：

```python
class CreateThreadRequest(ApiSchemaModel):
    title: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)

class CreateThreadResponse(ApiSchemaModel):
    thread: ThreadDTO

class GetRunResponse(ApiSchemaModel):
    run: RunDTO

class ListThreadsResponse(ApiSchemaModel):
    threads: list[ThreadDTO]
    next_cursor: str | None = Field(default=None, alias="nextCursor")

class GetThreadResponse(ApiSchemaModel):
    thread: ThreadDTO

class ListThreadMessagesResponse(ApiSchemaModel):
    messages: list[MessageDTO]
    next_cursor: str | None = Field(default=None, alias="nextCursor")

class UpdateThreadRequest(ApiSchemaModel):
    title: str | None = None
    status: Literal["active", "archived"] | None = None

class UpdateThreadResponse(ApiSchemaModel):
    thread: ThreadDTO
```

并补齐 HITL resume 与人审请求 DTO：

```python
ResumeToolCallAction = Literal["approve", "reject", "edit"]

class ResumeToolCallRequest(ApiSchemaModel):
    action: ResumeToolCallAction
    reason: str | None = None
    values: dict[str, Any] = Field(default_factory=dict)

class HumanRequestDTO(ApiSchemaModel):
    kind: Literal["approval", "input"]
    title: str
    description: str | None = None
    allowed_actions: list[ResumeToolCallAction] = Field(alias="allowedActions")
```

要求：

- `action` 的全集只能是 `approve|reject|edit`
- 单个 `toolCallId` 实际允许的动作以后端持久化的 `humanRequest.allowedActions` 为准
- `action` 不在 `allowedActions` 中时返回 `TOOL_CALL_DECISION_NOT_ALLOWED`
- `edit` 必须校验 `values` 能映射为当前工具调用参数，再转换成 LangGraph `edited_action`

- [ ] **Step 2: 把 `POST /agent/runs` 从“旧 chat 风格”改成“单次执行资源”**

在 `agents/app/api/routes.py`：

- 不再接受 `messages[]`
- 不再允许 `threadId` 缺失；线程必须先通过 `POST /agent/threads` 创建
- 校验 `threadId` 必须存在且属于当前用户
- 写入用户消息
- 预创建 assistant message
- 创建 run
- 返回 `thread + run + acceptedMessage + assistantMessage`

响应组装形态按 spec：

```python
return CreateRunResponse(
    thread=to_thread_dto(thread),
    run=to_run_dto(run),
    acceptedMessage=to_message_dto(user_message),
    assistantMessage=to_message_dto(assistant_message),
)
```

- [ ] **Step 3: 补齐 `GET /agent/runs/{runId}` 与线程资源接口**

在 `agents/app/api/routes.py` 增加或收敛以下接口，并补齐 README 示例：

- `POST /agent/threads`
- `GET /agent/runs/{runId}`
- `GET /agent/threads?cursor=<cursor>&limit=20`
- `GET /agent/threads/{threadId}`
- `GET /agent/threads/{threadId}/messages?before=<cursor>&limit=50`
- `PATCH /agent/threads/{threadId}`

要求：

- `POST /agent/threads` 只创建空线程，返回 `ThreadDTO`，`lastMessageAt=null`，`activeRunId=null`
- `POST /agent/runs` 不隐式创建线程，缺少或不存在 `threadId` 时返回业务错误
- `ThreadDTO.activeRunId` 通过当前 active run 推导，不落线程表
- `GET /agent/threads/{threadId}/messages` 返回稳定倒序分页结果与 `nextCursor`
- `PATCH /agent/threads/{threadId}` 只允许更新 `title` 与 `status`
- `GET /agent/runs/{runId}`、`GET /agent/threads/{threadId}` 都要做归属校验

- [ ] **Step 4: 新增 `/agent/runs/{runId}/events`，删除 `/stream` 对外契约**

在 `agents/app/api/routes.py`：

- 把旧 `stream_run()` 改成 `list_run_events()`
- 路由改为 `@router.get("/agent/runs/{run_id}/events")`
- `StreamingResponse` 继续保留，但输出 SSE envelope
- SSE 行格式必须包含 `id: <sequenceNo>`
- `after` 的查询语义必须严格实现为 `sequenceNo > after`
- `README.md` 与 `agents/README.md` 同步改成 `/events`

同时把错误码统一到 spec 稳定英文常量：

- `ACTIVE_RUN_EXISTS`
- `RUN_NOT_FOUND`
- `THREAD_NOT_FOUND`
- `TOOL_CALL_NOT_FOUND`
- `TOOL_CALL_NOT_WAITING_HUMAN`
- `TOOL_CALL_DECISION_NOT_ALLOWED`
- `LANGGRAPH_RUNTIME_ERROR`
- `UPSTREAM_TOOL_ERROR`
- `RUN_CANCELLED`
- `RUN_NOT_ACTIVE`

- [ ] **Step 5: 跑 API 与文档测试**

Run:

```bash
cd agents && uv run pytest \
  tests/test_api.py \
  tests/test_run_contract_api.py \
  tests/test_docs.py -v
```

Expected: PASS，HTTP 路由与 README 已切到目标资源契约。


### Task 4: 收口 Run/Thread/Message 服务与并发规则

**Files:**
- Modify: `agents/app/runs/service.py`
- Modify: `agents/app/runs/repository.py`
- Modify: `agents/app/messages/service.py`
- Modify: `agents/app/messages/repository.py`
- Modify: `agents/app/threads/service.py`
- Modify: `agents/app/threads/repository.py`
- Modify: `agents/tests/test_thread_message_run_services.py`
- Modify: `agents/tests/test_run_resume_cancel_api.py`

- [ ] **Step 1: 先写 active run 并发失败用例**

新增断言：

- 同一 `threadId` 同时只能有一个 `queued|running|requires_action`
- 第二次 `POST /agent/runs` 返回 `409`
- `error.code == "ACTIVE_RUN_EXISTS"`
- `thread.activeRunId` 来自 `RunRepository.find_active_by_thread()`
- run 进入 `completed|failed|cancelled` 后，`thread.activeRunId` 必须变回 `null`

- [ ] **Step 2: 把 run 创建收敛成单事务**

在 `RunService` / `RunRepository` 中新增一个原子入口，例如：

```python
def create_run_with_messages(
    self,
    *,
    user_id: int,
    thread_id: str,
    client_message_id: str | None,
    parts: list[dict[str, Any]],
) -> tuple[RunRecord, MessageRecord, MessageRecord]:
    ...
```

要求在一个事务里完成：

1. 校验线程归属
2. 锁定线程或 active run 查询
3. 写 user message
4. 预写 assistant message
5. 写 run（包含 `assistant_message_id`）
6. 更新线程 `last_message_at`

- [ ] **Step 3: 补齐线程查询、消息分页与线程更新服务**

在 `ThreadService` / `MessageService` 中补齐：

- `list_threads(user_id, cursor, limit)`
- `get_thread(user_id, thread_id)`
- `list_thread_messages(user_id, thread_id, before, limit)`
- `update_thread(user_id, thread_id, title, status)`

并明确：

- `activeRunId` 必须通过 `RunRepository.find_active_by_thread()` 派生
- 新线程首次创建时要有可接受的默认标题策略，后续允许 `PATCH` 覆盖
- `list_thread_messages` 只返回当前用户可见消息，不从 LangGraph 原始历史读取
- 线程归属校验失败统一返回 `THREAD_NOT_FOUND`

- [ ] **Step 4: 让 `thread.id` 永远等于 LangGraph `thread_id`**

不要再生成额外运行态上下文 id。`AgentRuntimeService`、`RunService`、`resume` 都必须直接复用同一个 `thread.id`：

```python
config = {"configurable": {"thread_id": thread.id}}
```

后端只允许在 `POST /agent/threads` 创建线程；`POST /agent/runs` 只能复用已存在的 `thread.id`。后续 LangGraph checkpoint、恢复、回放都围绕这一个 id。

- [ ] **Step 5: 跑服务层测试**

Run:

```bash
cd agents && uv run pytest \
  tests/test_thread_message_run_services.py \
  tests/test_run_resume_cancel_api.py -v
```

Expected: PASS，单线程单 active run 规则稳定，且 `thread.id == LangGraph thread_id`。


### Task 5: 重做 RunEventStore、Projector 与最终快照语义

**Files:**
- Modify: `agents/app/runs/event_models.py`
- Modify: `agents/app/runs/event_store.py`
- Modify: `agents/app/runs/event_projector.py`
- Modify: `agents/app/runs/stream_service.py`
- Modify: `agents/app/runs/event_bus.py`
- Modify: `agents/tests/test_run_stream_service.py`
- Modify: `agents/tests/test_run_executor.py`

- [ ] **Step 1: 先写 envelope 与终态快照失败用例**

新增断言：

- 所有事件都有 `runId`、`threadId`
- 消息事件都有 `messageId`
- 工具事件都有 `messageId + toolCallId`
- 新 run 创建后必须实际写入并输出 `run.created`，不能只依赖 `POST /agent/runs` 响应里的 `run`
- 工具调用开始时必须实际写入并输出 `tool_call.created`，不能只输出后续 `tool_call.updated`
- 同一 assistant message 上，`message.created` 必须先于任何 `tool_call.created`、`tool_call.updated`、`tool_call.progress`、`tool_call.completed`、`tool_call.failed`
- 若后端尚未确定或持久化 `messageId`，不得发送任何 `tool_call.*` 事件
- `SSE id == sequenceNo`
- `after` 只回放严格大于游标的事件
- 默认 SSE envelope 不包含 `debug`
- `?debug=true` 仅在开发/测试允许时附加 `debug`，且不得泄露 checkpoint 或原始 LangGraph chunk
- 连接建立时必须先回放历史事件再进入实时 tail；回放与订阅切换期间新落库事件不能丢、不能重复
- 成功终态顺序至少包含：
  - `run.created`
  - `message.created`
  - `run.updated(status=running)`
  - `message.delta`
  - `message.updated(status=completed)`
  - `run.updated(status=completed)`

- [ ] **Step 2: 把事件模型改成 spec 的业务 envelope**

把旧事件类型：

```python
"run_started"
"run_completed"
"tool_call_requires_human"
```

替换成：

```python
"run.created"
"run.updated"
"run.completed"
"run.failed"
"run.cancelled"
"message.created"
"message.delta"
"message.updated"
"tool_call.created"
"tool_call.updated"
"tool_call.progress"
"tool_call.completed"
"tool_call.failed"
"run.progress"
```

SSE `data:` 必须由统一序列化器输出：

```python
{
    "schemaVersion": "2026-04-16",
    "sequenceNo": event.sequence_no,
    "type": event.event_type,
    "runId": event.run_id,
    "threadId": event.thread_id,
    "messageId": event.message_id,
    "toolCallId": event.tool_call_id,
    "createdAt": event.created_at.isoformat().replace("+00:00", "Z"),
    "payload": event.payload,
}
```

并约束：

- `debug` 默认不输出，只在开发模式或 `?debug=true` 时附加
- 本轮默认只实现 `?after=` 游标；`Last-Event-ID` 仅作为后续增强，不做半套兼容
- `message.delta` 只允许落到 assistant message

- [ ] **Step 3: 强制“先落库，再 publish，再 tail”**

在 `RunEventProjector` 与 `RunExecutor` 中统一走：

```python
record = self.event_store.append(...)
self.event_bus.publish(run_id=record.run_id, sequence_no=record.sequence_no)
```

不要再先发内存事件再补写数据库。

`finalize_run()` 和 `fail_run()` 必须落最终消息快照：

```python
await callbacks.on_message_updated(..., status="completed")
await callbacks.on_run_updated(..., status="completed")
```

失败与取消同理，且 `message.status` 不能再误写成 `completed`。

同时补齐：

- 取消时也必须输出 `message.updated(status=cancelled)` + `run.updated(status=cancelled)` 或 `run.cancelled`
- 若取消发生在 `requires_action`，当前 waiting tool call 必须收敛到 `cancelled` 或等价可解释终态，并通过 `tool_call.updated` 或最终 `message.updated` 快照体现
- run 终态落库后必须让线程查询侧看到 `activeRunId = null`

- [ ] **Step 4: 跑事件与执行器测试**

Run:

```bash
cd agents && uv run pytest \
  tests/test_run_stream_service.py \
  tests/test_run_executor.py -v
```

Expected: PASS，事件回放、尾随订阅、终态快照与 sequenceNo 递增都符合 spec。


### Task 6: 用 LangGraph streaming projector 替换一次性结果投影

**Files:**
- Modify: `agents/app/agent_runtime/callbacks.py`
- Modify: `agents/app/agent_runtime/service.py`
- Modify: `agents/app/runs/executor.py`
- Modify: `agents/app/runs/resume_command_executor.py`
- Modify: `agents/app/graph.py`
- Modify: `agents/tests/test_run_executor.py`
- Modify: `agents/tests/test_hitl_bridge.py`
- Modify: `agents/tests/test_run_resume_cancel_api.py`

- [ ] **Step 1: 先写 `astream()` 投影失败用例**

在 `agents/tests/test_run_executor.py` 新增 fake runtime，模拟按顺序吐出：

```python
[
  ("updates", {"run": {"status": "running"}}),
  ("messages", {"delta": "正在查询订单"}),
  ("custom", {"tool": {"name": "preview_refund_order", "status": "completed"}}),
]
```

并断言 executor/projector 不再依赖：

```python
result.get("final_reply")
result.get("tool_call")
```

同时明确新的运行时装配方式：

- `agents/app/graph.py` 负责组装外层 LangGraph workflow，而不是只暴露一个扁平 agent runtime
- workflow 中至少保留一个“agent node”，该节点内部通过 LangChain v1 `create_agent(...)` 创建 agent
- 不允许继续使用 `langgraph.prebuilt.create_react_agent`

同时补 4 条失败断言，锁死 HITL 的执行边界：

- 只允许 `Command(resume=...)` 恢复，不允许第二套私有 pause/resume 原语
- `approve|reject|edit` 必须映射成 LangGraph `Command(resume={"decisions":[...]})`，不能把前端请求体原样透传给 LangGraph
- 对不在当前 `humanRequest.allowedActions` 中的 `action`，必须返回 `TOOL_CALL_DECISION_NOT_ALLOWED`
- `edit` 必须生成包含工具名和修改后参数的 `edited_action`
- 前端事件里不允许出现 `__interrupt__`、checkpoint、graph node、LangGraph 原始 chunk
- `thread.id == LangGraph thread_id`
- 命中人工确认后，状态判断以后端资源为准，而不是前端临时推断

- [ ] **Step 2: 把 runtime service 改成流式投影**

在 `agents/app/graph.py` 与 `agents/app/agent_runtime/service.py` 中按以下方式重构：

- `graph.py`：定义外层 LangGraph workflow，负责业务流程编排、状态节点、resume/cancel 收口与 checkpoint 对接
- `agent_runtime/service.py`：只负责驱动 workflow 执行与 chunk 投影，不再自己扮演完整业务编排器
- workflow 中的 agent 节点统一使用 `create_agent(...)` 构建，并注入：
  - `HumanInTheLoopMiddleware(interrupt_on=...)`
  - 当前工具集合
  - 持久化 checkpointer
- `refund_order`、`cancel_order`、`query_order` 等工具的 HITL 策略由 `interrupt_on` 声明：
  - `refund_order` -> `approve|reject|edit`
  - `cancel_order` -> `approve|reject`
  - `query_order` -> `False`

流式执行统一用：

```python
async for mode, chunk in self.agent_runtime.astream(
    payload,
    config={"configurable": {"thread_id": thread_id}},
    stream_mode=["messages", "updates", "custom"],
):
    await self._project_chunk(run=run, callbacks=callbacks, mode=mode, chunk=chunk)
```

规则：

- `messages` -> `message.delta`
- `updates` -> `run.updated` / `message.updated`
- `custom` -> `tool_call.progress`
- 长耗时但未绑定工具的自定义进度 -> `run.progress`
- interrupt -> `tool_call.updated(status=waiting_human)` + `run.updated(status=requires_action)`

实现时强制执行 3 条约束：

- `InterruptBridge` 是唯一允许做 `interrupt payload -> tool call DTO` 翻译的地方
- `agent`、`executor`、`api route` 里禁止各自拼装 HITL payload
- 外部只输出后端事件 envelope，不输出 LangGraph 内部结构
- tool-calling 推理放在 `create_agent(...)` 节点内，业务流程编排放在外层 LangGraph workflow，二者职责不能混写

- [ ] **Step 3: 让 resume 继续同一个 run，同一个 thread**

`RunExecutor.resume()` 必须使用：

```python
Command(resume=resume_payload)
```

并保持：

- 不新建 run
- 不新建 thread
- `run.id` 不变
- `thread_id` 不变

恢复成功后只更新当前 `tool_call`，然后继续流式投影后续消息与终态。

实现完成后必须额外验证：

- `RunExecutor.resume()` 只做归属校验、状态推进、事件写入、调用恢复执行器
- `RunExecutor.resume()` 不再按工具名直执行业务逻辑
- 恢复时仍使用同一个 `thread_id`，且不新建 run
- `approve` 转为 `{"type":"approve"}`
- `reject` 转为 `{"type":"reject","message": reason}`，其中 `reason` 可为空但字段语义稳定
- `edit` 转为 `{"type":"edit","edited_action":{"name": toolName,"args": values}}`

- [ ] **Step 4: 加上取消执行链路与终态闭环**

在 `RunExecutor` / `resume_command_executor.py` 中补齐取消机制：

- `cancel(run_id)` 只允许作用于 `queued|running|requires_action`
- 取消要能中止后续 LangGraph 流投影或阻止继续 publish 新事件
- 取消后必须统一走终态写入：更新 run、assistant message、thread activeRunId
- 若取消发生在 `requires_action`，当前 waiting tool call 也要收敛到可解释终态，避免前端残留待处理卡片

- [ ] **Step 5: 跑 runtime / HITL 测试**

Run:

```bash
cd agents && uv run pytest \
  tests/test_run_executor.py \
  tests/test_hitl_bridge.py \
  tests/test_run_resume_cancel_api.py -v
```

Expected: PASS，执行链路已经从“结果字典驱动”切换为“LangGraph 流驱动投影”。


### Task 7: 补 MCP adapter / interceptor 与副作用幂等

**Files:**
- Modify: `agents/app/mcp_client/registry.py`
- Create: `agents/app/mcp_client/execution_context.py`
- Create: `agents/app/mcp_client/interceptor.py`
- Modify: `agents/app/agent_runtime/service.py`
- Modify: `agents/app/runs/tool_call_repository.py`
- Modify: `agents/app/runs/tool_call_models.py`
- Modify: `agents/tests/test_mcp_registry.py`
- Create: `agents/tests/test_mcp_interceptor.py`

- [ ] **Step 1: 先写 MCP 调用上下文失败用例**

新增断言：每次工具调用都必须带：

```python
{
    "userId": "3001",
    "threadId": "thr_001",
    "runId": "run_001",
    "toolCallId": "tool_001",
}
```

并且 `registry.invoke()` 不能直接把原始 payload 扔给 MCP；必须先经由后端 interceptor 注入上下文、统一超时、统一错误格式。

同时补 3 条与 HITL 直接相关的失败断言：

- 同一 run 任一时刻最多只有一个 `waiting_human` 的 tool call
- 每个 waiting tool call 必须绑定 `toolCallId + messageId + runId + threadId`
- 每个 waiting tool call 的 `humanRequest.allowedActions` 必须来自当前工具的 `allowed_decisions`
- 刷新页面后可以只靠 `run + tool_calls + events(after=)` 重建待处理 HITL
- `toolCall` DTO 必须稳定暴露 `humanRequest`、`result`、`error`
- MCP 超时、不可用、坏响应、工具不存在、执行失败必须分别归一化为稳定错误码：
  - `MCP_TIMEOUT`
  - `MCP_UNAVAILABLE`
  - `MCP_BAD_RESPONSE`
  - `MCP_TOOL_NOT_FOUND`
  - `MCP_EXECUTION_ERROR`
- `tool_call.failed.payload.error.code` 与持久化的 `tool_call.error.code` 必须一致

- [ ] **Step 2: 新增 MCP 执行上下文与拦截器**

创建：

```python
@dataclass(slots=True)
class ToolExecutionContext:
    user_id: str
    thread_id: str
    run_id: str
    tool_call_id: str
    channel_code: str | None = None
    request_id: str | None = None
```

以及：

```python
class MCPToolInterceptor:
    async def invoke(self, *, server_name: str, tool_name: str, payload: dict[str, Any], context: ToolExecutionContext) -> Any:
        ...
```

`registry.py` 只负责发现工具，不再承担最终执行策略。

同时把 `tool_call` 资源模型补齐为稳定字段，而不是临时 metadata 拼装：

- `status`
- `human_request`
- `result`
- `error`
- `request_json`
- `message_id`

- [ ] **Step 3: 用 `toolCallId` 做副作用幂等键**

对 `refund_order` 这类副作用工具，至少把 `tool_call_id` 注入 payload：

```python
payload["_meta"] = {
    "toolCallId": context.tool_call_id,
    "runId": context.run_id,
    "threadId": context.thread_id,
}
```

后续如 Go MCP server 需要透传幂等键，也直接复用这个值，不再额外生成新 id。

并把副作用时机明确写进实现：

- `refund_order`、支付、发券、创建订单等副作用工具只能出现在 `resume` 之后
- `interrupt()` 之前只允许读取型或幂等安全动作
- 同一 `runId + toolCallId` 的副作用 MCP 工具重复 `resume` / retry 只能实际调用下游一次
- 重复 `resume` 命中既有结果时，应返回当前 run 状态并复用已持久化的 `tool_call.result/error` 与事件，不重新执行副作用

- [ ] **Step 4: 补工具调用结果、错误与人审请求的持久化映射**

在 `tool_call_repository.py`、projector 与 DTO 转换中明确：

- `humanRequest` 必须从稳定字段投影，不从 LangGraph 原始 chunk 临时拼接
- 工具成功后把标准化结果写入 `result`
- 工具失败后把统一错误 DTO 写入 `error`
- `message.updated` 与 `tool_call.updated/completed/failed` 对同一 `toolCallId` 的投影必须一致

- [ ] **Step 5: 跑 MCP 定向测试**

Run:

```bash
cd agents && uv run pytest \
  tests/test_mcp_registry.py \
  tests/test_mcp_interceptor.py -v
```

Expected: PASS，MCP 已经退化成纯工具执行层，运行态上下文与错误归一化由后端接管。


### Task 8: 更新 gateway、文档与联调脚本，完成整体验证

**Files:**
- Modify: `services/gateway-api/etc/gateway-api.yaml`
- Modify: `services/gateway-api/etc/gateway-api.perf.yaml`
- Modify: `services/gateway-api/tests/testkit/gateway.go`
- Modify: `services/gateway-api/tests/integration/agents_run_routes_test.go`
- Modify: `README.md`
- Modify: `agents/README.md`
- Modify: `scripts/acceptance/agent_threads.sh`
- Modify: `agents/tests/test_e2e_contract.py`

- [ ] **Step 1: 更新 gateway route 映射**

把：

```yaml
- Method: GET
  Path: /agent/runs/:runId/stream
```

替换成：

```yaml
- Method: GET
  Path: /agent/runs/:runId/events
```

其余资源集合维持：

- `POST /agent/threads`
- `POST /agent/runs`
- `GET /agent/threads`
- `GET /agent/threads/{threadId}`
- `GET /agent/threads/{threadId}/messages`
- `PATCH /agent/threads/{threadId}`
- `GET /agent/runs/{runId}`
- `POST /agent/runs/{runId}/tool-calls/{toolCallId}/resume`
- `POST /agent/runs/{runId}/cancel`

- [ ] **Step 2: 更新 README 与 acceptance 文案**

把所有 `/stream`、`messages[]`、`run_started` 等旧名词替换成：

- `/events?after=`
- `input.parts`
- `message.updated`
- `run.updated`
- `tool_call.updated(waiting_human)`

并把验收脚本 / 文档样例至少覆盖这 5 类场景：

- 正常完成
- 运行失败
- HITL 等待 + resume
- 运行取消
- 断线后 `after=` 重连回放

- [ ] **Step 3: 跑最终定向验证**

Run:

```bash
cd /home/chenjiahao/code/project/damai-go && go test ./services/gateway-api/... -count=1

cd /home/chenjiahao/code/project/damai-go/agents && uv run pytest \
  tests/test_api.py \
  tests/test_run_contract_api.py \
  tests/test_run_stream_service.py \
  tests/test_run_executor.py \
  tests/test_run_resume_cancel_api.py \
  tests/test_thread_message_run_repositories.py \
  tests/test_thread_message_run_services.py \
  tests/test_e2e_contract.py \
  tests/test_docs.py -v

cd /home/chenjiahao/code/project/damai-go && JWT=<user-jwt> bash scripts/acceptance/agent_threads.sh
```

Expected: PASS，网关转发、agents 资源契约、事件回放、HITL 恢复与联调脚本全部与目标 spec 一致。

- [ ] **Step 4: 提交计划涉及的文档与配置改动**

```bash
git add \
  docs/superpowers/plans/2026-04-16-agents-external-store-backend-alignment.md \
  README.md \
  agents/README.md \
  scripts/acceptance/agent_threads.sh \
  services/gateway-api/etc/gateway-api.yaml \
  services/gateway-api/etc/gateway-api.perf.yaml \
  services/gateway-api/tests/testkit/gateway.go \
  services/gateway-api/tests/integration/agents_run_routes_test.go

git commit -m "docs: add agents external store backend alignment plan"
```
