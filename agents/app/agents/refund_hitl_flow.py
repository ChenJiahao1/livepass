from __future__ import annotations

from typing import Any, Mapping

from langchain_core.messages import AIMessage
from langgraph.runtime import Runtime
from langgraph.types import interrupt

from app.agent_runtime.human_tools import build_human_approval_interrupt
from app.agents.refund import RefundAgent
from app.state import ConversationState, GraphContext


async def prepare_refund(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    agent = _refund_agent(runtime)
    order_id = agent.extract_order_id(hydrated)
    if not order_id:
        reply = "请先提供需要处理的订单号。"
        return _finish_payload(state=state, reply=reply, result_summary="缺少订单号")

    return {
        "current_agent": "refund",
        "selected_order_id": order_id,
        "current_user_id": hydrated.get("current_user_id"),
        "pending_human_action": None,
        "human_decision": None,
        "refund_preview": None,
        "refund_result": None,
        "refund_rejected": False,
    }


async def preview_refund(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    order_id = state.get("selected_order_id")
    if not order_id:
        return {}

    tools = await _refund_agent(runtime).get_tools()
    preview_tool = _refund_agent(runtime).find_tool(tools, "preview_refund_order")
    if preview_tool is None:
        reply = "当前暂时无法预览退款，请稍后再试。"
        return _finish_payload(state=state, reply=reply, result_summary="缺少退款预览工具")

    payload = {"order_id": str(order_id)}
    current_user_id = state.get("current_user_id") or runtime.context.get("current_user_id")
    if current_user_id is not None:
        payload["user_id"] = current_user_id
    preview = await preview_tool.ainvoke(payload)
    preview_payload = _normalize_preview(preview, str(order_id))
    if not preview_payload["allow_refund"]:
        reply = preview_payload["reject_reason"] or "当前订单不可退。"
        return {
            **_finish_payload(state=state, reply=reply, result_summary="退款预览拒绝"),
            "refund_preview": preview_payload,
        }

    reply = (
        f"订单 {order_id} 当前可退款，预计退款 {preview_payload['refund_amount']}，"
        f"退款比例 {preview_payload['refund_percent']}%。是否确认退款？"
    )
    return {
        "current_agent": "refund",
        "selected_order_id": str(order_id),
        "refund_preview": preview_payload,
        "pending_human_action": _build_pending_refund_action(
            order_id=str(order_id),
            preview=preview_payload,
            current_user_id=current_user_id,
        ),
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
    }


async def request_refund_approval(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    pending_action = state.get("pending_human_action") or {}
    interrupt_payload = pending_action.get("interrupt")
    if not isinstance(interrupt_payload, Mapping):
        return {}

    decision = interrupt(dict(interrupt_payload))
    return {"human_decision": _normalize_decision(decision)}


def apply_human_decision(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    decision = state.get("human_decision") or {}
    return {"refund_rejected": decision.get("action") != "approve"}


async def submit_refund(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    decision = state.get("human_decision") or {}
    if decision.get("action") not in {"approve", "edit"}:
        return {"refund_result": None, "refund_rejected": True}

    tools = await _refund_agent(runtime).get_tools()
    refund_tool = _refund_agent(runtime).find_tool(tools, "refund_order")
    if refund_tool is None:
        raise ValueError("refund_order tool is required")

    pending_action = state.get("pending_human_action") or {}
    values = dict(pending_action.get("values") or {})
    values.update(decision.get("values") or {})
    result = await refund_tool.ainvoke(values)
    return {"refund_result": dict(result), "refund_rejected": False}


def finish_refund(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    decision = state.get("human_decision") or {}
    result = state.get("refund_result")
    if decision.get("action") == "reject" or state.get("refund_rejected"):
        reply = "已取消本次退款操作。"
        return _finish_payload(state=state, reply=reply, result_summary="用户拒绝退款")

    if result:
        order_id = result.get("order_id") or result.get("orderId") or state.get("selected_order_id")
        refund_amount = result.get("refund_amount") or result.get("refundAmount", "待确认")
        reply = f"订单 {order_id} 已提交退款，退款金额 {refund_amount}。"
        return _finish_payload(state=state, reply=reply, result_summary="退款已提交")

    reply = state.get("reply") or state.get("final_reply") or ""
    return _finish_payload(state=state, reply=str(reply), result_summary=state.get("reply") or "退款流程结束")


def should_preview_refund(state: ConversationState) -> str:
    specialist_result = state.get("specialist_result") or {}
    if specialist_result.get("completed"):
        return "finish"
    return "preview"


def should_request_refund_approval(state: ConversationState) -> str:
    if state.get("pending_human_action"):
        return "request_approval"
    return "finish"


def _refund_agent(runtime: Runtime[GraphContext]) -> RefundAgent:
    return RefundAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))


def _hydrate_state(state: ConversationState, runtime: Runtime[GraphContext]) -> ConversationState:
    payload = dict(state)
    if not payload.get("current_user_id") and runtime.context.get("current_user_id"):
        payload["current_user_id"] = runtime.context["current_user_id"]
    return payload


def _normalize_preview(preview: Mapping[str, Any], order_id: str) -> dict[str, Any]:
    allow_refund = preview.get("allow_refund")
    if allow_refund is None:
        allow_refund = preview.get("allowRefund")
    return {
        "order_id": str(preview.get("order_id") or preview.get("orderId") or order_id),
        "allow_refund": bool(allow_refund),
        "refund_amount": str(preview.get("refund_amount") or preview.get("refundAmount", "待确认")),
        "refund_percent": preview.get("refund_percent") or preview.get("refundPercent", "待确认"),
        "reject_reason": str(preview.get("reject_reason") or preview.get("rejectReason") or ""),
    }


def _build_pending_refund_action(
    *,
    order_id: str,
    preview: Mapping[str, Any],
    current_user_id: int | None,
) -> dict[str, Any]:
    values = {
        "order_id": order_id,
        "reason": "用户发起退款",
        "user_id": current_user_id,
    }
    payload = build_human_approval_interrupt(
        action="refund_order",
        args={
            "orderId": order_id,
            "refundAmount": preview["refund_amount"],
            "values": values,
        },
        request={
            "title": "退款前确认",
            "description": f"订单 {order_id} 预计退款 {preview['refund_amount']}",
            "riskLevel": "medium",
            "allowedActions": ["approve", "reject", "edit"],
        },
    )
    return {
        "action": "refund_order",
        "values": values,
        "interrupt": {
            "toolName": payload.tool_name,
            "args": dict(payload.args),
            "request": dict(payload.request),
        },
    }


def _normalize_decision(decision: Any) -> dict[str, Any]:
    if not isinstance(decision, Mapping):
        return {"action": "reject", "reason": "invalid_resume_payload", "values": {}}
    if isinstance(decision.get("decisions"), list) and decision["decisions"]:
        first = decision["decisions"][0]
        if not isinstance(first, Mapping):
            return {"action": "reject", "reason": "invalid_resume_payload", "values": {}}
        decision_type = str(first.get("type") or "").strip()
        if decision_type == "approve":
            return {"action": "approve", "reason": first.get("message"), "values": {}}
        if decision_type == "reject":
            return {"action": "reject", "reason": first.get("message"), "values": {}}
        if decision_type == "edit":
            edited_action = first.get("edited_action")
            if not isinstance(edited_action, Mapping):
                return {"action": "reject", "reason": "invalid_resume_payload", "values": {}}
            args = edited_action.get("args")
            return {
                "action": "edit",
                "reason": first.get("message"),
                "values": dict(args) if isinstance(args, Mapping) else {},
            }
        return {"action": "reject", "reason": "invalid_resume_payload", "values": {}}
    action = str(decision.get("action") or "").strip()
    if action not in {"approve", "reject", "edit"}:
        action = "reject"
    values = decision.get("values")
    return {
        "action": action,
        "reason": decision.get("reason"),
        "values": dict(values) if isinstance(values, Mapping) else {},
    }


def _finish_payload(*, state: ConversationState, reply: str, result_summary: str) -> dict[str, Any]:
    return {
        "current_agent": "refund",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)] if reply else [],
        "specialist_result": {
            "agent": "refund",
            "completed": True,
            "need_handoff": False,
            "result_summary": result_summary,
        },
        "selected_order_id": state.get("selected_order_id"),
        "need_handoff": False,
    }
