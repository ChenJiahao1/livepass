from __future__ import annotations

import base64
import json


def encode_cursor(payload: dict[str, str]) -> str:
    raw = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")


def decode_cursor(cursor: str) -> dict[str, str]:
    padding = "=" * (-len(cursor) % 4)
    raw = base64.urlsafe_b64decode(f"{cursor}{padding}".encode("ascii"))
    payload = json.loads(raw.decode("utf-8"))
    if not isinstance(payload, dict):
        raise ValueError("cursor payload must be an object")
    return {str(key): str(value) for key, value in payload.items()}
