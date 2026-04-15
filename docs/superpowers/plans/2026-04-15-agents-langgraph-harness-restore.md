# agents LangGraph Harness 恢复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `agents` 主链路从 `ParentAgent -> TaskCard -> SubagentRuntime` 切回 `LangGraph`，恢复 `coordinator + supervisor + specialist + graph` 架构，同时保留当前 `go-zero MCP server + Python MCP client` 的能力接入形态与 `/agent/chat` 对外 contract。

**Architecture:** 本次恢复分四层推进。第一层先恢复 LangGraph 所需的 prompt 渲染、结构化 LLM schema、共享状态模型与 Redis checkpointer，让 graph 可以作为唯一主编排和主业务状态源。第二层恢复 `coordinator`、`supervisor` 与 graph 骨架，再把 `order`、`refund`、`activity`、`handoff`、`knowledge` 五个 specialist 逐个接回，但底层工具访问继续复用当前 `MCPToolRegistry`。第三层把 `/agent/chat` 切到 graph 主链路，同时保留一个极薄的 ownership store 做 `conversation_id -> user_id` 归属校验。最后一层补齐 graph、API、checkpoint、主流程测试，并更新 README/docs，让旧 `ParentAgent` 代码退出热路径但暂不删除。

**Tech Stack:** Python, FastAPI, LangGraph, Pydantic, Redis, Jinja2, MCP, pytest

---

### Task 1: 恢复 prompt 渲染、结构化 schema 与 prompt 模板

**Files:**
- Create: `agents/app/prompts.py`
- Create: `agents/app/llm/schemas.py`
- Create: `agents/prompts/coordinator/system.md`
- Create: `agents/prompts/supervisor/system.md`
- Create: `agents/prompts/order/system.md`
- Create: `agents/prompts/refund/system.md`
- Create: `agents/prompts/activity/system.md`
- Create: `agents/prompts/handoff/system.md`
- Create: `agents/prompts/knowledge/system.md`
- Create: `agents/tests/test_prompts.py`
- Modify: `agents/tests/fakes.py`

- [ ] **Step 1: 先写 prompt 与 structured output 的失败测试**

在 `agents/tests/test_prompts.py` 新增最小断言，确认 renderer 能加载 `coordinator/system.md` 与 `supervisor/system.md`，并且 `CoordinatorDecision` / `SupervisorDecision` 能解析预期字段。

```python
from app.llm.schemas import CoordinatorDecision, SupervisorDecision
from app.prompts import PromptRenderer


def test_prompt_renderer_loads_coordinator_template():
    renderer = PromptRenderer()
    prompt = renderer.render(
        "coordinator/system.md",
        selected_order_id=None,
        last_intent="unknown",
        current_user_id="1001",
    )
    assert "coordinator" in prompt.lower()


def test_supervisor_decision_schema_accepts_finish():
    decision = SupervisorDecision.model_validate({"next_agent": "finish", "need_handoff": False})
    assert decision.next_agent == "finish"
```

- [ ] **Step 2: 跑定向测试，确认当前缺少源码文件而失败**

Run:

```bash
cd agents && uv run pytest tests/test_prompts.py -v
```

Expected: FAIL，提示 `app.prompts` 或 `app.llm.schemas` 不存在。

- [ ] **Step 3: 从备份分支恢复最小可用 renderer 与 schema**

`agents/app/prompts.py` 直接恢复 `PromptRenderer`，保持 `Jinja2 + StrictUndefined`。`agents/app/llm/schemas.py` 先只放 graph 主链路需要的两类结构化输出：

```python
class CoordinatorDecision(BaseModel):
    action: Literal["respond", "clarify", "delegate"]
    reply: str = ""
    selected_order_id: str | None = None
    business_ready: bool = False
    reason: str = ""


class SupervisorDecision(BaseModel):
    next_agent: Literal["activity", "order", "refund", "handoff", "knowledge", "finish"]
    selected_order_id: str | None = None
    need_handoff: bool = False
    reason: str = ""
```

- [ ] **Step 4: 恢复 7 份 prompt 模板，先保证 graph 路由语义完整**

模板内容先按备份分支恢复，再按新 spec 收紧两点：

- `coordinator` 只负责 `respond / clarify / delegate`
- `supervisor` 只决定下一跳 specialist 或 `finish`

`knowledge/system.md` 本期必须补上，避免角色清单与 prompts 不一致。

- [ ] **Step 5: 跑 prompt 测试并提交这一小步**

Run:

```bash
cd agents && uv run pytest tests/test_prompts.py -v
```

