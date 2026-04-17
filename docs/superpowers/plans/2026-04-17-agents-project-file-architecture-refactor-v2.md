# Agents Project File Architecture Refactor V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将整个 `agents` 项目重整为按运行时模块组织的目录结构：`api / graph / agents / runs / conversations / integrations / shared`，移除旧的混合式目录，建立清晰稳定的重构基线。

**Architecture:** 本次重构采用“硬切”策略，不保留兼容层，不保留旧导出，不要求旧 import 继续可用。目录划分优先表达系统运行结构：`graph` 只做编排，`agents` 只放角色与能力，`runs` 单独表达执行过程，`conversations` 承接线程与消息资源，`integrations` 统一收口外部系统接入。

**Tech Stack:** Python 3.12、FastAPI、LangGraph 1.1.x、LangChain、Redis、MySQL、Pytest

---

## 参考上下文

- 当前启动入口：`agents/app/main.py`
- 当前 HTTP 层：`agents/app/api/routes.py`、`agents/app/api/schemas.py`
- 当前 graph 层：`agents/app/graph.py`、`agents/app/state.py`
- 当前 runtime：`agents/app/agent_runtime/*.py`
- 当前 agents：`agents/app/agents/*.py`
- 当前 runs：`agents/app/runs/*.py`
- 当前 threads：`agents/app/threads/*.py`
- 当前 messages：`agents/app/messages/*.py`
- 当前 MCP：`agents/app/mcp_client/*.py`
- 当前 LLM：`agents/app/llm/*.py`
- 当前 Redis checkpoint：`agents/app/session/checkpointer.py`
- 当前知识服务：`agents/app/knowledge/service.py`
- 当前 prompts：`agents/prompts/*/system.md`

## 本次重构边界

### 本次要做

- 重整 `agents/app` 顶层目录为运行时模块结构
- 改造启动入口为 `app.api.app:app`
- 重构 prompts 命名与加载方式
- 将 graph 编排从 agent 实现中剥离
- 将 thread/message 收口到 `conversations`
- 将 run 资源与 run 执行过程收口到 `runs`
- 将外部依赖统一收口到 `integrations`
- 全量修改 imports、README、测试路径与测试引用

### 本次不做

- 不修改外部 API 契约
- 不修改 MySQL 表结构
- 不修改 graph 业务策略
- 不先重做退款 HITL 业务策略
- 不保留旧路径兼容层
- 不保留旧入口兼容导出

---

## 目标目录结构

```text
agents/app/
  api/
    app.py
    routes.py
    schemas.py
    dependencies.py

  graph/
    builder.py
    state.py
    routing.py
    nodes.py
    subgraphs/
      refund.py

  agents/
    llm.py
    coordinator.py
    supervisor.py
    tools/
      human_tools.py
    specialists/
      activity_specialist.py
      order_specialist.py
      refund_specialist.py
      handoff_specialist.py
      knowledge_specialist.py

  runs/
    models.py
    repository.py
    event_models.py
    event_store.py
    tool_call_models.py
    tool_call_repository.py
    tool_call_contract.py
    interrupt_models.py
    execution/
      runtime.py
      callbacks.py
      executor.py
      stream.py
      projector.py
      resume.py
      interrupt_bridge.py
      event_bus.py

  conversations/
    threads/
      models.py
      repository.py
      service.py
    messages/
      models.py
      repository.py
      service.py

  integrations/
    mcp/
      registry.py
      interceptor.py
      execution_context.py
    knowledge/
      service.py
    storage/
      mysql.py
      redis.py

  shared/
    config.py
    errors.py
    ids.py
    cursor.py
    prompt_loader.py
```

## Prompts 命名约定

```text
agents/prompts/
  coordinator.md
  supervisor.md
  activity_specialist.md
  order_specialist.md
  refund_specialist.md
  handoff_specialist.md
  knowledge_specialist.md
```

规则：

- prompt 文件与 Python 文件一一对齐
- `coordinator.py` 对应 `coordinator.md`
- `activity_specialist.py` 对应 `activity_specialist.md`
- 不再保留 `*/system.md` 多级目录结构

## 模块职责

- `api/`
  - FastAPI app 创建、HTTP 路由、schema、依赖注入
  - 不承载 graph 编排细节，不承载 repository 实现细节

- `graph/`
  - 只做图编排
  - `builder.py` 负责整图装配
  - `routing.py` 负责图跳转判断
  - `nodes.py` 负责 graph state 与 agent 调用之间的适配
  - `subgraphs/` 放退款等子图

