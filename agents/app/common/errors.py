from __future__ import annotations

from fastapi import HTTPException


class ApiErrorCode:
    UNAUTHORIZED = "UNAUTHORIZED"
    FORBIDDEN = "FORBIDDEN"
    THREAD_NOT_FOUND = "THREAD_NOT_FOUND"
    MESSAGE_NOT_FOUND = "MESSAGE_NOT_FOUND"
    RUN_NOT_FOUND = "RUN_NOT_FOUND"
    TOOL_CALL_NOT_FOUND = "TOOL_CALL_NOT_FOUND"
    ACTIVE_RUN_EXISTS = "ACTIVE_RUN_EXISTS"
    TOOL_CALL_NOT_WAITING_HUMAN = "TOOL_CALL_NOT_WAITING_HUMAN"
    TOOL_CALL_DECISION_NOT_ALLOWED = "TOOL_CALL_DECISION_NOT_ALLOWED"
    VALIDATION_ERROR = "VALIDATION_ERROR"
    LANGGRAPH_RUNTIME_ERROR = "LANGGRAPH_RUNTIME_ERROR"
    UPSTREAM_TOOL_ERROR = "UPSTREAM_TOOL_ERROR"
    RUN_CANCELLED = "RUN_CANCELLED"
    RUN_NOT_ACTIVE = "RUN_NOT_ACTIVE"
    INTERNAL_ERROR = "INTERNAL_ERROR"


class ApiError(Exception):
    def __init__(
        self,
        *,
        code: str,
        message: str,
        http_status: int,
        details: dict | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.http_status = http_status
        self.details = details or {}


def to_http_exception(error: ApiError) -> HTTPException:
    return HTTPException(
        status_code=error.http_status,
        detail={
            "error": {
                "code": error.code,
                "message": error.message,
                "details": error.details,
            }
        },
    )