Expected: PASS。

Commit:

```bash
git add agents/app/prompts.py agents/app/llm/schemas.py agents/prompts agents/tests/test_prompts.py agents/tests/fakes.py
git commit -m "feat(agents): restore graph prompt layer"
```

### Task 2: 恢复 LangGraph 状态模型与 Redis checkpointer

**Files:**
- Create: `agents/app/state.py`
- Create: `agents/app/session/checkpointer.py`
- Modify: `agents/app/session/__init__.py`
- Modify: `agents/app/session/store.py`
- Create: `agents/tests/test_graph.py`
- Modify: `agents/tests/test_session_store.py`
- Modify: `agents/tests/fakes.py`

- [ ] **Step 1: 先写状态恢复与 ownership 的失败测试**

补两类测试：

1. `RedisCheckpointSaver` 能按 `thread_id=conversation_id` 存取 checkpoint
2. `ConversationStateStore` 退化为 ownership store 后，仍会阻止其他用户访问同一 `conversation_id`

```python
def test_checkpointer_round_trip(fake_redis):
    saver = RedisCheckpointSaver(redis_client=fake_redis, ttl_seconds=60)
    config = {"configurable": {"thread_id": "conv-1"}}
    next_config = saver.put(config, checkpoint, metadata, {})
    loaded = saver.get_tuple(next_config)
    assert loaded is not None


def test_session_store_rejects_foreign_user(redis_client):
    store = ConversationStateStore(redis_client=redis_client, ttl_seconds=60)
    store.get_or_create(user_id=1, conversation_id="conv-1")
    with pytest.raises(SessionOwnershipError):
        store.get_or_create(user_id=2, conversation_id="conv-1")
```

- [ ] **Step 2: 跑定向测试，确认 checkpointer 源码尚未恢复**

Run:

```bash
cd agents && uv run pytest tests/test_session_store.py tests/test_graph.py -v
```

Expected: FAIL，提示 `app.state` 或 `app.session.checkpointer` 缺失。

- [ ] **Step 3: 恢复 `ConversationState` 与 `GraphContext`**

`agents/app/state.py` 先恢复备份分支的 typed state，并按本次 spec 补上当前需要的跨轮字段：

```python
class ConversationState(MessagesState):
    last_intent: NotRequired[Intent]
    selected_program_id: NotRequired[str | None]
    selected_order_id: NotRequired[str | None]
    current_user_id: NotRequired[str | None]
    last_refund_preview: NotRequired[dict[str, Any] | None]
    pending_confirmation: NotRequired[bool]
    pending_action: NotRequired[str | None]
    need_handoff: NotRequired[bool]
```

- [ ] **Step 4: 恢复备份分支的 `RedisCheckpointSaver`，并收窄 `session/store.py` 的职责**

`session/checkpointer.py` 基本按备份分支恢复。`session/store.py` 保留，但只做 ownership 校验，不再承载 `selected_order_id`、`last_refund_preview` 等业务状态。

- [ ] **Step 5: 跑测试确认“checkpoint 主状态 + ownership 辅助校验”成立**

Run:

```bash
cd agents && uv run pytest tests/test_session_store.py tests/test_graph.py -v
```

Expected: PASS，且 `test_session_store.py` 的断言只围绕 conversation ownership 和 conversation id 生成，不再围绕业务字段持久化。

Commit:

```bash
git add agents/app/state.py agents/app/session/checkpointer.py agents/app/session/__init__.py agents/app/session/store.py agents/tests/test_graph.py agents/tests/test_session_store.py agents/tests/fakes.py
git commit -m "feat(agents): restore graph state and checkpoint storage"
```

### Task 3: 恢复 `coordinator`、`supervisor` 与 graph 骨架

**Files:**
- Create: `agents/app/agents/__init__.py`
- Create: `agents/app/agents/coordinator.py`
- Create: `agents/app/agents/supervisor.py`
- Create: `agents/app/graph.py`
- Create: `agents/tests/test_coordinator_agent.py`
- Create: `agents/tests/test_supervisor_agent.py`
- Modify: `agents/tests/test_graph.py`
- Modify: `agents/tests/fakes.py`

- [ ] **Step 1: 先写 coordinator / supervisor / graph 的失败测试**

从备份分支恢复三类最小测试：

- `coordinator` 能把小闲聊判成 `respond`
- `coordinator` 能把业务请求判成 `delegate`
- `supervisor` 能把退款诉求路由到 `refund`
- graph 能在 `coordinator -> supervisor -> finish` 这条短链上返回 `final_reply`