- `agents/`
  - 只放角色与能力
  - `coordinator.py`、`supervisor.py` 是决策角色
  - `specialists/` 放真正 specialist
  - `tools/` 放 agent 侧工具封装
  - `llm.py` 放模型入口

- `runs/`
  - `Run` 资源定义 + run 执行过程
  - 顶层放模型、事件、tool call、repository
  - `execution/` 放 runtime、executor、stream、resume、projector 等执行链路

- `conversations/`
  - 承接 thread/message 资源
  - `threads/`、`messages/` 分开收口

- `integrations/`
  - 收口外部系统接入
  - MCP、知识服务、MySQL/Redis 连接与存储适配统一放这里

- `shared/`
  - 仅放跨模块稳定小件
  - 禁止堆放业务逻辑

---

## 现状到目标的映射

- `agents/app/main.py` -> 删除，入口改为 `agents/app/api/app.py`
- `agents/app/api/routes.py` -> `agents/app/api/routes.py`（保留路径，重写职责）
- `agents/app/api/schemas.py` -> `agents/app/api/schemas.py`（保留路径，重写职责）
- `agents/app/graph.py` -> `agents/app/graph/builder.py`
- `agents/app/state.py` -> `agents/app/graph/state.py`
- `agents/app/agents/activity.py` -> `agents/app/agents/specialists/activity_specialist.py`
- `agents/app/agents/order.py` -> `agents/app/agents/specialists/order_specialist.py`
- `agents/app/agents/refund.py` -> `agents/app/agents/specialists/refund_specialist.py`
- `agents/app/agents/handoff.py` -> `agents/app/agents/specialists/handoff_specialist.py`
- `agents/app/agents/knowledge.py` -> `agents/app/agents/specialists/knowledge_specialist.py`
- `agents/app/agents/coordinator.py` -> `agents/app/agents/coordinator.py`（保留路径，重写 prompt 与依赖引用）
- `agents/app/agents/supervisor.py` -> `agents/app/agents/supervisor.py`（保留路径，重写 prompt 与依赖引用）
- `agents/app/agents/refund_hitl_flow.py` -> `agents/app/graph/subgraphs/refund.py`
- `agents/app/agent_runtime/service.py` -> `agents/app/runs/execution/runtime.py`
- `agents/app/agent_runtime/callbacks.py` -> `agents/app/runs/execution/callbacks.py`
- `agents/app/agent_runtime/human_tools.py` -> `agents/app/agents/tools/human_tools.py`
- `agents/app/agent_runtime/interrupt_models.py` -> `agents/app/runs/interrupt_models.py`
- `agents/app/messages/models.py` -> `agents/app/conversations/messages/models.py`
- `agents/app/messages/repository.py` -> `agents/app/conversations/messages/repository.py`
- `agents/app/messages/service.py` -> `agents/app/conversations/messages/service.py`
- `agents/app/threads/models.py` -> `agents/app/conversations/threads/models.py`
- `agents/app/threads/repository.py` -> `agents/app/conversations/threads/repository.py`
- `agents/app/threads/service.py` -> `agents/app/conversations/threads/service.py`
- `agents/app/runs/service.py` -> 删除，由 `agents/app/runs/repository.py` + `agents/app/runs/execution/*` 分担
- `agents/app/runs/executor.py` -> `agents/app/runs/execution/executor.py`
- `agents/app/runs/stream_service.py` -> `agents/app/runs/execution/stream.py`
- `agents/app/runs/event_bus.py` -> `agents/app/runs/execution/event_bus.py`
- `agents/app/runs/event_projector.py` -> `agents/app/runs/execution/projector.py`
- `agents/app/runs/interrupt_bridge.py` -> `agents/app/runs/execution/interrupt_bridge.py`
- `agents/app/runs/resume_command_executor.py` -> `agents/app/runs/execution/resume.py`
- `agents/app/llm/client.py` + `agents/app/llm/schemas.py` -> `agents/app/agents/llm.py`
- `agents/app/mcp_client/*` -> `agents/app/integrations/mcp/*`
- `agents/app/knowledge/service.py` -> `agents/app/integrations/knowledge/service.py`
- `agents/app/session/checkpointer.py` -> `agents/app/integrations/storage/redis.py`
- `agents/app/common/*` + `agents/app/config.py` -> `agents/app/shared/*`
- `agents/app/prompts.py` -> `agents/app/shared/prompt_loader.py`
- `agents/prompts/*/system.md` -> `agents/prompts/*.md`

