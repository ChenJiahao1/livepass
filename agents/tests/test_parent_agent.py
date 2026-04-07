import pytest

from app.orchestrator.parent_agent import ParentAgent
from tests.fakes import ScriptedChatModel


class StubRuntime:
    def __init__(self, results: list[dict] | None = None) -> None:
        self.results = list(results or [])
        self.calls: list[dict] = []

    async def execute(self, *, task, resolution, session_state, llm):
        self.calls.append(
            {
                "task_id": task.task_id,
                "task_type": task.task_type,
                "allowed_skills": list(task.allowed_skills),
                "requires_confirmation": task.requires_confirmation,
                "session_state": dict(session_state),
                "llm": llm,
            }
        )
        if not self.results:
            raise AssertionError("unexpected runtime execute call")
        return self.results.pop(0)


@pytest.mark.anyio
async def test_parent_agent_uses_llm_to_plan_refund_read_flow():
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "task_type": "refund_read"},
            {"action": "reply", "reply": "订单 ORD-10002 当前可退款，预计退款 99.00。是否确认退款？"},
        ]
    )
    runtime = StubRuntime(
        results=[
            {
                "task_id": "task-read",
                "task_type": "refund_read",
                "skill_id": "refund.read",
                "skill_ids": ["refund.read"],
                "tool_calls": ["list_user_orders", "preview_refund_order"],
                "output": {
                    "order_id": "ORD-10002",
                    "allow_refund": True,
                    "refund_amount": "99.00",
                    "refund_percent": 100,
                    "reject_reason": "",
                },
                "summary": "退款资格已确认",
                "need_handoff": False,
                "selected_order_id": "ORD-10002",
                "recent_order_candidates": [
                    {"order_id": "ORD-10002", "create_order_time": "2026-04-05 09:00:00"},
                    {"order_id": "ORD-10001", "create_order_time": "2026-04-05 08:00:00"},
                ],
                "last_refund_preview": {
                    "order_id": "ORD-10002",
                    "allow_refund": True,
                    "refund_amount": "99.00",
                    "reject_reason": "",
                },
            },
        ]
    )
    agent = ParentAgent(runtime=runtime)

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": llm, "session_state": {"user_id": 3001}},
    )

    assert [item["task_type"] for item in result["task_trace"]] == ["refund_read"]
    assert result["reply"] == "订单 ORD-10002 当前可退款，预计退款 99.00。是否确认退款？"
    assert result["selected_order_id"] == "ORD-10002"
    assert runtime.calls[0]["task_type"] == "refund_read"
    assert runtime.calls[0]["allowed_skills"] == ["refund.read"]
    assert result["session_state"]["last_refund_preview"]["refund_amount"] == "99.00"


@pytest.mark.anyio
async def test_parent_agent_can_clarify_without_calling_runtime():
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "clarify", "reply": "请先告诉我具体订单号，或说明是最近一单。"},
        ]
    )
    runtime = StubRuntime()
    agent = ParentAgent(runtime=runtime)

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我处理一下订单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": llm, "session_state": {"user_id": 3001}},
    )

    assert result["reply"] == "请先告诉我具体订单号，或说明是最近一单。"
    assert result["task_trace"] == []
    assert runtime.calls == []


@pytest.mark.anyio
async def test_parent_agent_only_allows_refund_write_after_confirmed_preview():
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "task_type": "refund_write"},
            {"action": "reply", "reply": "订单 ORD-10002 已提交退款。"},
        ]
    )
    runtime = StubRuntime(
        results=[
            {
                "task_id": "task-write",
                "task_type": "refund_write",
                "skill_id": "refund.write",
                "skill_ids": ["refund.write"],
                "tool_calls": ["refund_order"],
                "output": {"order_id": "ORD-10002", "accepted": True},
                "summary": "退款申请已提交",
                "need_handoff": False,
                "selected_order_id": "ORD-10002",
            }
        ]
    )
    agent = ParentAgent(runtime=runtime)

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "确认退款"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={
            "current_user_id": "3001",
            "llm": llm,
            "session_state": {
                "user_id": 3001,
                "selected_order_id": "ORD-10002",
                "last_refund_preview": {
                    "order_id": "ORD-10002",
                    "allow_refund": True,
                    "refund_amount": "99.00",
                    "reject_reason": "",
                },
            },
        },
    )

    assert result["reply"] == "订单 ORD-10002 已提交退款。"
    assert runtime.calls[0]["task_type"] == "refund_write"
    assert runtime.calls[0]["allowed_skills"] == ["refund.write"]
    assert runtime.calls[0]["requires_confirmation"] is True


@pytest.mark.anyio
async def test_parent_agent_rejects_refund_write_without_preview_state():
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "task_type": "refund_write"},
        ]
    )
    runtime = StubRuntime()
    agent = ParentAgent(runtime=runtime)

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "确认退款"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": llm, "session_state": {"user_id": 3001}},
    )

    assert "先确认订单退款资格" in result["reply"]
    assert result["task_trace"] == []
    assert runtime.calls == []


@pytest.mark.anyio
async def test_parent_agent_requires_llm():
    agent = ParentAgent(runtime=StubRuntime())

    with pytest.raises(ValueError, match="llm is required"):
        await agent.ainvoke(
            {"messages": [{"role": "user", "content": "帮我查订单"}]},
            config={"configurable": {"thread_id": "sess-001"}},
            context={"current_user_id": "3001", "llm": None, "session_state": {"user_id": 3001}},
        )
