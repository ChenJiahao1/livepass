import pytest
from langchain_core.messages import AIMessage

from app.orchestrator.parent_agent import ParentAgent
from app.orchestrator.policy_engine import PolicyEngine
from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy
from tests.fakes import ScriptedChatModel, build_async_tool, make_tool_call_message


class FlowToolRegistry:
    def __init__(self, *, force_tool_failure: bool = False):
        self.force_tool_failure = force_tool_failure

    async def get_provider_tools(self, server_name: str) -> list:
        async def _list_user_orders(user_id: int, session_id: str | None = None, task_id: str | None = None):
            return {"ok": True}

        async def _preview_refund_order(
            order_id: str,
            user_id: int | None = None,
            session_id: str | None = None,
            task_id: str | None = None,
        ):
            return {"ok": True}

        async def _refund_order(
            order_id: str,
            reason: str | None = None,
            user_id: int | None = None,
            session_id: str | None = None,
            task_id: str | None = None,
        ):
            return {"ok": True}

        async def _create_handoff_ticket(
            reason: str | None = None,
            user_id: int | None = None,
            session_id: str | None = None,
            task_id: str | None = None,
        ):
            return {"ok": True}

        toolsets = {
            "order": [
                build_async_tool(name="list_user_orders", description="list_user_orders", coroutine=_list_user_orders),
                build_async_tool(
                    name="preview_refund_order",
                    description="preview_refund_order",
                    coroutine=_preview_refund_order,
                ),
                build_async_tool(name="refund_order", description="refund_order", coroutine=_refund_order),
            ],
            "handoff": [
                build_async_tool(
                    name="create_handoff_ticket",
                    description="create_handoff_ticket",
                    coroutine=_create_handoff_ticket,
                )
            ],
        }
        return toolsets.get(server_name, [])

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict):
        if self.force_tool_failure and tool_name != "create_handoff_ticket":
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
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "task_type": "order_list_recent"},
            {"action": "delegate", "task_type": "refund_preview"},
            {"action": "reply", "reply": "订单 ORD-10002 当前可退款，预计退款 99.00。"},
        ],
        responses=[
            make_tool_call_message("list_user_orders", {"user_id": 3001}),
            AIMessage(content="最近订单已返回。"),
            make_tool_call_message("preview_refund_order", {"order_id": "ORD-10002"}),
            AIMessage(content="订单可退款。"),
        ],
    )

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": llm, "session_state": {"user_id": 3001}},
    )

    assert result["task_trace"][0]["task_type"] == "order_list_recent"
    assert result["task_trace"][1]["task_type"] == "refund_preview"
    assert result["selected_order_id"] == "ORD-10002"
    assert result["reply"] == "订单 ORD-10002 当前可退款，预计退款 99.00。"
    assert ["list_user_orders"] in llm.bound_tool_names
    assert ["preview_refund_order"] in llm.bound_tool_names