---

### Task 1: 建立骨架并切换入口

**Files:**
- Create: `agents/app/api/app.py`
- Create: `agents/app/api/dependencies.py`
- Create: `agents/app/graph/__init__.py`
- Create: `agents/app/graph/subgraphs/__init__.py`
- Create: `agents/app/agents/tools/__init__.py`
- Create: `agents/app/agents/specialists/__init__.py`
- Create: `agents/app/runs/execution/__init__.py`
- Create: `agents/app/conversations/__init__.py`
- Create: `agents/app/conversations/threads/__init__.py`
- Create: `agents/app/conversations/messages/__init__.py`
- Create: `agents/app/integrations/__init__.py`
- Create: `agents/app/integrations/mcp/__init__.py`
- Create: `agents/app/integrations/knowledge/__init__.py`
- Create: `agents/app/integrations/storage/__init__.py`
- Create: `agents/app/shared/__init__.py`
- Modify: `agents/README.md`
- Delete: `agents/app/main.py`
- Test: `agents/tests/test_smoke.py`
- Test: `agents/tests/test_docs.py`

- [ ] **Step 1: 写骨架与入口失败测试**

补充测试覆盖：

- 新入口为 `app.api.app:app`
- 新顶层目录均可导入
- README 启动命令更新为新入口

- [ ] **Step 2: 运行骨架测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_smoke.py tests/test_docs.py -v
```

Expected:

- FAIL，失败点集中在新目录与新入口尚未建立

- [ ] **Step 3: 建立目录骨架并切换入口**

实施内容：

- 创建新包目录与 `__init__.py`
- 新建 `api/app.py` 并创建 `FastAPI app`
- 删除 `main.py`
- README 启动命令改为 `uv run uvicorn app.api.app:app --reload`

- [ ] **Step 4: 运行骨架测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_smoke.py tests/test_docs.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/api/app.py agents/app/api/dependencies.py agents/app/graph agents/app/agents/tools agents/app/agents/specialists agents/app/runs/execution agents/app/conversations agents/app/integrations agents/app/shared agents/README.md
git rm agents/app/main.py
git commit -m "refactor: establish agents runtime-module skeleton"
```

---

### Task 2: 重构 prompts 与 agent 命名

**Files:**
- Create: `agents/prompts/coordinator.md`
- Create: `agents/prompts/supervisor.md`
- Create: `agents/prompts/activity_specialist.md`
- Create: `agents/prompts/order_specialist.md`
- Create: `agents/prompts/refund_specialist.md`
- Create: `agents/prompts/handoff_specialist.md`
- Create: `agents/prompts/knowledge_specialist.md`
- Create: `agents/app/shared/prompt_loader.py`
- Modify: `agents/app/agents/coordinator.py`
- Modify: `agents/app/agents/supervisor.py`
- Delete: `agents/prompts/coordinator/system.md`
- Delete: `agents/prompts/supervisor/system.md`
- Delete: `agents/prompts/activity/system.md`
- Delete: `agents/prompts/order/system.md`
- Delete: `agents/prompts/refund/system.md`
- Delete: `agents/prompts/handoff/system.md`
- Delete: `agents/prompts/knowledge/system.md`
- Test: `agents/tests/test_prompts.py`
- Test: `agents/tests/test_coordinator_agent.py`
- Test: `agents/tests/test_supervisor_agent.py`

- [ ] **Step 1: 写 prompt 重命名失败测试**

补充测试覆盖：

- prompt 文件名与 Python 文件名一一对齐
- `PromptRenderer` 替换为新 `prompt_loader`
- `coordinator`、`supervisor` 新路径仍能正确加载 prompt

- [ ] **Step 2: 运行 prompt 与决策角色测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_prompts.py tests/test_coordinator_agent.py tests/test_supervisor_agent.py -v
```

Expected:

- FAIL，失败点集中在 prompt 路径与加载器变更

- [ ] **Step 3: 重写 prompt 目录与加载器**

实施内容：

- 将多级 `system.md` prompt 重命名为单文件 prompt
- 新建 `shared/prompt_loader.py`
- `coordinator.py`、`supervisor.py` 改为依赖新加载器

- [ ] **Step 4: 运行 prompt 与决策角色测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_prompts.py tests/test_coordinator_agent.py tests/test_supervisor_agent.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/prompts agents/app/shared/prompt_loader.py agents/app/agents/coordinator.py agents/app/agents/supervisor.py agents/tests/test_prompts.py agents/tests/test_coordinator_agent.py agents/tests/test_supervisor_agent.py
git commit -m "refactor: align agents prompts with role filenames"
```