```python
def test_coordinator_delegates_business_request():
    llm = FakeStructuredLLM([{"action": "delegate", "business_ready": True}])
    result = CoordinatorAgent(llm=llm).handle({"messages": [{"role": "user", "content": "帮我查订单"}]})
    assert result["action"] == "delegate"


def test_supervisor_routes_refund_request():
    llm = FakeStructuredLLM([{"next_agent": "refund", "need_handoff": False}])
    result = SupervisorAgent(llm=llm).handle({"messages": [{"role": "user", "content": "我要退款"}]})
    assert result["next_agent"] == "refund"
```

- [ ] **Step 2: 跑定向测试，确认 agent 与 graph 文件尚未恢复**

Run:

```bash
cd agents && uv run pytest tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_graph.py -v
```

Expected: FAIL，提示 `app.agents.*` 或 `app.graph` 不存在。

- [ ] **Step 3: 恢复 `CoordinatorAgent` 与 `SupervisorAgent`**

两个 agent 尽量按备份分支恢复，但只保留 graph 主链路用到的依赖：

- `CoordinatorAgent`：`PromptRenderer + CoordinatorDecision`
- `SupervisorAgent`：`PromptRenderer + SupervisorDecision`

不要在这一步引入 specialist 或工具调用逻辑。

- [ ] **Step 4: 恢复最小 graph 骨架，只接 `prepare_turn -> coordinator -> supervisor -> finish`**

`agents/app/graph.py` 先恢复节点装配和条件跳转，但 specialist 节点先只放 stub，保证 graph 编译成功：

```python
builder = StateGraph(ConversationState, context_schema=GraphContext)
builder.add_node("prepare_turn", _prepare_turn_node)
builder.add_node("coordinator", _coordinator_node)
builder.add_node("supervisor", _supervisor_node)
```

`_prepare_turn_node` 负责清理 turn-local 字段，避免上一轮脏状态残留。

- [ ] **Step 5: 跑 graph 骨架测试并提交**

Run:

```bash
cd agents && uv run pytest tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_graph.py -v
```

Expected: PASS。

Commit:

```bash
git add agents/app/agents/__init__.py agents/app/agents/coordinator.py agents/app/agents/supervisor.py agents/app/graph.py agents/tests/test_coordinator_agent.py agents/tests/test_supervisor_agent.py agents/tests/test_graph.py agents/tests/fakes.py
git commit -m "feat(agents): restore coordinator supervisor graph skeleton"
```

### Task 4: 恢复 specialist 基类与 `order` / `refund` 主链路

**Files:**
- Create: `agents/app/agents/base.py`
- Create: `agents/app/agents/order.py`
- Create: `agents/app/agents/refund.py`
- Modify: `agents/app/graph.py`
- Modify: `agents/app/mcp_client/registry.py`
- Create: `agents/tests/test_agents.py`
- Modify: `agents/tests/test_graph.py`
- Modify: `agents/tests/test_order_refund_flow.py`
- Modify: `agents/tests/fakes.py`

- [ ] **Step 1: 先写失败测试，锁定订单与退款主流程**

在 `agents/tests/test_order_refund_flow.py` 增加 graph 版本的两个关键用例：

1. 没有 `selected_order_id` 时，订单查询先调用 `list_user_orders`
2. 退款请求必须先预览，再在确认后调用 `refund_order`

```python
async def test_graph_lists_orders_before_refund_submit():
    registry = FakeRegistry(
        order_tools={"list_user_orders": [{"order_id": "ORD-1", "status": "PAID"}]},
        refund_tools={"preview_refund_order": {"allow_refund": True, "refund_amount": "100"}},
    )
    result = await run_graph("我要退款", registry=registry, user_id="1001")
    assert "订单" in result["final_reply"]
```

- [ ] **Step 2: 跑定向测试，确认 specialist 尚未接回**

Run:

```bash
cd agents && uv run pytest tests/test_order_refund_flow.py tests/test_graph.py -v
```

Expected: FAIL，提示 graph 跳不到 `order` / `refund`，或 specialist 文件不存在。

- [ ] **Step 3: 恢复 `ToolCallingAgent` 基类，并适配当前 `MCPToolRegistry`**

`agents/app/agents/base.py` 从备份分支恢复，但要删掉旧的 RPC-only 假设；`get_tools()` 和 `find_tool()` 统一走当前 `MCPToolRegistry.get_tools(toolset)`。

```python
async def get_tools(self) -> list:
    return await self.registry.get_tools(self.toolset)
```

不要把旧 `ToolBroker` 或 `TaskCard` 再引回主链路。

