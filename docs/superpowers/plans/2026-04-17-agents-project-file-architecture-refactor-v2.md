# Agents Project File Architecture Refactor V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将整个 `agents` 项目的文件结构重整为清晰稳定的六层架构：`bootstrap / transport / application / workflows / domain / infrastructure`，先完成文件边界与目录职责收口，再为后续 graph 行为演进提供干净基础。

**Architecture:** 本次重构明确以“文件架构重整优先、运行行为尽量不变”为原则，不先改变图的业务策略，不先调整外部 HTTP 契约，不先做数据库 schema 迁移。实施路径是先建立新目录骨架与兼容导出，再按层迁移文件：先 `bootstrap + transport`，再 `workflows`，再 `domain + infrastructure`，最后统一清理 imports、README 和测试入口。

**Tech Stack:** Python 3.12、FastAPI、LangGraph 1.1.x、LangChain、Redis、MySQL、Pytest

---

## 参考上下文

- 当前入口：`agents/app/main.py`
- 当前 HTTP 层：`agents/app/api/routes.py`
- 当前 graph 层：`agents/app/graph.py`
- 当前状态定义：`agents/app/state.py`
- 当前 runtime：`agents/app/agent_runtime/service.py`
- 当前 specialist：`agents/app/agents/*.py`
- 当前 runs：`agents/app/runs/*.py`
- 当前 threads：`agents/app/threads/*.py`
- 当前 messages：`agents/app/messages/*.py`
- 当前 MCP：`agents/app/mcp_client/*.py`
- 当前 LLM：`agents/app/llm/*.py`
- 当前 Redis checkpoint：`agents/app/session/checkpointer.py`
- 当前知识服务：`agents/app/knowledge/service.py`
- 旧版局部计划：`docs/superpowers/plans/2026-04-17-agents-langgraph-architecture-refactor.md`

## 本次重构边界

### 本次要做

- 重整 `agents/app` 顶层目录
- 重新定义各目录职责
- 将 graph 相关代码从项目其余层中剥离
- 将 repo / runtime / transport / infra 分层
- 通过兼容导出降低迁移期风险
- 更新 README 与测试入口

### 本次不做

- 不修改外部 API 契约
- 不修改 MySQL 表结构
- 不重新设计 graph 策略
- 不先决定 `router / supervisor / handoff` 的终局模式
- 不引入新的工具协议或新的消息协议

---

## 目标目录结构

### 顶层目录

- `agents/app/bootstrap/`
  - `__init__.py`
  - `app.py`
  - `container.py`
  - `dependencies.py`
  - 责任：应用装配、依赖注入、FastAPI app 创建

- `agents/app/transport/http/`
  - `__init__.py`
  - `routes.py`
  - `schemas.py`
  - `presenters.py`
  - 责任：HTTP 路由、请求响应模型、DTO 映射

- `agents/app/application/`
  - `__init__.py`
  - `agent_runtime_service.py`
  - `run_execution_service.py`
  - `run_stream_service.py`
  - `thread_service.py`
  - `message_service.py`
  - `run_query_service.py`
  - 责任：用例编排、run 生命周期、跨模块协调

- `agents/app/workflows/support/`
  - `__init__.py`
  - `compile.py`
  - `state.py`
  - `context.py`
  - `edges.py`
  - `nodes/`
  - `subgraphs/`
  - 责任：LangGraph 编排、节点、条件边、子图

- `agents/app/domain/agents/`
  - `__init__.py`
  - `base.py`
  - `activity.py`
  - `coordinator.py`
  - `handoff.py`
  - `knowledge.py`
  - `order.py`
  - `refund.py`
  - `supervisor.py`
  - 责任：领域专员与领域决策封装

- `agents/app/domain/messages/`
  - `__init__.py`
  - `models.py`

- `agents/app/domain/runs/`
  - `__init__.py`
  - `models.py`
  - `event_models.py`
  - `tool_call_models.py`
  - `tool_call_contract.py`

- `agents/app/domain/threads/`
  - `__init__.py`
  - `models.py`

- `agents/app/infrastructure/llm/`
  - `__init__.py`
  - `client.py`
  - `schemas.py`