---

### Task 3: 重构 graph 编排

**Files:**
- Create: `agents/app/graph/builder.py`
- Create: `agents/app/graph/state.py`
- Create: `agents/app/graph/routing.py`
- Create: `agents/app/graph/nodes.py`
- Create: `agents/app/graph/subgraphs/refund.py`
- Modify: `agents/app/agents/coordinator.py`
- Modify: `agents/app/agents/supervisor.py`
- Create: `agents/app/agents/specialists/activity_specialist.py`
- Create: `agents/app/agents/specialists/order_specialist.py`
- Create: `agents/app/agents/specialists/refund_specialist.py`
- Create: `agents/app/agents/specialists/handoff_specialist.py`
- Create: `agents/app/agents/specialists/knowledge_specialist.py`
- Delete: `agents/app/graph.py`
- Delete: `agents/app/state.py`
- Delete: `agents/app/agents/activity.py`
- Delete: `agents/app/agents/order.py`
- Delete: `agents/app/agents/refund.py`
- Delete: `agents/app/agents/handoff.py`
- Delete: `agents/app/agents/knowledge.py`
- Delete: `agents/app/agents/refund_hitl_flow.py`
- Test: `agents/tests/test_graph.py`
- Test: `agents/tests/test_order_refund_flow.py`
- Test: `agents/tests/test_handoff_flow.py`
- Test: `agents/tests/test_agents.py`
- Test: `agents/tests/test_knowledge_agent.py`

- [ ] **Step 1: 写 graph/agents 拆分失败测试**

补充测试覆盖：

- `graph` 只通过 `builder.py` 装配整图
- `nodes.py` 只做 state 映射与节点适配
- specialist 文件名改为 `*_specialist.py`