- [ ] **Step 4: 恢复 `order.py` 与 `refund.py`，并把 graph 接到两个 specialist**

`order.py` 先支持：

- `list_user_orders`
- `get_order_detail_for_service`

`refund.py` 先支持：

- `preview_refund_order`
- `refund_order`

这一步在 `graph.py` 中补上：

- `builder.add_node("order", _order_node)`
- `builder.add_node("refund", _refund_node)`
- `builder.add_edge("order", "supervisor")`
- `builder.add_edge("refund", "supervisor")`

并把 `last_refund_preview` / `selected_order_id` 正确回写到 graph state。

- [ ] **Step 5: 跑订单/退款主流程测试并提交**

Run:

```bash
cd agents && uv run pytest tests/test_agents.py tests/test_graph.py tests/test_order_refund_flow.py -v
```

Expected: PASS，且退款链路满足“预览先于提交”的硬约束。

Commit:

```bash
git add agents/app/agents/base.py agents/app/agents/order.py agents/app/agents/refund.py agents/app/graph.py agents/app/mcp_client/registry.py agents/tests/test_agents.py agents/tests/test_graph.py agents/tests/test_order_refund_flow.py agents/tests/fakes.py
git commit -m "feat(agents): restore order and refund specialists"
```

### Task 5: 恢复 `activity` / `handoff` / `knowledge` specialist

**Files:**
- Create: `agents/app/agents/activity.py`
- Create: `agents/app/agents/handoff.py`
- Create: `agents/app/agents/knowledge.py`
- Modify: `agents/app/graph.py`
- Modify: `agents/app/knowledge/service.py`
- Modify: `agents/app/mcp_client/registry.py`
- Modify: `agents/tests/test_handoff_flow.py`
- Modify: `agents/tests/test_knowledge_agent.py`
- Modify: `agents/tests/test_graph.py`
- Create: `agents/tests/test_e2e_flows.py`

- [ ] **Step 1: 先写失败测试，覆盖剩余三类 specialist 行为**

新增或恢复三类断言：

- `activity` 能返回节目/票档咨询结果
- `handoff` 会创建工单并设置 `need_handoff=True`
- `knowledge` 对基础百科返回结果，对实时八卦返回边界提示

```python
async def test_handoff_specialist_sets_need_handoff():
    result = await HandoffAgent(registry=registry, llm=None).handle(state)
    assert result["need_handoff"] is True
```

- [ ] **Step 2: 跑定向测试，确认 graph 还没有这三个 specialist**

Run:

```bash
cd agents && uv run pytest tests/test_handoff_flow.py tests/test_knowledge_agent.py tests/test_graph.py -v
```

Expected: FAIL。

- [ ] **Step 3: 恢复 `activity.py` 与 `handoff.py`，继续复用当前 MCP toolset**

`activity.py` 走 `activity` toolset；`handoff.py` 走 `handoff` toolset，并兼容 `request_handoff` 与 `create_handoff_ticket` 两个工具名。

- [ ] **Step 4: 恢复 `knowledge.py`，并保持知识链路不走 MCP**

`knowledge.py` 直接包一层现有 `KnowledgeService`，图内保持独立 specialist。对实时新闻、八卦、热搜要返回边界提示，而不是编造实时事实。

- [ ] **Step 5: 把三类 specialist 接到 graph 并回归测试**

Run:

```bash
cd agents && uv run pytest tests/test_handoff_flow.py tests/test_knowledge_agent.py tests/test_graph.py tests/test_e2e_flows.py -v
```

Expected: PASS。

Commit:

```bash
git add agents/app/agents/activity.py agents/app/agents/handoff.py agents/app/agents/knowledge.py agents/app/graph.py agents/app/knowledge/service.py agents/app/mcp_client/registry.py agents/tests/test_handoff_flow.py agents/tests/test_knowledge_agent.py agents/tests/test_graph.py agents/tests/test_e2e_flows.py
git commit -m "feat(agents): restore remaining specialists"
```

### Task 6: 切换 `/agent/chat` 到 graph 主链路并保持 contract 兼容

