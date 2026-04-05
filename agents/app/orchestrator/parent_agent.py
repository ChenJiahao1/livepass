"""Parent agent for lightweight task orchestration."""

from __future__ import annotations

import re
from typing import Any
from uuid import uuid4

from app.orchestrator.policy_engine import PolicyEngine
from app.orchestrator.skill_resolver import SkillResolver
from app.mcp_client.registry import MCPToolRegistry
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.tasking.task_card import TaskCard
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy

ORDER_ID_PATTERN = re.compile(r"(ORD-\d+|\d+)", re.IGNORECASE)
REFUND_KEYWORDS = ("退款", "退票", "退单", "退")
SUBMIT_REFUND_KEYWORDS = ("申请退款", "发起退款", "确认退款")
HANDOFF_KEYWORDS = ("人工", "客服", "转人工")


class ParentAgent:
    def __init__(
        self,
        *,
        policy_engine: PolicyEngine | None = None,
        skill_resolver: SkillResolver | None = None,
        runtime: SubagentRuntime | None = None,
    ) -> None:
        skill_registry = SkillRegistry.from_default()
        provider_registry = ProviderRegistry.from_default()
        self.policy_engine = policy_engine or PolicyEngine(max_steps_limit=3)
        self.skill_resolver = skill_resolver or SkillResolver(
            skill_registry=skill_registry,
            provider_registry=provider_registry,
        )
        self.runtime = runtime

    def _ensure_runtime(self, registry: Any | None) -> SubagentRuntime:
        if self.runtime is not None:
            return self.runtime

        skill_registry = self.skill_resolver.skill_registry
        provider_registry = self.skill_resolver.provider_registry
        self.runtime = SubagentRuntime(
            broker=ToolBroker(
                registry=registry or MCPToolRegistry(),
                skill_registry=skill_registry,
                provider_registry=provider_registry,
                policy=ToolPolicy.from_skill_registry(skill_registry),
            ),
            skill_registry=skill_registry,
        )
        return self.runtime

    def build_task(
        self,
        *,
        user_message: str,
        session_state: dict[str, Any],
        llm: Any,
        task_type: str | None = None,
        overrides: dict[str, Any] | None = None,
    ) -> TaskCard:
        task_type = task_type or self._route_task_type(user_message, session_state)
        input_slots = dict(overrides or {})
        if "user_id" not in input_slots and session_state.get("user_id") is not None:
            input_slots["user_id"] = session_state["user_id"]
        order_id = self._extract_order_id(user_message) or session_state.get("selected_order_id")
        if order_id and "order_id" not in input_slots:
            input_slots["order_id"] = order_id

        skill_id: str
        required_slots: list[str]
        expected_output_schema: str
        goal: str
        fallback_policy = "handoff"

        if task_type == "order_list_recent":
            skill_id = "order.list_recent"
            required_slots = []
            expected_output_schema = "order_list_recent_v1"
            goal = "查询最近订单"
            fallback_policy = "return_parent"
        elif task_type == "order_get_detail":
            skill_id = "order.get_detail"
            required_slots = ["order_id"]
            expected_output_schema = "order_detail_v1"
            goal = "查询订单详情"
            fallback_policy = "return_parent"
        elif task_type == "refund_submit":
            skill_id = "refund.submit"
            required_slots = ["order_id"]
            expected_output_schema = "refund_submit_v1"
            goal = "提交订单退款"
        elif task_type == "handoff_create_ticket":
            skill_id = "handoff.create_ticket"
            required_slots = []
            expected_output_schema = "handoff_ticket_v1"
            goal = "创建人工工单"
        else:
            skill_id = "refund.preview"
            required_slots = ["order_id"]
            expected_output_schema = "refund_preview_v1"
            goal = "确认订单是否可退款并返回预计退款金额"

        return TaskCard(
            task_id=f"task_{uuid4().hex[:12]}",
            session_id=str(session_state.get("session_id") or "session"),
            domain=skill_id.split(".")[0],
            task_type=task_type,
            skill_id=skill_id,
            goal=goal,
            source_message=user_message,
            input_slots=input_slots,
            required_slots=required_slots,
            risk_level="medium" if "refund" in task_type else "low",
            fallback_policy=fallback_policy,
            expected_output_schema=expected_output_schema,
        )

    async def ainvoke(self, input_state: dict[str, Any], config: dict | None, context: dict | None) -> dict[str, Any]:
        context = context or {}
        session_state = dict(context.get("session_state") or {})
        session_state["session_id"] = (
            config or {}
        ).get("configurable", {}).get("thread_id", session_state.get("session_id", "session"))
        if context.get("current_user_id") and "user_id" not in session_state:
            session_state["user_id"] = int(context["current_user_id"])

        user_message = self._latest_user_message(input_state)
        task_trace: list[dict[str, Any]] = []
        route_source = "rule" if context.get("llm") is None else "skill"
        runtime = self._ensure_runtime(context.get("registry"))

        current_task = self.policy_engine.apply(
            self.build_task(user_message=user_message, session_state=session_state, llm=context.get("llm"))
        )
        final_result: dict[str, Any] | None = None

        while current_task is not None:
            try:
                resolution = self.skill_resolver.resolve(current_task)
                final_result = await runtime.execute(
                    task=current_task,
                    resolution=resolution,
                    session_state=session_state,
                    llm=context.get("llm"),
                )
            except Exception:
                final_result = await self._handoff_after_failure(
                    session_state=session_state,
                    user_message=user_message,
                    llm=context.get("llm"),
                    runtime=runtime,
                )
                task_trace.append(final_result["task_trace"][0])
                break

            task_trace.append(
                {
                    "task_id": current_task.task_id,
                    "task_type": current_task.task_type,
                    "skill_id": final_result["skill_id"],
                    "tool_calls": final_result["tool_calls"],
                }
            )
            self._merge_session_state(session_state, final_result)

            next_task = self._build_follow_up_task(
                current_task=current_task,
                user_message=user_message,
                session_state=session_state,
                execution=final_result,
            )
            if next_task is None:
                break
            current_task = self.policy_engine.apply(next_task)

        final_result = final_result or {}
        return {
            "reply": self._build_reply(final_result, session_state),
            "final_reply": self._build_reply(final_result, session_state),
            "need_handoff": bool(final_result.get("need_handoff")),
            "task_trace": task_trace,
            "selected_order_id": session_state.get("selected_order_id"),
            "session_state": session_state,
            "route_source": route_source,
        }

    async def _handoff_after_failure(
        self,
        *,
        session_state: dict[str, Any],
        user_message: str,
        llm: Any,
        runtime: SubagentRuntime,
    ) -> dict[str, Any]:
        handoff_task = self.policy_engine.apply(
            self.build_task(
                user_message=user_message,
                session_state=session_state,
                llm=None,
                task_type="handoff_create_ticket",
            )
        )
        try:
            resolution = self.skill_resolver.resolve(handoff_task)
            result = await runtime.execute(
                task=handoff_task,
                resolution=resolution,
                session_state=session_state,
                llm=llm,
            )
            result["task_trace"] = [
                {
                    "task_id": handoff_task.task_id,
                    "task_type": handoff_task.task_type,
                    "skill_id": result["skill_id"],
                    "tool_calls": result["tool_calls"],
                }
            ]
            return result
        except Exception:
            return {
                "need_handoff": True,
                "summary": "人工工单已创建",
                "task_trace": [
                    {
                        "task_id": handoff_task.task_id,
                        "task_type": handoff_task.task_type,
                        "skill_id": "handoff.create_ticket",
                        "tool_calls": [],
                    }
                ],
            }

    def _build_follow_up_task(
        self,
        *,
        current_task: TaskCard,
        user_message: str,
        session_state: dict[str, Any],
        execution: dict[str, Any],
    ) -> TaskCard | None:
        if current_task.task_type == "order_list_recent":
            orders = execution.get("output", {}).get("orders", [])
            if not orders:
                return None
            session_state["selected_order_id"] = orders[0].get("order_id")
            if any(keyword in user_message for keyword in REFUND_KEYWORDS):
                next_task_type = "refund_submit" if any(
                    keyword in user_message for keyword in SUBMIT_REFUND_KEYWORDS
                ) else "refund_preview"
                return self.build_task(
                    user_message=user_message,
                    session_state=session_state,
                    llm=None,
                    task_type=next_task_type,
                )
            return None
        return None

    def _merge_session_state(self, session_state: dict[str, Any], execution: dict[str, Any]) -> None:
        if execution.get("selected_order_id"):
            session_state["selected_order_id"] = execution["selected_order_id"]
        if execution.get("recent_order_candidates") is not None:
            session_state["recent_order_candidates"] = execution["recent_order_candidates"]
        if execution.get("summary"):
            session_state["last_task_summary"] = execution["summary"]
        if execution.get("handoff_ticket_id"):
            session_state["last_handoff_ticket_id"] = execution["handoff_ticket_id"]

    def _route_task_type(self, user_message: str, session_state: dict[str, Any]) -> str:
        if any(keyword in user_message for keyword in HANDOFF_KEYWORDS):
            return "handoff_create_ticket"
        order_id = self._extract_order_id(user_message) or session_state.get("selected_order_id")
        if any(keyword in user_message for keyword in REFUND_KEYWORDS):
            if order_id:
                if any(keyword in user_message for keyword in SUBMIT_REFUND_KEYWORDS):
                    return "refund_submit"
                return "refund_preview"
            return "order_list_recent"
        if order_id:
            return "order_get_detail"
        return "order_list_recent"

    def _extract_order_id(self, message: str) -> str | None:
        match = ORDER_ID_PATTERN.search(message)
        if not match:
            return None
        order_id = match.group(1)
        return order_id if order_id.startswith("ORD-") else f"ORD-{order_id}"

    def _latest_user_message(self, input_state: dict[str, Any]) -> str:
        messages = input_state.get("messages", [])
        if not messages:
            return ""
        last_message = messages[-1]
        if hasattr(last_message, "content"):
            return str(last_message.content)
        if isinstance(last_message, dict):
            return str(last_message.get("content", ""))
        return str(last_message)

    def _build_reply(self, execution: dict[str, Any], session_state: dict[str, Any]) -> str:
        output = execution.get("output", {})
        if execution.get("need_handoff"):
            return "已为你转接人工客服，请稍候。"
        if execution.get("task_type") == "refund_preview":
            if output.get("allow_refund"):
                return (
                    f"订单 {output.get('order_id', session_state.get('selected_order_id'))} 当前可退款，"
                    f"预计退款 {output.get('refund_amount')}，退款比例 {output.get('refund_percent')}%。"
                )
            return output.get("reject_reason", "当前订单不可退。")
        if execution.get("task_type") == "refund_submit":
            return f"订单 {output.get('order_id', session_state.get('selected_order_id'))} 已提交退款。"
        if execution.get("task_type") == "order_list_recent":
            orders = output.get("orders", [])
            if orders:
                return f"已找到最近订单 {orders[0].get('order_id')}。"
            return "当前账号下没有可查询订单。"
        return session_state.get("last_task_summary", "已处理完成。")
