"""Bounded skill execution runtime."""

from __future__ import annotations

import json
from typing import Any

from langchain.agents import create_agent
from langchain_core.messages import AIMessage, BaseMessage, ToolMessage

from app.orchestrator.skill_resolver import SkillResolution
from app.registry.skill_registry import SkillRegistry
from app.tasking.task_card import TaskCard

DEFAULT_SUBAGENT_SYSTEM_PROMPT = """你是 damai-go 的通用业务子代理。

你一次只执行一个已经选定的 skill，不负责重新路由，也不负责决定是否切换 skill。

执行规则：
1. 先严格阅读当前加载的 skill 内容，再决定如何完成任务。
2. 只使用当前显式暴露给你的 tools；看不到的 tool 视为不可用。
3. 缺少必填输入时不要猜测，直接依据当前事实返回失败结论。
4. 所有业务结论必须以 tool 返回结果为依据，禁止编造订单、退款或工单数据。
5. 优先返回结构化、简洁、可审计的结果。
"""


class SubagentRuntime:
    def __init__(
        self,
        *,
        broker,
        skill_registry: SkillRegistry | None = None,
        system_prompt: str = DEFAULT_SUBAGENT_SYSTEM_PROMPT,
    ) -> None:
        self.broker = broker
        self.skill_registry = skill_registry or getattr(broker, "skill_registry", None)
        self.system_prompt = system_prompt

    async def execute(
        self,
        *,
        task: TaskCard,
        resolution: SkillResolution,
        session_state: dict[str, Any],
        llm: Any | None = None,
    ) -> dict[str, Any]:
        resolution_skill_ids = [skill.skill_id for skill in resolution.skills]
        if resolution_skill_ids != task.allowed_skills:
            raise PermissionError(
                f"resolved skills {resolution_skill_ids} do not match task skills {task.allowed_skills}"
            )
        if llm is None:
            raise ValueError("llm is required")

        output, tool_calls = await self._run_with_llm(
            task=task,
            resolution=resolution,
            llm=llm,
        )
        summary = self._build_summary(task.task_type, output)
        result = {
            "task_id": task.task_id,
            "task_type": task.task_type,
            "skill_id": task.allowed_skills[0],
            "skill_ids": list(task.allowed_skills),
            "tool_calls": tool_calls,
            "output": output,
            "summary": summary,
            "need_handoff": task.task_type == "handoff_write",
            "selected_order_id": self._extract_selected_order_id(output, session_state),
        }
        if task.task_type in {"order_read", "refund_read"}:
            result["recent_order_candidates"] = output.get("orders", [])
        if task.task_type in {"refund_read"} and (
            "allow_refund" in output or "refund_amount" in output or "reject_reason" in output
        ):
            result["last_refund_preview"] = {
                "order_id": output.get("order_id") or result["selected_order_id"],
                "allow_refund": output.get("allow_refund"),
                "refund_amount": output.get("refund_amount"),
                "reject_reason": output.get("reject_reason", ""),
            }
        if task.task_type == "handoff_write":
            result["handoff_ticket_id"] = output.get("ticket_id")
        return result

    async def _run_with_llm(
        self,
        *,
        task: TaskCard,
        resolution: SkillResolution,
        llm: Any,
    ) -> tuple[dict[str, Any], list[str]]:
        tools = await self.broker.get_task_tools(task)
        agent = create_agent(
            model=llm,
            tools=tools,
            system_prompt=self._build_system_prompt(task.allowed_skills),
            name=resolution.skills[0].agent_type,
        )
        result = await agent.ainvoke(
            {"messages": [{"role": "user", "content": self._build_task_message(task)}]},
            config={"recursion_limit": max(4, task.max_steps * 2 + 2)},
        )
        messages = result.get("messages", [])
        return self._extract_structured_output(messages), self._extract_tool_calls(messages)

    def _build_system_prompt(self, skill_ids: list[str]) -> str:
        skill_prompt = ""
        if self.skill_registry is not None:
            loaded_skills = [
                f"<loaded_skill id=\"{skill_id}\">\n{self.skill_registry.load_skill_markdown(skill_id)}\n</loaded_skill>"
                for skill_id in skill_ids
            ]
            skill_prompt = "\n\n".join(loaded_skills)
        return f"{self.system_prompt}\n\n{skill_prompt}"

    def _build_task_message(self, task: TaskCard) -> str:
        lines = [
            f"allowed_skills: {', '.join(task.allowed_skills)}",
            f"task_type: {task.task_type}",
            f"goal: {task.goal}",
            f"expected_output_schema: {task.expected_output_schema}",
        ]
        if task.source_message:
            lines.append(f"source_message: {task.source_message}")
        if task.required_slots:
            lines.append(f"required_slots: {', '.join(task.required_slots)}")
        if task.input_slots:
            lines.append(f"input_slots: {json.dumps(task.input_slots, ensure_ascii=False, sort_keys=True)}")
        lines.append("请按当前 skill 执行任务；若需要工具，请调用工具后再给出结论。")
        return "\n".join(lines)

    def _extract_structured_output(self, messages: list[BaseMessage]) -> dict[str, Any]:
        for message in reversed(messages):
            if not isinstance(message, ToolMessage):
                continue
            normalized = self._normalize_content(message.content)
            if isinstance(normalized, dict):
                return normalized
        return {}

    def _extract_tool_calls(self, messages: list[BaseMessage]) -> list[str]:
        tool_calls: list[str] = []
        for message in messages:
            if not isinstance(message, AIMessage):
                continue
            for tool_call in message.tool_calls:
                name = tool_call.get("name")
                if isinstance(name, str):
                    tool_calls.append(name)
        return tool_calls

    def _normalize_content(self, content: Any) -> Any:
        if isinstance(content, dict):
            return content
        if isinstance(content, str):
            return self._maybe_parse_json(content)
        if isinstance(content, list):
            text_parts: list[str] = []
            for item in content:
                if isinstance(item, str):
                    text_parts.append(item)
                elif isinstance(item, dict) and item.get("type") == "text":
                    text_parts.append(str(item.get("text", "")))
            if text_parts:
                return self._maybe_parse_json("\n".join(text_parts))
        return content

    def _maybe_parse_json(self, text: str) -> Any:
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text

    def _build_summary(self, task_type: str, output: dict[str, Any]) -> str:
        if task_type == "order_read":
            orders = output.get("orders", [])
            if not orders:
                return "当前账号无可处理订单"
            return "已获取最近订单列表"
        if task_type == "refund_read":
            if output.get("allow_refund"):
                return "退款资格已确认"
            if output.get("orders"):
                return "已获取退款相关订单信息"
            return output.get("reject_reason", "退款被拒绝")
        if task_type == "refund_write":
            return "退款申请已提交"
        if task_type == "handoff_write":
            return "人工工单已创建"
        return "任务已执行"

    def _extract_selected_order_id(self, output: dict[str, Any], session_state: dict[str, Any]) -> str | None:
        if output.get("order_id"):
            return str(output["order_id"])
        orders = output.get("orders", [])
        if isinstance(orders, list) and orders:
            first = orders[0]
            if isinstance(first, dict) and first.get("order_id"):
                return str(first["order_id"])
        selected = session_state.get("selected_order_id")
        return str(selected) if selected is not None else None
