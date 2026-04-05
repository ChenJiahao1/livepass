import pytest

from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.tasking.task_card import TaskCard
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy


class FakeToolRegistry:
    def __init__(self):
        self.calls: list[tuple[str, str, dict]] = []
        self.get_tools_calls: list[str] = []

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict):
        self.calls.append((server_name, tool_name, payload))
        return {"ok": True, "payload": payload}

    async def get_provider_tools(self, server_name: str) -> list:
        self.get_tools_calls.append(server_name)
        return [
            type("Tool", (), {"name": "list_user_orders"})(),
            type("Tool", (), {"name": "preview_refund_order"})(),
            type("Tool", (), {"name": "refund_order"})(),
        ]


def test_tool_broker_blocks_tool_outside_skill_whitelist():
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    broker = ToolBroker(
        registry=FakeToolRegistry(),
        skill_registry=skill_registry,
        provider_registry=provider_registry,
        policy=ToolPolicy.from_skill_registry(skill_registry),
    )

    with pytest.raises(PermissionError):
        broker.assert_allowed("refund.preview", "refund_order")


@pytest.mark.anyio
async def test_tool_broker_injects_task_context_and_calls_provider():
    registry = FakeToolRegistry()
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    broker = ToolBroker(
        registry=registry,
        skill_registry=skill_registry,
        provider_registry=provider_registry,
        policy=ToolPolicy.from_skill_registry(skill_registry),
    )
    task = TaskCard(
        task_id="task-001",
        session_id="sess-001",
        domain="refund",
        task_type="refund_preview",
        skill_id="refund.preview",
        goal="确认订单是否可退款",
        input_slots={"user_id": 3001, "order_id": "ORD-10001"},
        required_slots=["order_id"],
        risk_level="medium",
        fallback_policy="handoff",
        expected_output_schema="refund_preview_v1",
    )

    result = await broker.call(
        task=task,
        skill_id="refund.preview",
        tool_name="preview_refund_order",
        payload={"order_id": "ORD-10001"},
    )

    assert result["ok"] is True
    assert registry.calls == [
        (
            "order",
            "preview_refund_order",
            {
                "order_id": "ORD-10001",
                "user_id": 3001,
                "session_id": "sess-001",
                "task_id": "task-001",
            },
        )
    ]


@pytest.mark.anyio
async def test_tool_broker_returns_only_tools_visible_to_current_skill():
    registry = FakeToolRegistry()
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    broker = ToolBroker(
        registry=registry,
        skill_registry=skill_registry,
        provider_registry=provider_registry,
        policy=ToolPolicy.from_skill_registry(skill_registry),
    )

    tools = await broker.get_skill_tools("refund.preview")

    assert [tool.name for tool in tools] == ["preview_refund_order"]
    assert registry.get_tools_calls == ["order"]
