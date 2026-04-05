"""Bounded skill execution runtime."""

from __future__ import annotations

import json
from typing import Any

from langchain.agents import create_agent
from langchain_core.messages import AIMessage, BaseMessage, ToolMessage

from app.orchestrator.skill_resolver import SkillResolution
from app.runtime.react_loop import ReactLoop
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
        self.react_loop = ReactLoop(broker=broker)
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
        if resolution.skill.skill_id != task.skill_id:
            raise PermissionError(
                f"skill {resolution.skill.skill_id} does not match task skill {task.skill_id}"
            )

        if llm is None:
            ctx = await self.react_loop.run(task=task, resolution=resolution)
            output = ctx.structured_output or {}
            tool_calls = ctx.tool_calls
        else:
            output, tool_calls = await self._run_with_llm(
                task=task,
                resolution=resolution,
                llm=llm,
            )
        summary = self._build_summary(task.task_type, output)
        result = {
            "task_id": task.task_id,
            "task_type": task.task_type,
            "skill_id": resolution.skill.skill_id,
            "tool_calls": tool_calls,
            "output": output,
            "summary": summary,
            "need_handoff": task.task_type == "handoff_create_ticket",
            "selected_order_id": output.get("order_id") or session_state.get("selected_order_id"),
        }
        if task.task_type == "order_list_recent":
            result["recent_order_candidates"] = output.get("orders", [])
        if task.task_type == "handoff_create_ticket":
            result["handoff_ticket_id"] = output.get("ticket_id")
        return result

    async def _run_with_llm(
        self,
        *,
        task: TaskCard,
        resolution: SkillResolution,
        llm: Any,
    ) -> tuple[dict[str, Any], list[str]]:
        tools = await self.broker.get_skill_tools(resolution.skill.skill_id, task=task)
        agent = create_agent(
            model=llm,
            tools=tools,
            system_prompt=self._build_system_prompt(resolution.skill.skill_id),
            name=resolution.skill.agent_type,
        )
        result = await agent.ainvoke(
            {"messages": [{"role": "user", "content": self._build_task_message(task)}]},
            config={"recursion_limit": max(4, task.max_steps * 2 + 2)},
        )
        messages = result.get("messages", [])
        return self._extract_structured_output(messages), self._extract_tool_calls(messages)

    def _build_system_prompt(self, skill_id: str) -> str:
        skill_prompt = ""
        if self.skill_registry is not None:
            skill_prompt = self.skill_registry.load_skill_markdown(skill_id)
        return f"{self.system_prompt}\n\n<loaded_skill id=\"{skill_id}\">\n{skill_prompt}\n</loaded_skill>"

    def _build_task_message(self, task: TaskCard) -> str:
        lines = [
            f"skill_id: {task.skill_id}",
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
        if task_type == "order_list_recent":
            orders = output.get("orders", [])
            if not orders:
                return "当前账号无可处理订单"
            return "已获取最近订单列表"
        if task_type == "refund_preview":
            if output.get("allow_refund"):
                return "退款资格已确认"
            return output.get("reject_reason", "退款被拒绝")
        if task_type == "refund_submit":
            return "退款申请已提交"
        if task_type == "handoff_create_ticket":
            return "人工工单已创建"
        return "任务已执行"