- `agents/app/infrastructure/mcp/`
  - `__init__.py`
  - `registry.py`
  - `interceptor.py`
  - `execution_context.py`

- `agents/app/infrastructure/persistence/mysql/`
  - `__init__.py`
  - `connection.py`
  - `message_repository.py`
  - `run_repository.py`
  - `run_event_store.py`
  - `thread_repository.py`
  - `tool_call_repository.py`

- `agents/app/infrastructure/persistence/redis/`
  - `__init__.py`
  - `checkpointer.py`

- `agents/app/infrastructure/knowledge/`
  - `__init__.py`
  - `service.py`

- `agents/app/shared/`
  - `__init__.py`
  - `errors.py`
  - `ids.py`
  - `cursor.py`
  - `prompts.py`
  - `config.py`
  - 责任：跨层共享的稳定小模块

### 兼容期保留入口

- `agents/app/main.py`
  - 兼容导出 `bootstrap.app:create_app`
- `agents/app/api/routes.py`
  - 兼容导出 `transport/http/routes.py`
- `agents/app/api/schemas.py`
  - 兼容导出 `transport/http/schemas.py`
- `agents/app/graph.py`
  - 兼容导出 `workflows/support/compile.py`
- `agents/app/state.py`
  - 兼容导出 `workflows/support/state.py`

---

## 现状到目标的映射

- `agents/app/api/*` -> `agents/app/transport/http/*`
- `agents/app/agent_runtime/service.py` -> `agents/app/application/agent_runtime_service.py`
- `agents/app/agent_runtime/callbacks.py` -> `agents/app/application/runtime_callbacks.py`
- `agents/app/agent_runtime/human_tools.py` -> `agents/app/application/human_tools.py`
- `agents/app/agent_runtime/interrupt_models.py` -> `agents/app/application/interrupt_models.py`
- `agents/app/agents/*` -> `agents/app/domain/agents/*`
- `agents/app/agents/refund_hitl_flow.py` -> `agents/app/workflows/support/subgraphs/refund.py`
- `agents/app/graph.py` -> `agents/app/workflows/support/compile.py`
- `agents/app/state.py` -> `agents/app/workflows/support/state.py`
- `agents/app/messages/models.py` -> `agents/app/domain/messages/models.py`
- `agents/app/messages/service.py` -> `agents/app/application/message_service.py`
- `agents/app/messages/repository.py` -> `agents/app/infrastructure/persistence/mysql/message_repository.py`
- `agents/app/threads/models.py` -> `agents/app/domain/threads/models.py`
- `agents/app/threads/service.py` -> `agents/app/application/thread_service.py`
- `agents/app/threads/repository.py` -> `agents/app/infrastructure/persistence/mysql/thread_repository.py`
- `agents/app/runs/models.py` -> `agents/app/domain/runs/models.py`
- `agents/app/runs/event_models.py` -> `agents/app/domain/runs/event_models.py`
- `agents/app/runs/tool_call_models.py` -> `agents/app/domain/runs/tool_call_models.py`
- `agents/app/runs/tool_call_contract.py` -> `agents/app/domain/runs/tool_call_contract.py`
- `agents/app/runs/service.py` -> `agents/app/application/run_query_service.py`
- `agents/app/runs/executor.py` -> `agents/app/application/run_execution_service.py`
- `agents/app/runs/stream_service.py` -> `agents/app/application/run_stream_service.py`
- `agents/app/runs/repository.py` -> `agents/app/infrastructure/persistence/mysql/run_repository.py`
- `agents/app/runs/tool_call_repository.py` -> `agents/app/infrastructure/persistence/mysql/tool_call_repository.py`
- `agents/app/runs/event_store.py` -> `agents/app/infrastructure/persistence/mysql/run_event_store.py`
- `agents/app/runs/event_bus.py` -> `agents/app/application/run_event_bus.py`
- `agents/app/runs/event_projector.py` -> `agents/app/application/run_event_projector.py`
- `agents/app/runs/interrupt_bridge.py` -> `agents/app/application/interrupt_bridge.py`
- `agents/app/runs/resume_command_executor.py` -> `agents/app/application/resume_command_executor.py`
- `agents/app/mcp_client/*` -> `agents/app/infrastructure/mcp/*`
- `agents/app/llm/*` -> `agents/app/infrastructure/llm/*`
- `agents/app/session/checkpointer.py` -> `agents/app/infrastructure/persistence/redis/checkpointer.py`
- `agents/app/knowledge/service.py` -> `agents/app/infrastructure/knowledge/service.py`
- `agents/app/common/*` + `agents/app/config.py` + `agents/app/prompts.py` -> `agents/app/shared/*`

