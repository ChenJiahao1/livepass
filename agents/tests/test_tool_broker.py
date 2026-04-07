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
            type("Tool", (), {"name": "get_order_detail_for_service"})(),
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
        broker.assert_allowed("refund.read", "refund_order")


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
        task_type="refund_read",
        goal="处理退款咨询并确认退款资格",
        input_slots={"user_id": 3001, "order_id": "ORD-10001"},
        required_slots=["order_id"],
        allowed_skills=["refund.read"],
        risk_level="medium",
        requires_confirmation=False,
        fallback_policy="handoff",
        expected_output_schema="refund_read_result_v1",
    )

    result = await broker.call(
        task=task,
        skill_id="refund.read",
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

    task = TaskCard(
        task_id="task-002",
        session_id="sess-002",
        domain="refund",
        task_type="refund_read",
        goal="处理退款咨询并确认退款资格",
        input_slots={"user_id": 3001},
        required_slots=[],
        allowed_skills=["order.read", "refund.read"],
        risk_level="medium",
        requires_confirmation=False,
        fallback_policy="return_parent",
        expected_output_schema="refund_read_result_v1",
    )

    tools = await broker.get_task_tools(task)

    assert [tool.name for tool in tools] == [
        "list_user_orders",
        "get_order_detail_for_service",
        "preview_refund_order",
    ]
    assert registry.get_tools_calls == ["order"]


@pytest.mark.anyio
async def test_tool_broker_blocks_write_tool_without_confirmation():
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
        task_id="task-003",
        session_id="sess-003",
        domain="refund",
        task_type="refund_write",
        goal="提交退款申请",
        input_slots={"user_id": 3001, "order_id": "ORD-10001"},
        required_slots=["order_id"],
        allowed_skills=["refund.write"],
        risk_level="high",
        requires_confirmation=False,
        fallback_policy="handoff",
        expected_output_schema="refund_submit_v1",
    )

    with pytest.raises(PermissionError, match="confirmation"):
        await broker.call(
            task=task,
            skill_id="refund.write",
            tool_name="refund_order",
            payload={"order_id": "ORD-10001"},
        )


@pytest.mark.anyio
async def test_tool_broker_allows_write_tool_after_confirmation():
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
        task_id="task-004",
        session_id="sess-004",
        domain="refund",
        task_type="refund_write",
        goal="提交退款申请",
        input_slots={"user_id": 3001, "order_id": "ORD-10001"},
        required_slots=["order_id"],
        allowed_skills=["refund.write"],
        risk_level="high",
        requires_confirmation=True,
        fallback_policy="handoff",
        expected_output_schema="refund_submit_v1",
    )

    result = await broker.call(
        task=task,
        skill_id="refund.write",
        tool_name="refund_order",
        payload={"order_id": "ORD-10001"},
    )

    assert result["ok"] is True
