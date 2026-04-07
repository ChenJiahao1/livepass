"""LLM-driven parent agent orchestration."""

from __future__ import annotations

import re
from typing import Any, Literal
from uuid import uuid4

from langchain_core.messages import HumanMessage, SystemMessage
from pydantic import BaseModel, model_validator

from app.knowledge.service import KnowledgeService
from app.mcp_client.registry import MCPToolRegistry
from app.orchestrator.policy_engine import PolicyEngine
from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.tasking.task_card import TaskCard
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy

ORDER_ID_PATTERN = re.compile(r"(ORD-\d+|\d+)", re.IGNORECASE)
TASK_TYPES = (
    "order_list_recent",
    "order_get_detail",
    "refund_preview",
    "refund_submit",
    "handoff_create_ticket",
)
ParentAction = Literal["reply", "clarify", "delegate", "knowledge"]
ParentTaskType = Literal[
    "order_list_recent",
    "order_get_detail",
    "refund_preview",
    "refund_submit",
    "handoff_create_ticket",
]

PARENT_SYSTEM_PROMPT = """你是 damai-go 的父层客服编排 Agent。

你的职责是理解用户诉求，并在以下动作中四选一：
1. `reply`：你已经有足够事实，可以直接回复用户。
2. `clarify`：缺少关键信息，必须先追问用户。
3. `delegate`：需要发起一个具体业务 task，交给受控 subagent 执行一个 skill。
4. `knowledge`：这是明星基础百科类问题，应交给知识能力模块处理。

编排约束：
- 你自己不直接执行工具，也不假装拥有业务结果。
- 一次只下发一张 TaskCard，只允许选择一个 task_type。
- 只有在拿到前一步执行结果后，才能决定是否继续下发下一张 TaskCard。
- 如果用户要人工客服，或自动处理明显不合适，优先选择 `delegate` + `handoff_create_ticket`。
- 对明星实时新闻、八卦、热搜，不要当成知识问答主链；让知识模块返回边界提示。

可用 task_type：
- order_list_recent: 查询最近订单
- order_get_detail: 查询指定订单详情
- refund_preview: 预览退款资格
- refund_submit: 提交退款申请
- handoff_create_ticket: 创建人工客服工单
"""


class ParentDecision(BaseModel):
    action: ParentAction
    reply: str | None = None
    task_type: ParentTaskType | None = None

    @model_validator(mode="after")
    def validate_payload(self) -> "ParentDecision":
        if self.action in {"reply", "clarify"} and not self.reply:
            raise ValueError("reply is required for reply/clarify actions")
        if self.action == "delegate" and self.task_type is None:
            raise ValueError("task_type is required for delegate action")
        if self.action != "delegate" and self.task_type is not None:
            raise ValueError("task_type is only allowed for delegate action")
        return self