---

### Task 1: 建立新目录骨架与兼容入口

**Files:**
- Create: `agents/app/bootstrap/__init__.py`
- Create: `agents/app/bootstrap/app.py`
- Create: `agents/app/bootstrap/container.py`
- Create: `agents/app/bootstrap/dependencies.py`
- Create: `agents/app/transport/http/__init__.py`
- Create: `agents/app/application/__init__.py`
- Create: `agents/app/workflows/support/__init__.py`
- Create: `agents/app/domain/__init__.py`
- Create: `agents/app/infrastructure/__init__.py`
- Create: `agents/app/shared/__init__.py`
- Modify: `agents/app/main.py`
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/api/schemas.py`
- Modify: `agents/app/graph.py`
- Modify: `agents/app/state.py`
- Test: `agents/tests/test_smoke.py`
- Test: `agents/tests/test_api.py`

- [ ] **Step 1: 写兼容入口失败测试**

补充测试覆盖：

- 旧 import 路径仍可工作
- 新目录已建立
- `FastAPI` 应用仍可启动

- [ ] **Step 2: 运行入口与 smoke 测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_smoke.py tests/test_api.py -v
```

Expected:

- FAIL，失败点集中在模块路径与导出位置变更

- [ ] **Step 3: 建立骨架并加兼容导出**

实施内容：

- 先创建新目录和空 `__init__.py`
- 旧入口文件改为转发导出
- 先不迁移具体实现，只让 imports 开始稳定

- [ ] **Step 4: 运行入口与 smoke 测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_smoke.py tests/test_api.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/bootstrap agents/app/transport agents/app/application agents/app/workflows agents/app/domain agents/app/infrastructure agents/app/shared agents/app/main.py agents/app/api/routes.py agents/app/api/schemas.py agents/app/graph.py agents/app/state.py agents/tests/test_smoke.py agents/tests/test_api.py
git commit -m "refactor: establish agents project architecture skeleton"
```

---

### Task 2: 迁移 transport 与 bootstrap

**Files:**
- Create: `agents/app/transport/http/routes.py`
- Create: `agents/app/transport/http/schemas.py`
- Create: `agents/app/transport/http/presenters.py`
- Modify: `agents/app/bootstrap/app.py`
- Modify: `agents/app/bootstrap/container.py`
- Modify: `agents/app/bootstrap/dependencies.py`
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/api/schemas.py`
- Test: `agents/tests/test_api.py`
- Test: `agents/tests/test_run_contract_api.py`

- [ ] **Step 1: 写 transport 迁移失败测试**

补充测试覆盖：

- `routes` 只关心 HTTP 层
- 依赖装配由 `bootstrap/dependencies.py` 提供
- `schemas` 从新路径导入后外部接口不变

- [ ] **Step 2: 运行 API 契约测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_api.py tests/test_run_contract_api.py -v
```

Expected:

- FAIL，失败点集中在 import 路径与依赖注入边界

- [ ] **Step 3: 迁移 transport 与 bootstrap 实现**

实施内容：

- 将 `api/routes.py` 主实现迁移到 `transport/http/routes.py`
- 将 `api/schemas.py` 主实现迁移到 `transport/http/schemas.py`
- 新增 `presenters.py` 承载 DTO 映射
- `bootstrap` 统一应用创建与依赖函数

- [ ] **Step 4: 运行 API 契约测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_api.py tests/test_run_contract_api.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/transport/http agents/app/bootstrap agents/app/api/routes.py agents/app/api/schemas.py agents/tests/test_api.py agents/tests/test_run_contract_api.py
git commit -m "refactor: separate agents transport and bootstrap layers"
```

