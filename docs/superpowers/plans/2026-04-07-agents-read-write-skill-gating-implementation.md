# Agents Read/Write Skill Gating Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `agents` 从“父层指定单一步骤 skill”改为“父层控制读写能力包，subagent 自主执行读链路，写链路必须确认后才能启用”。

**Architecture:** `TaskCard` 改为声明 `allowed_skills` 与 `requires_confirmation`。`ParentAgent` 只负责判断当前是读任务还是写任务；`SubagentRuntime` 绑定授权 skill 的工具并集；`ToolBroker` 对写 skill 做确认态硬校验，会话层保留退款预览结果以支撑写门禁。

**Tech Stack:** Python, FastAPI, LangChain, Pydantic, pytest

---

### Task 1: 更新 TaskCard 与 skill 元数据模型

**Files:**
- Modify: `agents/app/tasking/task_card.py`
- Modify: `agents/app/registry/skill_registry.py`
- Modify: `agents/app/skills/registry.yaml`
- Test: `agents/tests/test_task_card.py`

- [ ] **Step 1: 先写 `TaskCard` 与 skill 元数据的新失败测试**
- [ ] **Step 2: 运行 `uv run pytest tests/test_task_card.py -q`，确认失败**
- [ ] **Step 3: 实现 `allowed_skills`、`requires_confirmation`、`access_mode`**
- [ ] **Step 4: 再次运行 `uv run pytest tests/test_task_card.py -q`，确认通过**

### Task 2: 让 ToolBroker 支持读写 gating

**Files:**
- Modify: `agents/app/tools/broker.py`
- Modify: `agents/app/tools/policies.py`
- Test: `agents/tests/test_tool_broker.py`

- [ ] **Step 1: 先补“多 skill 工具可见性”和“未确认阻断写工具”的失败测试**
- [ ] **Step 2: 运行 `uv run pytest tests/test_tool_broker.py -q`，确认失败**
- [ ] **Step 3: 实现多 skill 工具聚合与写门禁**
- [ ] **Step 4: 再次运行 `uv run pytest tests/test_tool_broker.py -q`，确认通过**

### Task 3: 让 SubagentRuntime 支持能力包执行

**Files:**
- Modify: `agents/app/runtime/subagent_runtime.py`
- Modify: `agents/app/orchestrator/skill_resolver.py`
- Test: `agents/tests/test_subagent_runtime.py`

- [ ] **Step 1: 先写“多 skill 工具并集绑定”的失败测试**
- [ ] **Step 2: 运行 `uv run pytest tests/test_subagent_runtime.py -q`，确认失败**
- [ ] **Step 3: 实现 runtime 基于 `allowed_skills` 绑定工具**
- [ ] **Step 4: 再次运行 `uv run pytest tests/test_subagent_runtime.py -q`，确认通过**

### Task 4: 调整 ParentAgent 与会话状态

**Files:**
- Modify: `agents/app/orchestrator/parent_agent.py`
- Modify: `agents/app/session/store.py`
- Modify: `agents/app/api/routes.py`
- Test: `agents/tests/test_parent_agent.py`
- Test: `agents/tests/test_session_store.py`

- [ ] **Step 1: 先写“退款咨询生成读任务”和“确认退款生成写任务”的失败测试**
- [ ] **Step 2: 运行 `uv run pytest tests/test_parent_agent.py tests/test_session_store.py -q`，确认失败**
- [ ] **Step 3: 实现父层读写切换与 `last_refund_preview` 会话状态**
- [ ] **Step 4: 再次运行 `uv run pytest tests/test_parent_agent.py tests/test_session_store.py -q`，确认通过**

### Task 5: 回归退款主链路

**Files:**
- Modify: `agents/tests/test_order_refund_flow.py`
- Modify: `agents/README.md`

- [ ] **Step 1: 先改退款主链路测试，体现“读链路自主、写链路确认闸”**
- [ ] **Step 2: 运行 `uv run pytest tests/test_order_refund_flow.py -q`，确认失败**
- [ ] **Step 3: 完成最小实现补齐后再次运行 `uv run pytest tests/test_order_refund_flow.py -q`，确认通过**
- [ ] **Step 4: 更新 README，说明 skill 改为 read/write bundle**

### Task 6: 执行回归

**Files:**
- Verify only

- [ ] **Step 1: 运行 `uv run pytest tests/test_task_card.py tests/test_tool_broker.py tests/test_subagent_runtime.py tests/test_parent_agent.py tests/test_session_store.py tests/test_order_refund_flow.py -q`**
- [ ] **Step 2: 记录残余风险：FAQ 尚未接入、Go provider 未扩展新字段时的兼容约束**
