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
                "skill_id": task.skill_id,
                "session_state": dict(session_state),
                "llm": llm,
            }
        )
        if not self.results:
            raise AssertionError("unexpected runtime execute call")
        return self.results.pop(0)


@pytest.mark.anyio
async def test_parent_agent_uses_llm_to_plan_multi_step_refund_flow():
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "task_type": "order_list_recent"},
            {"action": "delegate", "task_type": "refund_preview"},
            {"action": "reply", "reply": "订单 ORD-10002 当前可退款，预计退款 99.00。"},
        ]
    )
    runtime = StubRuntime(
        results=[
            {
                "task_id": "task-order",
                "task_type": "order_list_recent",
                "skill_id": "order.list_recent",
                "tool_calls": ["list_user_orders"],
                "output": {
                    "orders": [
                        {"order_id": "ORD-10002", "create_order_time": "2026-04-05 09:00:00"},
                        {"order_id": "ORD-10001", "create_order_time": "2026-04-05 08:00:00"},
                    ]
                },
                "summary": "已获取最近订单列表",
                "need_handoff": False,
                "selected_order_id": "ORD-10002",
                "recent_order_candidates": [
                    {"order_id": "ORD-10002", "create_order_time": "2026-04-05 09:00:00"},
                    {"order_id": "ORD-10001", "create_order_time": "2026-04-05 08:00:00"},
                ],
            },
            {
                "task_id": "task-refund",
                "task_type": "refund_preview",
                "skill_id": "refund.preview",
                "tool_calls": ["preview_refund_order"],
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
            },
        ]
    )
    agent = ParentAgent(runtime=runtime)

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": llm, "session_state": {"user_id": 3001}},
    )

    assert [item["task_type"] for item in result["task_trace"]] == ["order_list_recent", "refund_preview"]
    assert result["reply"] == "订单 ORD-10002 当前可退款，预计退款 99.00。"
    assert result["selected_order_id"] == "ORD-10002"
    assert runtime.calls[0]["task_type"] == "order_list_recent"
    assert runtime.calls[1]["task_type"] == "refund_preview"
    assert runtime.calls[1]["session_state"]["selected_order_id"] == "ORD-10002"


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
async def test_parent_agent_requires_llm():
    agent = ParentAgent(runtime=StubRuntime())

    with pytest.raises(ValueError, match="llm is required"):
        await agent.ainvoke(
            {"messages": [{"role": "user", "content": "帮我查订单"}]},
            config={"configurable": {"thread_id": "sess-001"}},
            context={"current_user_id": "3001", "llm": None, "session_state": {"user_id": 3001}},
        )