---

### Task 3: 迁移 workflows，整理 graph 文件结构

**Files:**
- Create: `agents/app/workflows/support/compile.py`
- Create: `agents/app/workflows/support/state.py`
- Create: `agents/app/workflows/support/context.py`
- Create: `agents/app/workflows/support/edges.py`
- Create: `agents/app/workflows/support/nodes/__init__.py`
- Create: `agents/app/workflows/support/nodes/prepare_turn.py`
- Create: `agents/app/workflows/support/nodes/coordinator.py`
- Create: `agents/app/workflows/support/nodes/supervisor.py`
- Create: `agents/app/workflows/support/nodes/activity.py`
- Create: `agents/app/workflows/support/nodes/order.py`
- Create: `agents/app/workflows/support/nodes/knowledge.py`
- Create: `agents/app/workflows/support/nodes/handoff.py`
- Create: `agents/app/workflows/support/subgraphs/__init__.py`
- Create: `agents/app/workflows/support/subgraphs/refund.py`
- Modify: `agents/app/graph.py`
- Modify: `agents/app/state.py`
- Modify: `agents/app/agents/refund_hitl_flow.py`
- Test: `agents/tests/test_graph.py`
- Test: `agents/tests/test_order_refund_flow.py`
- Test: `agents/tests/test_handoff_flow.py`

- [ ] **Step 1: 写 graph 文件迁移失败测试**

补充测试覆盖：

- `graph.py` 仍可导入 `build_graph_app`
- graph 节点分散到独立文件
- refund HITL 位于 subgraph 路径，而不是继续放在 `agents/`

- [ ] **Step 2: 运行 graph 相关测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_graph.py tests/test_order_refund_flow.py tests/test_handoff_flow.py -v
```

Expected:

- FAIL，失败点集中在模块路径与 graph 组装位置

- [ ] **Step 3: 迁移 workflows 主实现**

实施内容：

- `compile.py` 只做图装配
- `edges.py` 存条件边函数
- 每个 node 一个文件
- `refund_hitl_flow.py` 主实现迁到 `subgraphs/refund.py`
- 旧 `graph.py` / `state.py` 仅兼容导出

- [ ] **Step 4: 运行 graph 相关测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_graph.py tests/test_order_refund_flow.py tests/test_handoff_flow.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/workflows/support agents/app/graph.py agents/app/state.py agents/app/agents/refund_hitl_flow.py agents/tests/test_graph.py agents/tests/test_order_refund_flow.py agents/tests/test_handoff_flow.py
git commit -m "refactor: isolate agents workflows into dedicated package"
```

---

### Task 4: 迁移 domain 层

**Files:**
- Create: `agents/app/domain/agents/__init__.py`
- Create: `agents/app/domain/messages/__init__.py`
- Create: `agents/app/domain/runs/__init__.py`
- Create: `agents/app/domain/threads/__init__.py`
- Modify: `agents/app/domain/agents/base.py`
- Modify: `agents/app/domain/agents/activity.py`
- Modify: `agents/app/domain/agents/coordinator.py`
- Modify: `agents/app/domain/agents/handoff.py`
- Modify: `agents/app/domain/agents/knowledge.py`
- Modify: `agents/app/domain/agents/order.py`
- Modify: `agents/app/domain/agents/refund.py`
- Modify: `agents/app/domain/agents/supervisor.py`
- Modify: `agents/app/domain/messages/models.py`
- Modify: `agents/app/domain/runs/models.py`
- Modify: `agents/app/domain/runs/event_models.py`
- Modify: `agents/app/domain/runs/tool_call_models.py`
- Modify: `agents/app/domain/runs/tool_call_contract.py`
- Modify: `agents/app/domain/threads/models.py`
- Modify: `agents/app/agents/__init__.py`
- Modify: `agents/app/messages/models.py`
- Modify: `agents/app/runs/models.py`
- Modify: `agents/app/runs/event_models.py`
- Modify: `agents/app/runs/tool_call_models.py`
- Modify: `agents/app/runs/tool_call_contract.py`
- Modify: `agents/app/threads/models.py`
- Test: `agents/tests/test_agents.py`
- Test: `agents/tests/test_coordinator_agent.py`
- Test: `agents/tests/test_supervisor_agent.py`
- Test: `agents/tests/test_knowledge_agent.py`

