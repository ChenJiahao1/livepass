from __future__ import annotations

from typing import Any

from app.runs.interrupt_models import HumanInterruptPayload


def build_human_approval_interrupt(
    *,
    action: str,
    args: dict[str, Any],
    request: dict[str, Any],
) -> HumanInterruptPayload:
    return HumanInterruptPayload(
        tool_name="human_approval",
        action=action,
        args={**args, "action": action},
        request=dict(request),
    )


def build_human_input_interrupt(
    *,
    action: str,
    args: dict[str, Any],
    request: dict[str, Any],
) -> HumanInterruptPayload:
    return HumanInterruptPayload(
        tool_name="human_input",
        action=action,
        args={**args, "action": action},
        request=dict(request),
    )
