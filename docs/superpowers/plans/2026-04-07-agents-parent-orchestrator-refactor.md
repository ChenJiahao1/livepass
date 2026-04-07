# Agents Parent Orchestrator Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `agents` 收敛为“LLM 驱动的 ParentAgent + 受控 skill subagent”架构，移除旧兼容层和无模型 fallback。

**Architecture:** `POST /agent/chat` 只走 `ParentAgent -> TaskCard -> SkillResolver -> SubagentRuntime -> ToolBroker -> MCP Provider` 主链。ParentAgent 负责 LLM 决策、澄清、直接回复和多步任务编排；SubagentRuntime 只执行单一 skill；无模型时 API 直接报错，不再走规则兜底。

**Tech Stack:** Python, FastAPI, LangChain, Pydantic, MCP, pytest

---

### Task 1: 固化 ParentAgent 的 LLM 决策协议

**Files:**
- Create: `agents/tests/test_parent_agent.py`
- Modify: `agents/app/orchestrator/parent_agent.py`

- [ ] **Step 1: 写出 ParentAgent 决策的失败测试**
- [ ] **Step 2: 跑 `uv run pytest tests/test_parent_agent.py -q`，确认失败**
- [ ] **Step 3: 实现 `ParentDecision`、LLM 决策调用和多步编排最小闭环**
- [ ] **Step 4: 再跑 `uv run pytest tests/test_parent_agent.py -q`，确认通过**

### Task 2: 移除无模型 fallback，收紧 API 依赖

**Files:**
- Modify: `agents/app/api/routes.py`
- Modify: `agents/app/runtime/subagent_runtime.py`
- Modify: `agents/tests/test_api.py`
- Modify: `agents/tests/test_e2e_contract.py`
- Delete: `agents/tests/test_no_llm_fallback.py`

- [ ] **Step 1: 先补“未配置模型直接报错”的失败测试**
- [ ] **Step 2: 跑 `uv run pytest tests/test_api.py tests/test_e2e_contract.py -q`，确认失败**
- [ ] **Step 3: 实现 `get_llm()` 强依赖配置、`SubagentRuntime` 拒绝空 LLM**
- [ ] **Step 4: 再跑 `uv run pytest tests/test_api.py tests/test_e2e_contract.py -q`，确认通过**

### Task 3: 删除旧兼容层和重复目录

**Files:**
- Delete: `agents/app/agents/`
- Delete: `agents/app/state.py`
- Delete: `agents/app/clients/`
- Delete: `agents/app/runtime/react_loop.py`
- Delete: `agents/app/runtime/execution_context.py`
- Modify: `agents/tests/test_smoke.py`
- Delete: `agents/tests/test_agents.py`

- [ ] **Step 1: 先补导入面失败测试或更新 smoke 断言**
- [ ] **Step 2: 删除旧兼容层与重复目录**
- [ ] **Step 3: 跑 `uv run pytest tests/test_smoke.py -q`，确认通过**

### Task 4: 收敛知识问答为父层能力模块

**Files:**
- Create: `agents/app/knowledge/__init__.py`
- Create: `agents/app/knowledge/service.py`
- Modify: `agents/app/orchestrator/parent_agent.py`
- Modify: `agents/tests/test_knowledge_agent.py`

- [ ] **Step 1: 先让知识能力测试指向新模块并确认失败**
- [ ] **Step 2: 迁移 LightRAG 逻辑到新模块，并接入 ParentAgent 决策分支**
- [ ] **Step 3: 跑 `uv run pytest tests/test_knowledge_agent.py tests/test_parent_agent.py -q`，确认通过**

### Task 5: 回归 MCP/runtime 主链并更新文档

**Files:**
- Modify: `agents/README.md`
- Modify: `agents/tests/test_docs.py`
- Modify: `agents/tests/test_order_refund_flow.py`
- Modify: `agents/tests/test_handoff_flow.py`
- Modify: `agents/tests/test_subagent_runtime.py`

- [ ] **Step 1: 更新测试以匹配“LLM ParentAgent + 单 skill runtime”语义**
- [ ] **Step 2: 运行 `uv run pytest tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_subagent_runtime.py tests/test_docs.py -q`**
- [ ] **Step 3: 更新 README，明确新主链与删除项**

### Task 6: 执行全量回归

**Files:**
- Verify only

- [ ] **Step 1: 运行 `uv run pytest tests/test_parent_agent.py tests/test_api.py tests/test_e2e_contract.py tests/test_order_refund_flow.py tests/test_handoff_flow.py tests/test_knowledge_agent.py tests/test_mcp_registry.py tests/test_go_provider_registry.py tests/test_skill_registry.py tests/test_tool_broker.py tests/test_subagent_runtime.py tests/test_smoke.py -q`**
- [ ] **Step 2: 记录残余风险和未纳入本次改造的模块**