- [ ] **Step 1: 写 domain 模块迁移失败测试**

补充测试覆盖：

- domain 模型和 specialist 从新路径可导入
- 旧路径继续兼容导出
- specialist 与 graph node 分离

- [ ] **Step 2: 运行 domain 相关测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_agents.py tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_knowledge_agent.py -v
```

Expected:

- FAIL，失败点集中在导入路径与模块归属变化

- [ ] **Step 3: 迁移 domain 主实现并保留兼容层**

实施内容：

- specialist 正式迁到 `domain/agents`
- message/run/thread 模型迁到 `domain`
- 旧目录仅保留薄兼容文件或兼容导出

- [ ] **Step 4: 运行 domain 相关测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_agents.py tests/test_coordinator_agent.py tests/test_supervisor_agent.py tests/test_knowledge_agent.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/domain agents/app/agents/__init__.py agents/app/messages/models.py agents/app/runs/models.py agents/app/runs/event_models.py agents/app/runs/tool_call_models.py agents/app/runs/tool_call_contract.py agents/app/threads/models.py agents/tests/test_agents.py agents/tests/test_coordinator_agent.py agents/tests/test_supervisor_agent.py agents/tests/test_knowledge_agent.py
git commit -m "refactor: move agents domain models and specialists into domain layer"
```

---

### Task 5: 迁移 application 层

**Files:**
- Create: `agents/app/application/agent_runtime_service.py`
- Create: `agents/app/application/run_execution_service.py`
- Create: `agents/app/application/run_stream_service.py`
- Create: `agents/app/application/run_query_service.py`
- Create: `agents/app/application/thread_service.py`
- Create: `agents/app/application/message_service.py`
- Create: `agents/app/application/runtime_callbacks.py`
- Create: `agents/app/application/interrupt_bridge.py`
- Create: `agents/app/application/resume_command_executor.py`
- Create: `agents/app/application/run_event_bus.py`
- Create: `agents/app/application/run_event_projector.py`
- Modify: `agents/app/agent_runtime/service.py`
- Modify: `agents/app/agent_runtime/callbacks.py`
- Modify: `agents/app/messages/service.py`
- Modify: `agents/app/threads/service.py`
- Modify: `agents/app/runs/service.py`
- Modify: `agents/app/runs/executor.py`
- Modify: `agents/app/runs/stream_service.py`
- Modify: `agents/app/runs/event_bus.py`
- Modify: `agents/app/runs/event_projector.py`
- Modify: `agents/app/runs/interrupt_bridge.py`
- Modify: `agents/app/runs/resume_command_executor.py`
- Test: `agents/tests/test_run_executor.py`
- Test: `agents/tests/test_run_stream_service.py`
- Test: `agents/tests/test_thread_message_run_services.py`

- [ ] **Step 1: 写 application 层迁移失败测试**

补充测试覆盖：

- run 执行与 stream 投影都走 application 层
- 旧 service 模块仍兼容导出
- 用例层不直接持有 HTTP 或 MySQL 细节

- [ ] **Step 2: 运行 application 相关测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_run_executor.py tests/test_run_stream_service.py tests/test_thread_message_run_services.py -v
```

Expected:

- FAIL，失败点集中在导入路径与 service 位置变化

- [ ] **Step 3: 迁移 application 主实现**

实施内容：

- 将 runtime / executor / projector / stream 归位到 `application`
- 将 message / thread / run 用例服务归位到 `application`
- 旧目录改为兼容导出

- [ ] **Step 4: 运行 application 相关测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_run_executor.py tests/test_run_stream_service.py tests/test_thread_message_run_services.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/application agents/app/agent_runtime/service.py agents/app/agent_runtime/callbacks.py agents/app/messages/service.py agents/app/threads/service.py agents/app/runs/service.py agents/app/runs/executor.py agents/app/runs/stream_service.py agents/app/runs/event_bus.py agents/app/runs/event_projector.py agents/app/runs/interrupt_bridge.py agents/app/runs/resume_command_executor.py agents/tests/test_run_executor.py agents/tests/test_run_stream_service.py agents/tests/test_thread_message_run_services.py
git commit -m "refactor: centralize agents use cases into application layer"
```

