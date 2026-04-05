import pytest

from app.orchestrator.parent_agent import ParentAgent
from app.orchestrator.policy_engine import PolicyEngine
from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy


class FlowToolRegistry:
    def __init__(self, *, force_tool_failure: bool = False):
        self.force_tool_failure = force_tool_failure

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict):
        if self.force_tool_failure:
            raise RuntimeError("forced tool failure")
        if tool_name == "list_user_orders":
            return {
                "orders": [
                    {
                        "order_id": "ORD-10002",
                        "create_order_time": "2026-04-05 09:00:00",
                    },
                    {
                        "order_id": "ORD-10001",
                        "create_order_time": "2026-04-05 08:00:00",
                    },
                ]
            }
        if tool_name == "preview_refund_order":
            return {
                "order_id": payload["order_id"],
                "allow_refund": True,
                "refund_amount": "99.00",
                "refund_percent": 100,
                "reject_reason": "",
            }
        if tool_name == "create_handoff_ticket":
            return {"ticket_id": "HOF-1001", "queued": True}
        raise AssertionError(tool_name)


def build_flow_agent(*, force_tool_failure: bool = False) -> ParentAgent:
    skill_registry = SkillRegistry.from_config("app/skills/registry.yaml")
    provider_registry = ProviderRegistry.from_config("app/skills/registry.yaml")
    return ParentAgent(
        policy_engine=PolicyEngine(max_steps_limit=3),
        skill_resolver=SkillResolver(
            skill_registry=skill_registry,
            provider_registry=provider_registry,
        ),
        runtime=SubagentRuntime(
            broker=ToolBroker(
                registry=FlowToolRegistry(force_tool_failure=force_tool_failure),
                skill_registry=skill_registry,
                provider_registry=provider_registry,
                policy=ToolPolicy.from_skill_registry(skill_registry),
            )
        ),
    )


@pytest.mark.anyio
async def test_parent_agent_splits_recent_order_refund_into_preview_flow():
    agent = build_flow_agent()

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": None, "session_state": {"user_id": 3001}},
    )

    assert result["task_trace"][0]["task_type"] == "order_list_recent"
    assert result["task_trace"][1]["task_type"] == "refund_preview"
    assert result["selected_order_id"] == "ORD-10002"

