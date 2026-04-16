from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(slots=True)
class LocalHumanTool:
    name: str

    async def ainvoke(self, payload: dict[str, Any]) -> dict[str, Any]:
        return {"toolName": self.name, "arguments": dict(payload)}


human_approval = LocalHumanTool(name="human_approval")
human_input = LocalHumanTool(name="human_input")