- [ ] **Step 2: 运行 graph 与 specialist 测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_graph.py tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_agents.py tests/test_knowledge_agent.py -v
```

Expected:

- FAIL，失败点集中在 graph 文件拆分与 specialist 文件重命名

- [ ] **Step 3: 重构 graph 与 specialist**

实施内容：

- 新建 `graph/builder.py`、`graph/state.py`、`graph/routing.py`、`graph/nodes.py`
- `nodes.py` 调用 `coordinator.py`、`supervisor.py` 与 `specialists/*.py`
- 退款 HITL 子图迁到 `graph/subgraphs/refund.py`
- 删除旧 `graph.py`、`state.py` 与旧 specialist 文件

- [ ] **Step 4: 运行 graph 与 specialist 测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_graph.py tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_agents.py tests/test_knowledge_agent.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/graph agents/app/agents agents/tests/test_graph.py agents/tests/test_order_refund_flow.py agents/tests/test_handoff_flow.py agents/tests/test_agents.py agents/tests/test_knowledge_agent.py
git rm agents/app/graph.py agents/app/state.py agents/app/agents/activity.py agents/app/agents/order.py agents/app/agents/refund.py agents/app/agents/handoff.py agents/app/agents/knowledge.py agents/app/agents/refund_hitl_flow.py
git commit -m "refactor: separate graph orchestration from specialist agents"
```

---

### Task 4: 收口 conversations

**Files:**
- Create: `agents/app/conversations/threads/models.py`
- Create: `agents/app/conversations/threads/repository.py`
- Create: `agents/app/conversations/threads/service.py`
- Create: `agents/app/conversations/messages/models.py`
- Create: `agents/app/conversations/messages/repository.py`
- Create: `agents/app/conversations/messages/service.py`
- Delete: `agents/app/threads/models.py`
- Delete: `agents/app/threads/repository.py`
- Delete: `agents/app/threads/service.py`
- Delete: `agents/app/messages/models.py`
- Delete: `agents/app/messages/repository.py`
- Delete: `agents/app/messages/service.py`
- Test: `agents/tests/test_thread_message_run_repositories.py`
- Test: `agents/tests/test_thread_message_run_services.py`
- Test: `agents/tests/test_api.py`

- [ ] **Step 1: 写 conversations 模块失败测试**

补充测试覆盖：

- thread/message 新路径导入成功
- thread/message 查询与分页语义不变
- API 层改用 `conversations` 模块

- [ ] **Step 2: 运行 conversations 相关测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_thread_message_run_repositories.py tests/test_thread_message_run_services.py tests/test_api.py -v
```

Expected:

- FAIL，失败点集中在 `threads`、`messages` 搬迁

- [ ] **Step 3: 迁移 threads/messages 到 conversations**

实施内容：

- 建立 `conversations/threads` 与 `conversations/messages`
- 更新 service、repository、model 引用
- 删除旧 `threads`、`messages` 目录实现

- [ ] **Step 4: 运行 conversations 相关测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_thread_message_run_repositories.py tests/test_thread_message_run_services.py tests/test_api.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/conversations agents/tests/test_thread_message_run_repositories.py agents/tests/test_thread_message_run_services.py agents/tests/test_api.py
git rm agents/app/threads/models.py agents/app/threads/repository.py agents/app/threads/service.py agents/app/messages/models.py agents/app/messages/repository.py agents/app/messages/service.py
git commit -m "refactor: consolidate threads and messages into conversations"
```

---

### Task 5: 收口 runs

**Files:**
- Create: `agents/app/runs/execution/runtime.py`
- Create: `agents/app/runs/execution/callbacks.py`
- Create: `agents/app/runs/execution/executor.py`
- Create: `agents/app/runs/execution/stream.py`
- Create: `agents/app/runs/execution/projector.py`
- Create: `agents/app/runs/execution/resume.py`
- Create: `agents/app/runs/execution/interrupt_bridge.py`
- Create: `agents/app/runs/execution/event_bus.py`
- Create: `agents/app/runs/interrupt_models.py`
- Modify: `agents/app/runs/models.py`
- Modify: `agents/app/runs/repository.py`
- Modify: `agents/app/runs/event_models.py`
- Modify: `agents/app/runs/event_store.py`
- Modify: `agents/app/runs/tool_call_models.py`
- Modify: `agents/app/runs/tool_call_repository.py`
- Modify: `agents/app/runs/tool_call_contract.py`
- Delete: `agents/app/agent_runtime/service.py`
- Delete: `agents/app/agent_runtime/callbacks.py`
- Delete: `agents/app/agent_runtime/interrupt_models.py`
- Delete: `agents/app/runs/executor.py`
- Delete: `agents/app/runs/stream_service.py`
- Delete: `agents/app/runs/event_bus.py`
- Delete: `agents/app/runs/event_projector.py`
- Delete: `agents/app/runs/interrupt_bridge.py`
- Delete: `agents/app/runs/resume_command_executor.py`
- Delete: `agents/app/runs/service.py`
- Test: `agents/tests/test_agent_runtime_service.py`
- Test: `agents/tests/test_run_executor.py`
- Test: `agents/tests/test_run_stream_service.py`
- Test: `agents/tests/test_hitl_bridge.py`
- Test: `agents/tests/test_run_resume_cancel_api.py`

- [ ] **Step 1: 写 runs 重构失败测试**

补充测试覆盖：

- `runs` 顶层承载资源定义
- `runs/execution` 承载执行过程
- runtime/callbacks/interrupt 模型均从新路径导入

- [ ] **Step 2: 运行 runs 相关测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_agent_runtime_service.py tests/test_run_executor.py tests/test_run_stream_service.py tests/test_hitl_bridge.py tests/test_run_resume_cancel_api.py -v
```

Expected:

- FAIL，失败点集中在 run 执行链路重组

- [ ] **Step 3: 重构 runs 资源与执行过程**

实施内容：

- 建立 `runs/execution/*`
- `agent_runtime/*` 合并进入 `runs/execution/*` 与 `runs/interrupt_models.py`
- 删除旧执行文件
- `runs` 顶层只保留资源与事件定义

- [ ] **Step 4: 运行 runs 相关测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_agent_runtime_service.py tests/test_run_executor.py tests/test_run_stream_service.py tests/test_hitl_bridge.py tests/test_run_resume_cancel_api.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/runs agents/tests/test_agent_runtime_service.py agents/tests/test_run_executor.py agents/tests/test_run_stream_service.py agents/tests/test_hitl_bridge.py agents/tests/test_run_resume_cancel_api.py
git rm agents/app/agent_runtime/service.py agents/app/agent_runtime/callbacks.py agents/app/agent_runtime/interrupt_models.py agents/app/runs/executor.py agents/app/runs/stream_service.py agents/app/runs/event_bus.py agents/app/runs/event_projector.py agents/app/runs/interrupt_bridge.py agents/app/runs/resume_command_executor.py agents/app/runs/service.py
git commit -m "refactor: separate run resources from execution pipeline"
```

---

### Task 6: 收口 integrations 与 shared

**Files:**
- Create: `agents/app/integrations/mcp/registry.py`
- Create: `agents/app/integrations/mcp/interceptor.py`
- Create: `agents/app/integrations/mcp/execution_context.py`
- Create: `agents/app/integrations/knowledge/service.py`
- Create: `agents/app/integrations/storage/mysql.py`
- Create: `agents/app/integrations/storage/redis.py`
- Create: `agents/app/shared/config.py`
- Create: `agents/app/shared/errors.py`
- Create: `agents/app/shared/ids.py`
- Create: `agents/app/shared/cursor.py`
- Modify: `agents/app/agents/llm.py`
- Delete: `agents/app/llm/client.py`
- Delete: `agents/app/llm/schemas.py`
- Delete: `agents/app/mcp_client/registry.py`
- Delete: `agents/app/mcp_client/interceptor.py`
- Delete: `agents/app/mcp_client/execution_context.py`
- Delete: `agents/app/knowledge/service.py`
- Delete: `agents/app/session/checkpointer.py`
- Delete: `agents/app/common/errors.py`
- Delete: `agents/app/common/ids.py`
- Delete: `agents/app/common/cursor.py`
- Delete: `agents/app/config.py`
- Delete: `agents/app/prompts.py`
- Test: `agents/tests/test_mcp_registry.py`
- Test: `agents/tests/test_mcp_interceptor.py`
- Test: `agents/tests/test_go_provider_registry.py`
- Test: `agents/tests/test_session_store.py`
- Test: `agents/tests/test_config.py`

- [ ] **Step 1: 写 integrations/shared 迁移失败测试**

补充测试覆盖：

- MCP、知识服务、Redis checkpoint 从新路径导入
- LLM 入口收口到 `agents/llm.py`
- `shared` 工具模块位置变更后仍能被全项目引用

- [ ] **Step 2: 运行 integrations/shared 测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_mcp_registry.py tests/test_mcp_interceptor.py tests/test_go_provider_registry.py tests/test_session_store.py tests/test_config.py -v
```

Expected:

- FAIL，失败点集中在外部依赖目录迁移

- [ ] **Step 3: 迁移外部依赖与共享模块**

实施内容：

- `mcp_client` 迁到 `integrations/mcp`
- 知识服务迁到 `integrations/knowledge/service.py`
- Redis/MySQL 连接与 checkpoint 收口到 `integrations/storage`
- `config/common/prompts` 收口到 `shared`
- LLM 入口收口到 `agents/llm.py`

- [ ] **Step 4: 运行 integrations/shared 测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_mcp_registry.py tests/test_mcp_interceptor.py tests/test_go_provider_registry.py tests/test_session_store.py tests/test_config.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/integrations agents/app/shared agents/app/agents/llm.py agents/tests/test_mcp_registry.py agents/tests/test_mcp_interceptor.py agents/tests/test_go_provider_registry.py agents/tests/test_session_store.py agents/tests/test_config.py
git rm agents/app/llm/client.py agents/app/llm/schemas.py agents/app/mcp_client/registry.py agents/app/mcp_client/interceptor.py agents/app/mcp_client/execution_context.py agents/app/knowledge/service.py agents/app/session/checkpointer.py agents/app/common/errors.py agents/app/common/ids.py agents/app/common/cursor.py agents/app/config.py agents/app/prompts.py
git commit -m "refactor: consolidate external integrations and shared helpers"
```

---

### Task 7: 收口 API 依赖注入

**Files:**
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/api/schemas.py`
- Modify: `agents/app/api/dependencies.py`
- Modify: `agents/app/api/app.py`
- Test: `agents/tests/test_api.py`
- Test: `agents/tests/test_run_contract_api.py`
- Test: `agents/tests/test_run_resume_cancel_api.py`
- Test: `agents/tests/test_e2e_contract.py`

- [ ] **Step 1: 写 API 依赖注入失败测试**

补充测试覆盖：

- `api/routes.py` 只保留 HTTP 协议处理
- 依赖提供函数集中到 `api/dependencies.py`
- 测试里的 dependency override 全部改到新路径

- [ ] **Step 2: 运行 API 契约测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_api.py tests/test_run_contract_api.py tests/test_run_resume_cancel_api.py tests/test_e2e_contract.py -v
```

Expected:

- FAIL，失败点集中在 HTTP 依赖注入重组

- [ ] **Step 3: 重构 API 依赖注入**

实施内容：

- `api/routes.py` 只保留路由与响应映射
- 依赖函数迁到 `api/dependencies.py`
- `api/app.py` 只负责创建 app 与注册 router

- [ ] **Step 4: 运行 API 契约测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_api.py tests/test_run_contract_api.py tests/test_run_resume_cancel_api.py tests/test_e2e_contract.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/api agents/tests/test_api.py agents/tests/test_run_contract_api.py agents/tests/test_run_resume_cancel_api.py agents/tests/test_e2e_contract.py
git commit -m "refactor: isolate agents api http concerns"
```

---

### Task 8: 全量改 import、文档与回归

**Files:**
- Modify: `agents/README.md`
- Modify: `agents/tests/test_smoke.py`
- Modify: `agents/tests/test_docs.py`
- Modify: `agents/tests/test_prompts.py`
- Modify: `agents/tests/test_graph.py`
- Modify: `agents/tests/test_agents.py`
- Modify: `agents/tests/test_knowledge_agent.py`
- Modify: `agents/tests/test_thread_message_run_repositories.py`
- Modify: `agents/tests/test_thread_message_run_services.py`
- Modify: `agents/tests/test_agent_runtime_service.py`
- Modify: `agents/tests/test_run_executor.py`
- Modify: `agents/tests/test_run_stream_service.py`
- Modify: `agents/tests/test_hitl_bridge.py`
- Modify: `agents/tests/test_mcp_registry.py`
- Modify: `agents/tests/test_mcp_interceptor.py`
- Modify: `agents/tests/test_go_provider_registry.py`
- Modify: `agents/tests/test_session_store.py`
- Modify: `agents/tests/test_api.py`
- Modify: `agents/tests/test_run_contract_api.py`
- Modify: `agents/tests/test_run_resume_cancel_api.py`
- Modify: `agents/tests/test_e2e_contract.py`
- Modify: `docs/superpowers/plans/2026-04-17-agents-project-file-architecture-refactor-v2.md`

- [ ] **Step 1: 写全量路径校验失败测试**

补充测试覆盖：

- 测试文件不再引用已删除旧路径
- README 目录说明与实际目录一致
- 新启动入口、prompt 路径、核心导入路径全部对齐

- [ ] **Step 2: 运行全量回归确认失败**

Run:

```bash
cd agents && uv run pytest tests -v
```

Expected:

- FAIL，失败点集中在残留旧 import 与文档未同步

- [ ] **Step 3: 清理全量 import 与文档**

实施内容：

- 全量替换测试引用为新路径
- README 更新新目录结构、启动方式与模块职责
- 删除所有旧路径残留引用

- [ ] **Step 4: 运行全量回归确认通过**

Run:

```bash
cd agents && uv run pytest tests -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/README.md agents/tests docs/superpowers/plans/2026-04-17-agents-project-file-architecture-refactor-v2.md
git commit -m "docs: finalize agents runtime-module architecture refactor plan"
```

---

## 执行顺序建议

1. 先建骨架并切换启动入口。
2. 再统一 prompt 命名与加载器。
3. 先稳住 `graph` 与 `agents` 边界。
4. 再迁 `conversations`，清空 thread/message 旧目录。
5. 再迁 `runs`，把执行链路彻底收口。
6. 最后迁 `integrations` 与 `shared`。
7. API 依赖注入在资源与执行链路稳定后统一清理。
8. 结尾全量清理 import、README 与测试。

## 完成标准

- `agents/app` 顶层目录稳定收敛到 `api / graph / agents / runs / conversations / integrations / shared`
- `graph` 只做编排，不再混放 specialist 主逻辑
- `agents` 只放角色与能力，不再混放 graph 装配代码
- `runs` 同时清晰表达资源定义与执行过程
- `conversations` 成为 thread/message 唯一归属
- `integrations` 成为 MCP、知识服务、MySQL/Redis 的唯一接入层
- prompt 文件名与 Python 文件名一一对齐
- 项目启动入口改为 `app.api.app:app`
- 全量测试在新目录结构下通过
