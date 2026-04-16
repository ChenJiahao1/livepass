# Run Hardening Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 agents run-first 重构中的异常收口、resume 归属校验与同线程并发 active run 防护问题。

**Architecture:** 在 `RunService` 收口线程级 active run 约束，在 `RunExecutor` 统一把可预期业务错误转为 `ApiError`、把后台执行异常投影为失败终态，并补上对应事件/消息终态。测试先覆盖 API、service 与 executor 层，再做最小实现。

**Tech Stack:** FastAPI、pytest、in-memory repository、MySQL-style repository abstraction

---

### Task 1: 限制同线程并发 active run

**Files:**
- Modify: `agents/app/runs/service.py`
- Test: `agents/tests/test_thread_message_run_services.py`

- [ ] **Step 1: 写失败用例**
  - 断言同一 `thread_id` 在已有 `queued/running/requires_action` run 时，再次 `create_run` 返回 `RUN_STATE_INVALID`。
- [ ] **Step 2: 跑单测确认失败**
  - Run: `uv run pytest agents/tests/test_thread_message_run_services.py -k active_run -v`
- [ ] **Step 3: 最小实现**
  - 在 `RunService.create_run()` 创建前调用 `run_repository.find_active_by_thread()` 并校验归属，命中时抛 `ApiError`。
- [ ] **Step 4: 跑单测确认通过**
  - Run: `uv run pytest agents/tests/test_thread_message_run_services.py -k active_run -v`

### Task 2: 修正 resume 归属校验与状态错误

**Files:**
- Modify: `agents/app/runs/executor.py`
- Test: `agents/tests/test_run_executor.py`
- Test: `agents/tests/test_run_resume_cancel_api.py`

- [ ] **Step 1: 写失败用例**
  - 断言 `tool_call.run_id != run.id` 时 `resume()` 抛契约化错误。
  - 断言不可恢复/不可取消状态时 API 返回 4xx，而不是 500。
- [ ] **Step 2: 跑单测确认失败**
  - Run: `uv run pytest agents/tests/test_run_executor.py agents/tests/test_run_resume_cancel_api.py -k "resume or cancel" -v`
- [ ] **Step 3: 最小实现**
  - 在 `RunExecutor.resume()` 校验 `tool_call.run_id/user_id/thread_id`。
  - 把 `ValueError` 改为 `ApiError`，维持路由层统一错误映射。
- [ ] **Step 4: 跑单测确认通过**
  - Run: `uv run pytest agents/tests/test_run_executor.py agents/tests/test_run_resume_cancel_api.py -k "resume or cancel" -v`

### Task 3: 收口后台执行失败为 run 终态

**Files:**
- Modify: `agents/app/runs/executor.py`
- Modify: `agents/app/runs/event_projector.py`
- Modify: `agents/app/runs/event_models.py`
- Test: `agents/tests/test_run_executor.py`
- Test: `agents/tests/test_run_contract_api.py`

- [ ] **Step 1: 写失败用例**
  - 断言 runtime 抛异常后，run 进入 `failed`，assistant message 不再停留在 `in_progress`，并产生 `run_failed` 终态事件。
- [ ] **Step 2: 跑单测确认失败**
  - Run: `uv run pytest agents/tests/test_run_executor.py agents/tests/test_run_contract_api.py -k failed -v`
- [ ] **Step 3: 最小实现**
  - 在 `RunExecutor.start()/resume()` 用 `try/except` 收口后台异常。
  - 将 message 更新为终态，并写入 `run_failed` 事件与 `mark_failed()`。
- [ ] **Step 4: 跑单测确认通过**
  - Run: `uv run pytest agents/tests/test_run_executor.py agents/tests/test_run_contract_api.py -k failed -v`

### Task 4: 回归关键契约

**Files:**
- Test: `agents/tests/test_api.py`
- Test: `agents/tests/test_run_contract_api.py`
- Test: `agents/tests/test_run_executor.py`
- Test: `agents/tests/test_run_resume_cancel_api.py`
- Test: `agents/tests/test_thread_message_run_services.py`

- [ ] **Step 1: 跑定向回归**
  - Run: `uv run pytest agents/tests/test_api.py agents/tests/test_run_contract_api.py agents/tests/test_run_executor.py agents/tests/test_run_resume_cancel_api.py agents/tests/test_thread_message_run_services.py -v`
- [ ] **Step 2: 如有失败，最小修正后重跑**
- [ ] **Step 3: 记录最终结果**
