import pytest
from langchain_core.messages import AIMessage

from app.orchestrator.skill_resolver import SkillResolution
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.tasking.task_card import TaskCard
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy
from tests.fakes import ScriptedChatModel, make_tool_call_message


class FakeToolRegistry:
    async def get_provider_tools(self, server_name: str) -> list:
        return [
            type("Tool", (), {"name": "preview_refund_order", "description": "预览退款资格", "args_schema": None})(),
            type("Tool", (), {"name": "refund_order", "description": "提交退款", "args_schema": None})(),
        ]

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict):
        return {"ok": True, "order_id": payload.get("order_id"), "tool_name": tool_name}


def build_runtime() -> SubagentRuntime:
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    return SubagentRuntime(
        broker=ToolBroker(
            registry=FakeToolRegistry(),
            skill_registry=skill_registry,
            provider_registry=provider_registry,
            policy=ToolPolicy.from_skill_registry(skill_registry),
        )
    )


@pytest.mark.anyio
async def test_subagent_runtime_rejects_unallowed_skill():
    runtime = build_runtime()
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    resolution = SkillResolution(
        skill=skill_registry.get_skill("refund.preview"),
        provider=provider_registry.get_provider_for_skill("refund.preview"),
    )
    task = TaskCard(
        task_id="task-001",
        session_id="sess-001",
        domain="refund",
        task_type="refund_preview",
        skill_id="order.list_recent",
        goal="确认订单是否可退款",
        input_slots={"order_id": "ORD-10001", "user_id": 3001},
        required_slots=["order_id"],
        risk_level="medium",
        fallback_policy="handoff",
        expected_output_schema="refund_preview_v1",
    )

    with pytest.raises(PermissionError):
        await runtime.execute(task=task, resolution=resolution, session_state={})


@pytest.mark.anyio
async def test_subagent_runtime_loads_skill_prompt_and_binds_only_skill_tools_for_llm():
    runtime = build_runtime()
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    resolution = SkillResolution(
        skill=skill_registry.get_skill("refund.preview"),
        provider=provider_registry.get_provider_for_skill("refund.preview"),
    )
    task = TaskCard(
        task_id="task-002",
        session_id="sess-002",
        domain="refund",
        task_type="refund_preview",
        skill_id="refund.preview",
        goal="确认订单是否可退款",
        input_slots={"order_id": "ORD-10001", "user_id": 3001},
        required_slots=["order_id"],
        risk_level="medium",
        fallback_policy="handoff",
        expected_output_schema="refund_preview_v1",
    )
    llm = ScriptedChatModel(
        responses=[
            make_tool_call_message("preview_refund_order", {"order_id": "ORD-10001"}),
            AIMessage(content="订单可退款"),
        ]
    )

    result = await runtime.execute(
        task=task,
        resolution=resolution,
        session_state={},
        llm=llm,
    )

    assert llm.bound_tool_names
    assert all(names == ["preview_refund_order"] for names in llm.bound_tool_names)
    assert any("用于预览订单退款资格" in str(message.content) for message in llm.calls[0])
    assert result["output"]["tool_name"] == "preview_refund_order"
