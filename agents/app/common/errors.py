from __future__ import annotations

from fastapi import HTTPException


class ApiErrorCode:
    UNAUTHORIZED = "UNAUTHORIZED"
    FORBIDDEN = "FORBIDDEN"
    THREAD_NOT_FOUND = "THREAD_NOT_FOUND"
    MESSAGE_NOT_FOUND = "MESSAGE_NOT_FOUND"
    RUN_NOT_FOUND = "RUN_NOT_FOUND"
    TOOL_CALL_NOT_FOUND = "TOOL_CALL_NOT_FOUND"
    RUN_STATE_INVALID = "RUN_STATE_INVALID"
    VALIDATION_ERROR = "VALIDATION_ERROR"
    AGENT_RUN_FAILED = "AGENT_RUN_FAILED"
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
