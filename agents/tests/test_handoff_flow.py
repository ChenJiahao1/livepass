import pytest

from app.agents.handoff import HandoffAgent
from tests.fakes import StubRegistry, build_async_tool


@pytest.mark.anyio
async def test_handoff_specialist_sets_need_handoff():
    async def _request_handoff(reason: str):
        return {"ticket_id": "HOF-1", "queued": True, "reason": reason}

    registry = StubRegistry(
        tools_by_toolset={
            "handoff": [
                build_async_tool(
                    name="request_handoff",
                    description="request handoff",
                    coroutine=_request_handoff,
                )
            ]
        }
    )

    result = await HandoffAgent(registry=registry, llm=object()).handle(
        {"messages": [{"role": "user", "content": "我要人工"}]}
    )

    assert result["need_handoff"] is True
