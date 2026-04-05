"""Audit event helpers for orchestrator execution."""

from __future__ import annotations

from typing import Any


def build_audit_record(*, result: dict[str, Any]) -> dict[str, Any]:
    return {
        "task_trace": result.get("task_trace", []),
        "need_handoff": result.get("need_handoff", False),
        "final_reply": result.get("final_reply") or result.get("reply", ""),
    }