---

### Task 6: 迁移 infrastructure 与 shared

**Files:**
- Create: `agents/app/infrastructure/llm/__init__.py`
- Create: `agents/app/infrastructure/mcp/__init__.py`
- Create: `agents/app/infrastructure/persistence/__init__.py`
- Create: `agents/app/infrastructure/persistence/mysql/__init__.py`
- Create: `agents/app/infrastructure/persistence/mysql/connection.py`
- Create: `agents/app/infrastructure/persistence/mysql/message_repository.py`
- Create: `agents/app/infrastructure/persistence/mysql/run_repository.py`
- Create: `agents/app/infrastructure/persistence/mysql/run_event_store.py`
- Create: `agents/app/infrastructure/persistence/mysql/thread_repository.py`
- Create: `agents/app/infrastructure/persistence/mysql/tool_call_repository.py`
- Create: `agents/app/infrastructure/persistence/redis/__init__.py`
- Create: `agents/app/infrastructure/persistence/redis/checkpointer.py`
- Create: `agents/app/infrastructure/knowledge/__init__.py`
- Create: `agents/app/shared/errors.py`
- Create: `agents/app/shared/ids.py`
- Create: `agents/app/shared/cursor.py`
- Create: `agents/app/shared/prompts.py`
- Create: `agents/app/shared/config.py`
- Modify: `agents/app/llm/client.py`
- Modify: `agents/app/llm/schemas.py`
- Modify: `agents/app/mcp_client/registry.py`
- Modify: `agents/app/mcp_client/interceptor.py`
- Modify: `agents/app/mcp_client/execution_context.py`
- Modify: `agents/app/session/checkpointer.py`
- Modify: `agents/app/knowledge/service.py`
- Modify: `agents/app/messages/repository.py`
- Modify: `agents/app/threads/repository.py`
- Modify: `agents/app/runs/repository.py`
- Modify: `agents/app/runs/tool_call_repository.py`
- Modify: `agents/app/runs/event_store.py`
- Modify: `agents/app/common/errors.py`
- Modify: `agents/app/common/ids.py`
- Modify: `agents/app/common/cursor.py`
- Modify: `agents/app/config.py`
- Modify: `agents/app/prompts.py`
- Test: `agents/tests/test_mcp_registry.py`
- Test: `agents/tests/test_mcp_interceptor.py`
- Test: `agents/tests/test_session_store.py`
- Test: `agents/tests/test_thread_message_run_repositories.py`

- [ ] **Step 1: 写 infrastructure/shared 迁移失败测试**

补充测试覆盖：

- repo 从新路径导入仍可工作
- LLM/MCP/checkpointer 从新路径可导入
- shared 工具模块位置变化不影响测试

- [ ] **Step 2: 运行 infra 相关测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_mcp_registry.py tests/test_mcp_interceptor.py tests/test_session_store.py tests/test_thread_message_run_repositories.py -v
```

Expected:

- FAIL，失败点集中在 repository、infra 与 shared 模块迁移

- [ ] **Step 3: 迁移 infrastructure 与 shared 主实现**

实施内容：

- 基础设施适配器全部迁到 `infrastructure`
- repository 实现迁到 `infrastructure/persistence`
- `common + config + prompts` 收敛到 `shared`
- 旧模块仅兼容导出

- [ ] **Step 4: 运行 infra 相关测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_mcp_registry.py tests/test_mcp_interceptor.py tests/test_session_store.py tests/test_thread_message_run_repositories.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/app/infrastructure agents/app/shared agents/app/llm/client.py agents/app/llm/schemas.py agents/app/mcp_client/registry.py agents/app/mcp_client/interceptor.py agents/app/mcp_client/execution_context.py agents/app/session/checkpointer.py agents/app/knowledge/service.py agents/app/messages/repository.py agents/app/threads/repository.py agents/app/runs/repository.py agents/app/runs/tool_call_repository.py agents/app/runs/event_store.py agents/app/common/errors.py agents/app/common/ids.py agents/app/common/cursor.py agents/app/config.py agents/app/prompts.py agents/tests/test_mcp_registry.py agents/tests/test_mcp_interceptor.py agents/tests/test_session_store.py agents/tests/test_thread_message_run_repositories.py
git commit -m "refactor: move agents adapters and repositories into infrastructure layer"
```