**Files:**
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/api/schemas.py`
- Modify: `agents/app/main.py`
- Modify: `agents/app/config.py`
- Modify: `agents/tests/test_api.py`
- Modify: `agents/tests/test_e2e_contract.py`
- Modify: `agents/tests/test_smoke.py`
- Modify: `agents/tests/test_config.py`

- [ ] **Step 1: 先写 API 失败测试，锁定 graph 化后的响应 contract**

在 `agents/tests/test_api.py` 增加断言：

- `/agent/chat` 仍然返回 `conversationId`、`reply`、`status`
- `conversationId` 由 ownership store 生成或复用
- graph 回复能映射到 `status="completed"` 或 `status="handoff"`

```python
def test_chat_api_returns_current_contract(client):
    response = client.post("/agent/chat", json={"message": "你好"}, headers={"X-User-Id": "1001"})
    payload = response.json()
    assert set(payload) == {"conversationId", "reply", "status"}
```

- [ ] **Step 2: 跑定向测试，确认当前 API 还绑定 `ParentAgent`**

Run:

```bash
cd agents && uv run pytest tests/test_api.py tests/test_e2e_contract.py -v
```

Expected: FAIL 或仍在使用 `ParentAgent` 路径。

- [ ] **Step 3: 把 `api/routes.py` 切到 `build_graph_app()`，保留薄 ownership store**

`get_agent_runtime()` 改为缓存 graph app；`chat()` 内的调用方式切到：

```python
result = await graph.ainvoke(
    {"messages": [{"role": "user", "content": request.message}]},
    config={"configurable": {"thread_id": session.conversation_id}},
    context={"registry": registry, "llm": llm, "current_user_id": str(user_id)},
)
```

注意：

- 不再把 `session.model_dump()` 当业务状态注入 graph
- `session_store.save(...)` 只用于保存 ownership
- API 层从 graph result 中提取 `final_reply` 与 `need_handoff`

- [ ] **Step 4: 跑 API 合同测试并补 smoke/config 断言**

Run:

```bash
cd agents && uv run pytest tests/test_api.py tests/test_e2e_contract.py tests/test_smoke.py tests/test_config.py -v
```

Expected: PASS。

- [ ] **Step 5: 提交 API 主链路切换**

```bash
git add agents/app/api/routes.py agents/app/api/schemas.py agents/app/main.py agents/app/config.py agents/tests/test_api.py agents/tests/test_e2e_contract.py agents/tests/test_smoke.py agents/tests/test_config.py
git commit -m "feat(agents): switch chat api to langgraph runtime"
```

### Task 7: 清理热路径依赖、补文档并做回归验证

**Files:**
- Modify: `agents/README.md`
- Modify: `README.md`
- Modify: `agents/tests/test_docs.py`
- Optional Modify: `agents/tests/test_parent_agent.py`
- Optional Modify: `agents/tests/test_subagent_runtime.py`
- Optional Modify: `agents/tests/test_task_card.py`
- Optional Modify: `agents/tests/test_skill_resolver.py`
- Optional Modify: `agents/tests/test_tool_broker.py`

- [ ] **Step 1: 先写 docs 失败测试，锁定 README 必须反映新的 graph 主链路**

在 `agents/tests/test_docs.py` 增加断言：

- `agents/README.md` 必须写明 `FastAPI + LangGraph + MCP + Redis`
- `/agent/chat` 主链路描述应是 `coordinator -> supervisor -> specialist`
- 使用 `session/checkpointer.py` 做主状态存储

- [ ] **Step 2: 跑 docs 测试，确认 README 仍描述旧主链路**

Run:

```bash
cd agents && uv run pytest tests/test_docs.py -v
```

Expected: FAIL。

- [ ] **Step 3: 更新 README，并把旧主链路标记为非热路径兼容代码**

文档中要明确：

- 当前热路径是 graph
- `ParentAgent`、`TaskCard`、`SubagentRuntime` 仅暂存，不再是 `/agent/chat` 主链路
- 测试命令以 graph、API、checkpoint、主流程用例为主

- [ ] **Step 4: 运行一轮回归测试，确认恢复方案整体成立**

Run:

```bash
cd agents && uv run pytest tests/test_prompts.py tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_graph.py tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_knowledge_agent.py tests/test_api.py tests/test_e2e_contract.py tests/test_session_store.py tests/test_docs.py -v
```

Expected: PASS。

如果时间允许，再补一轮更广的 smoke：

```bash
cd agents && uv run pytest -v
```

如果这里暴露的是旧热路径残留测试，再按“退出主链路但暂不删除”的原则定向修正断言，不要顺手重构无关模块。

- [ ] **Step 5: 提交文档与验证收尾**

```bash
git add agents/README.md README.md agents/tests/test_docs.py agents/tests/test_parent_agent.py agents/tests/test_subagent_runtime.py agents/tests/test_task_card.py agents/tests/test_skill_resolver.py agents/tests/test_tool_broker.py
git commit -m "docs(agents): document langgraph harness runtime"
```