class ParentAgent:
    def __init__(
        self,
        *,
        policy_engine: PolicyEngine | None = None,
        skill_resolver: SkillResolver | None = None,
        runtime: SubagentRuntime | None = None,
        knowledge_service: Any | None = None,
        max_turns: int = 4,
    ) -> None:
        skill_registry = SkillRegistry.from_default()
        provider_registry = ProviderRegistry.from_default()
        self.policy_engine = policy_engine or PolicyEngine(max_steps_limit=3)
        self.skill_resolver = skill_resolver or SkillResolver(
            skill_registry=skill_registry,
            provider_registry=provider_registry,
        )
        self.runtime = runtime
        self.knowledge_service = knowledge_service or KnowledgeService()
        self.max_turns = max_turns

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
        task_type: ParentTaskType,
        overrides: dict[str, Any] | None = None,
    ) -> TaskCard:
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
        llm = context.get("llm")
        if llm is None:
            raise ValueError("llm is required")

        session_state = dict(context.get("session_state") or {})
        session_state["session_id"] = (
            config or {}
        ).get("configurable", {}).get("thread_id", session_state.get("session_id", "session"))
        if context.get("current_user_id") and "user_id" not in session_state:
            session_state["user_id"] = int(context["current_user_id"])

        user_message = self._latest_user_message(input_state)
        runtime = self._ensure_runtime(context.get("registry"))
        task_trace: list[dict[str, Any]] = []
        last_execution: dict[str, Any] | None = None

        for _ in range(self.max_turns):
            decision = await self._decide(
                llm=llm,
                user_message=user_message,
                session_state=session_state,
                last_execution=last_execution,
            )
            if decision.action == "reply":
                return self._finalize(
                    reply=decision.reply or "已处理完成。",
                    need_handoff=False,
                    task_trace=task_trace,
                    session_state=session_state,
                    status="completed",
                )
            if decision.action == "clarify":
                return self._finalize(
                    reply=decision.reply or "请补充更多信息。",
                    need_handoff=False,
                    task_trace=task_trace,
                    session_state=session_state,
                    status="clarify",
                )
            if decision.action == "knowledge":
                knowledge_result = await self._answer_with_knowledge(user_message)
                return self._finalize(
                    reply=knowledge_result["reply"],
                    need_handoff=False,
                    task_trace=task_trace,
                    session_state=session_state,
                    status="completed",
                )

            current_task = self.policy_engine.apply(
                self.build_task(
                    user_message=user_message,
                    session_state=session_state,
                    task_type=decision.task_type or "handoff_create_ticket",
                )
            )
            try:
                resolution = self.skill_resolver.resolve(current_task)
                last_execution = await runtime.execute(
                    task=current_task,
                    resolution=resolution,
                    session_state=session_state,
                    llm=llm,
                )
            except Exception:
                handoff = await self._handoff_after_failure(
                    session_state=session_state,
                    user_message=user_message,
                    llm=llm,
                    runtime=runtime,
                )
                task_trace.extend(handoff.get("task_trace", []))
                self._merge_session_state(session_state, handoff)
                return self._finalize(
                    reply="已为你转接人工客服，请稍候。",
                    need_handoff=True,
                    task_trace=task_trace,
                    session_state=session_state,
                    status="handoff",
                )

            task_trace.append(
                {
                    "task_id": current_task.task_id,
                    "task_type": current_task.task_type,
                    "skill_id": last_execution["skill_id"],
                    "tool_calls": last_execution["tool_calls"],
                }
            )
            self._merge_session_state(session_state, last_execution)

            if last_execution.get("need_handoff"):
                return self._finalize(
                    reply="已为你转接人工客服，请稍候。",
                    need_handoff=True,
                    task_trace=task_trace,
                    session_state=session_state,
                    status="handoff",
                )

        fallback_reply = self._reply_from_execution(last_execution, session_state)
        return self._finalize(
            reply=fallback_reply,
            need_handoff=bool(last_execution and last_execution.get("need_handoff")),
            task_trace=task_trace,
            session_state=session_state,
            status="completed",
        )

    async def _decide(
        self,
        *,
        llm: Any,
        user_message: str,
        session_state: dict[str, Any],
        last_execution: dict[str, Any] | None,
    ) -> ParentDecision:
        planner = llm.with_structured_output(ParentDecision)
        return await planner.ainvoke(self._build_planning_messages(user_message, session_state, last_execution))

    def _build_planning_messages(
        self,
        user_message: str,
        session_state: dict[str, Any],
        last_execution: dict[str, Any] | None,
    ) -> list[Any]:
        session_lines = [
            f"user_id: {session_state.get('user_id')}",
            f"selected_order_id: {session_state.get('selected_order_id')}",
            f"recent_order_candidates: {session_state.get('recent_order_candidates', [])}",
            f"last_task_summary: {session_state.get('last_task_summary')}",
            f"last_handoff_ticket_id: {session_state.get('last_handoff_ticket_id')}",
        ]
        execution_summary = "无"
        if last_execution is not None:
            execution_summary = str(
                {
                    "task_type": last_execution.get("task_type"),
                    "skill_id": last_execution.get("skill_id"),
                    "summary": last_execution.get("summary"),
                    "output": last_execution.get("output"),
                    "need_handoff": last_execution.get("need_handoff", False),
                }
            )
        return [
            SystemMessage(content=PARENT_SYSTEM_PROMPT),
            HumanMessage(
                content="\n".join(
                    [
                        "请基于当前会话状态和最近一次执行结果做一个动作决策。",
                        f"用户消息: {user_message}",
                        "当前会话状态:",
                        *session_lines,
                        f"最近一次执行结果: {execution_summary}",
                        f"可选 task_type: {', '.join(TASK_TYPES)}",
                    ]
                )
            ),
        ]

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

    async def _answer_with_knowledge(self, user_message: str) -> dict[str, Any]:
        if self.knowledge_service is None:
            return {"reply": "当前未接入知识问答能力。"}
        return await self.knowledge_service.handle(
            {"messages": [{"role": "user", "content": user_message}]}
        )

    def _merge_session_state(self, session_state: dict[str, Any], execution: dict[str, Any]) -> None:
        if execution.get("selected_order_id"):
            session_state["selected_order_id"] = execution["selected_order_id"]
        if execution.get("recent_order_candidates") is not None:
            session_state["recent_order_candidates"] = execution["recent_order_candidates"]
        if execution.get("summary"):
            session_state["last_task_summary"] = execution["summary"]
        if execution.get("handoff_ticket_id"):
            session_state["last_handoff_ticket_id"] = execution["handoff_ticket_id"]

    def _reply_from_execution(self, execution: dict[str, Any] | None, session_state: dict[str, Any]) -> str:
        if execution is None:
            return "已处理完成。"
        output = execution.get("output", {})
        if execution.get("need_handoff"):
            return "已为你转接人工客服，请稍候。"
        if execution.get("task_type") == "refund_preview":
            if output.get("allow_refund"):
                return (
                    f"订单 {output.get('order_id', session_state.get('selected_order_id'))} 当前可退款，"
                    f"预计退款 {output.get('refund_amount')}。"
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

    def _finalize(
        self,
        *,
        reply: str,
        need_handoff: bool,
        task_trace: list[dict[str, Any]],
        session_state: dict[str, Any],
        status: str,
    ) -> dict[str, Any]:
        return {
            "reply": reply,
            "final_reply": reply,
            "need_handoff": need_handoff,
            "task_trace": task_trace,
            "selected_order_id": session_state.get("selected_order_id"),
            "session_state": session_state,
            "route_source": "llm",
            "status": status,
        }
