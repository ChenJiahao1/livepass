import pytest
from langchain_core.messages import AIMessage, HumanMessage

from app.agents.specialists.order_specialist import OrderAgent
from tests.fakes import StubRegistry, build_async_tool


@pytest.mark.anyio
async def test_base_specialist_agent_run_tool_agent_wires_tools_prompt_and_messages(monkeypatch):
    captured: dict[str, object] = {}

    class _FakeCreatedAgent:
        async def ainvoke(self, payload):
            captured["payload"] = payload
            return {"messages": [AIMessage(content="已处理")]}

    def _fake_create_agent(*, model, tools, system_prompt, name):
        captured["model"] = model
        captured["tool_names"] = [tool.name for tool in tools]
        captured["system_prompt"] = system_prompt
        captured["name"] = name
        return _FakeCreatedAgent()

    registry = StubRegistry(
        tools_by_toolset={
            "order": [
                build_async_tool(name="list_user_orders", description="list user orders", coroutine=lambda *, user_id: None),
                build_async_tool(
                    name="get_order_detail_for_service",
                    description="get order detail",
                    coroutine=lambda *, order_id, user_id=None: None,
                ),
                build_async_tool(
                    name="preview_refund_order",
                    description="preview refund",
                    coroutine=lambda *, order_id, user_id=None: None,
                ),
                build_async_tool(
                    name="refund_order",
                    description="submit refund",
                    coroutine=lambda *, order_id, reason, user_id=None: None,
                ),
            ]
        }
    )
    state = {"messages": [HumanMessage(content="帮我看看订单处理情况")]}
    llm = object()

    monkeypatch.setattr("app.agents.base.create_agent", _fake_create_agent)

    result = await OrderAgent(registry=registry, llm=llm).handle(state)

    assert result["reply"] == "已处理"
    assert captured["model"] is llm
    assert captured["name"] == "order"
    assert captured["payload"] == {"messages": state["messages"]}
    assert captured["tool_names"] == [
        "list_user_orders",
        "get_order_detail_for_service",
        "preview_refund_order",
        "refund_order",
    ]
    assert "selected_order_id" in str(captured["system_prompt"])


@pytest.mark.anyio
async def test_order_agent_uses_generic_tool_agent_flow_without_hardcoded_refund_branch(monkeypatch):
    calls: list[str] = []

    async def _fake_run_tool_agent(self, state):
        calls.append("run_tool_agent")
        return {"reply": "已处理", "final_reply": "已处理", "messages": [AIMessage(content="已处理")]}

    async def _preview_refund_order(order_id: str, user_id: int | None = None):
        return {"order_id": order_id, "allow_refund": True, "refund_amount": "100"}

    async def _refund_order(order_id: str, reason: str, user_id: int | None = None):
        return {"order_id": order_id, "accepted": True, "refund_amount": "100"}

    monkeypatch.setattr(OrderAgent, "run_tool_agent", _fake_run_tool_agent)
    registry = StubRegistry(
        tools_by_toolset={
            "order": [
                build_async_tool(
                    name="preview_refund_order",
                    description="preview refund",
                    coroutine=_preview_refund_order,
                ),
                build_async_tool(
                    name="refund_order",
                    description="submit refund",
                    coroutine=_refund_order,
                ),
            ]
        }
    )

    result = await OrderAgent(registry=registry, llm=object()).handle(
        {"messages": [HumanMessage(content="帮我退款 ORD-1")], "selected_order_id": "ORD-1", "current_user_id": 1001}
    )

    assert calls == ["run_tool_agent"]
    assert result["reply"] == "已处理"
