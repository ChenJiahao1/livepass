import pytest

from app.agents.handoff import HandoffAgent
from tests.fakes import StubRegistry


@pytest.mark.anyio
async def test_handoff_specialist_sets_need_handoff_with_todo_reply():
    registry = StubRegistry()

    result = await HandoffAgent(registry=registry, llm=object()).handle(
        {"messages": [{"role": "user", "content": "我要人工"}]}
    )

    assert result["need_handoff"] is True
    assert result["status"] == "handoff_todo"
    assert "TODO" in result["reply"]
