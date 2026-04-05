import pytest

from app.orchestrator.parent_agent import ParentAgent
from app.orchestrator.policy_engine import PolicyEngine
from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy


class FakeToolRegistry:
    async def get_provider_tools(self, server_name: str) -> list:
        return []

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict):
        if tool_name == "list_user_orders":
            return {"orders": [{"order_id": "ORD-10001", "create_order_time": "2026-04-05 10:00:00"}]}
        if tool_name == "preview_refund_order":
            return {
                "order_id": "ORD-10001",
                "allow_refund": True,
                "refund_amount": "99.00",
                "refund_percent": 100,
                "reject_reason": "",
            }
        raise AssertionError(tool_name)


@pytest.mark.anyio
async def test_rule_based_parent_agent_works_without_llm():
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    agent = ParentAgent(
        policy_engine=PolicyEngine(max_steps_limit=3),
        skill_resolver=SkillResolver(
            skill_registry=skill_registry,
            provider_registry=provider_registry,
        ),
        runtime=SubagentRuntime(
            broker=ToolBroker(
                registry=FakeToolRegistry(),
                skill_registry=skill_registry,
                provider_registry=provider_registry,
                policy=ToolPolicy.from_skill_registry(skill_registry),
            )
        ),
    )

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": None, "session_state": {"user_id": 3001}},
    )

    assert result["route_source"] == "rule"
    assert result["task_trace"][0]["task_type"] == "order_list_recent"
    assert result["task_trace"][1]["task_type"] == "refund_preview"


@pytest.mark.anyio
async def test_parent_agent_bootstraps_default_runtime_from_request_registry():
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    agent = ParentAgent(
        policy_engine=PolicyEngine(max_steps_limit=3),
        skill_resolver=SkillResolver(
            skill_registry=skill_registry,
            provider_registry=provider_registry,
        ),
    )

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={
            "registry": FakeToolRegistry(),
            "current_user_id": "3001",
            "llm": None,
            "session_state": {"user_id": 3001},
        },
    )

    assert result["task_trace"][0]["task_type"] == "order_list_recent"
    assert result["task_trace"][1]["task_type"] == "refund_preview"
