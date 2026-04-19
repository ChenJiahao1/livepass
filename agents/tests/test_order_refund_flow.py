import pytest
from langchain_core.messages import AIMessage
from langgraph.checkpoint.memory import InMemorySaver

from app.graph.builder import build_graph_app
from tests.fakes import ScriptedChatModel, StubRegistry, build_async_tool


async def run_graph_turns(*, messages: list[str], registry, llm) -> dict:
    app = build_graph_app(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": "conv-graph-flow"}}
    result = {}
    for message in messages:
        result = await app.ainvoke(
            {"messages": [{"role": "user", "content": message}]},
            config=config,
            context={"llm": llm, "registry": registry, "current_user_id": 1001},
        )
    return result


@pytest.mark.anyio
async def test_order_lane_can_complete_with_tool_agent_reply(monkeypatch):
    captured: dict[str, object] = {}

    class _FakeCreatedAgent:
        async def ainvoke(self, payload):
            captured["payload"] = payload
            return {"messages": [AIMessage(content="已通过通用 agent 处理")]}

    def _fake_create_agent(*, model, tools, system_prompt, name):
        captured["tool_names"] = [tool.name for tool in tools]
        captured["system_prompt"] = system_prompt
        captured["name"] = name
        return _FakeCreatedAgent()

    async def _list_user_orders(user_id: int):
        return {"orders": [{"order_id": "ORD-1", "status": "PAID"}]}

    async def _get_order_detail_for_service(order_id: str, user_id: int | None = None):
        return {"order_id": order_id, "status": "PAID"}

    async def _preview_refund_order(order_id: str, user_id: int | None = None):
        return {"order_id": order_id, "allow_refund": True, "refund_amount": "100", "refund_percent": 100}

    async def _refund_order(order_id: str, reason: str, user_id: int | None = None):
        return {"order_id": order_id, "accepted": True, "refund_amount": "100"}

    monkeypatch.setattr("app.agents.base.create_agent", _fake_create_agent)
    registry = StubRegistry(
        tools_by_toolset={
            "order": [
                build_async_tool(
                    name="list_user_orders",
                    description="list user orders",
                    coroutine=_list_user_orders,
                ),
                build_async_tool(
                    name="get_order_detail_for_service",
                    description="get order detail",
                    coroutine=_get_order_detail_for_service,
                ),
                build_async_tool(
                    name="preview_refund_order",
                    description="preview refund",
                    coroutine=_preview_refund_order,
                ),
                build_async_tool(
                    name="refund_order",
                    description="submit refund",
                    coroutine=_refund_order,
                )
            ]
        }
    )
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "reply": "", "selected_order_id": "ORD-1", "business_ready": True, "reason": "preview"},
            {"next_agent": "order", "selected_order_id": "ORD-1", "reason": "refund via order"},
        ]
    )

    result = await run_graph_turns(messages=["ORD-1 可以退款吗"], registry=registry, llm=llm)

    assert result["final_reply"] == "已通过通用 agent 处理"
    assert captured["name"] == "order"
    assert captured["tool_names"] == [
        "list_user_orders",
        "get_order_detail_for_service",
        "preview_refund_order",
        "refund_order",
    ]
    assert "current_user_id" in str(captured["system_prompt"])