---

### Task 7: 清理兼容层、README 与全量回归

**Files:**
- Modify: `agents/README.md`
- Modify: `docs/architecture` 下相关 agents 文档
- Modify: `agents/tests/test_docs.py`
- Modify: `agents/tests/test_smoke.py`
- Modify: `agents/tests/test_api.py`
- Modify: `agents/tests/test_graph.py`
- Modify: `agents/tests/test_run_executor.py`
- Modify: `agents/tests/test_run_stream_service.py`
- Modify: `agents/tests/test_thread_message_run_repositories.py`
- Modify: `agents/tests/test_thread_message_run_services.py`

- [ ] **Step 1: 写 README 与全量入口失败测试**

补充测试覆盖：

- README 与实际目录一致
- 关键旧路径仍兼容，或在 README 中明确兼容期策略
- 全量核心测试集通过

- [ ] **Step 2: 运行回归测试确认失败**

Run:

```bash
cd agents && uv run pytest tests/test_docs.py tests/test_smoke.py tests/test_api.py tests/test_graph.py tests/test_run_executor.py tests/test_run_stream_service.py tests/test_thread_message_run_repositories.py tests/test_thread_message_run_services.py -v
```

Expected:

- FAIL，失败点集中在文档与入口描述未同步

- [ ] **Step 3: 更新 README 与兼容策略说明**

实施内容：

- 写明新目录职责
- 写明兼容入口列表
- 写明后续可删除的旧路径

- [ ] **Step 4: 运行回归测试确认通过**

Run:

```bash
cd agents && uv run pytest tests/test_docs.py tests/test_smoke.py tests/test_api.py tests/test_graph.py tests/test_run_executor.py tests/test_run_stream_service.py tests/test_thread_message_run_repositories.py tests/test_thread_message_run_services.py -v
```

Expected:

- PASS

- [ ] **Step 5: Commit**

```bash
git add agents/README.md docs/architecture agents/tests/test_docs.py agents/tests/test_smoke.py agents/tests/test_api.py agents/tests/test_graph.py agents/tests/test_run_executor.py agents/tests/test_run_stream_service.py agents/tests/test_thread_message_run_repositories.py agents/tests/test_thread_message_run_services.py docs/superpowers/plans/2026-04-17-agents-project-file-architecture-refactor-v2.md
git commit -m "docs: document agents project architecture v2"
```

---

## 执行顺序建议

1. 先建骨架和兼容入口，避免大面积 import 爆炸。
2. 再迁 `transport + bootstrap`，把入口层先稳住。
3. 再迁 `workflows`，把 graph 文件结构从全项目中剥离。
4. 再迁 `domain`，明确业务专员与领域模型归属。
5. 再迁 `application`，把执行编排和用例协调收口。
6. 最后迁 `infrastructure + shared`，完成适配器归位。
7. 结尾统一更新 README 和兼容清理策略。

## 对旧 plan 的处理建议

- `docs/superpowers/plans/2026-04-17-agents-langgraph-architecture-refactor.md` 不再作为主计划执行。
- 其中关于 `graph / nodes / subgraphs` 的拆分思路可以作为本计划 Task 3 的子参考。
- 不要继续在旧 plan 上追加任务，避免出现“双主线计划”。

## 完成标准

- `agents/app` 顶层目录职责清晰，稳定收敛到 `bootstrap / transport / application / workflows / domain / infrastructure / shared`
- `graph` 不再与 runtime / API / repository 混放
- `domain` 与 `infrastructure` 不再交叉混放
- `routes.py` 不再承担装配职责
- `repository` 不再与 domain model、application service 混在同目录
- README 与测试入口对齐新结构
- 旧路径仅保留必要兼容层，并在文档中写明后续删除计划
