from pathlib import Path


def test_agents_readme_documents_thread_api():
    readme = Path("README.md").read_text(encoding="utf-8")

    assert "/agent/threads" in readme
    assert "/agent/threads/{threadId}" in readme
    assert "/agent/threads/{threadId}/messages" in readme
    assert "/agent/runs" in readme
    assert "/agent/runs/{runId}" in readme
    assert "/agent/runs/{runId}/events" in readme
    assert "/agent/runs/{runId}/tool-calls/{toolCallId}/resume" in readme
    assert "/agent/runs/{runId}/cancel" in readme
    assert "/agent/chat" not in readme
    assert "/agent/threads/{threadId}/runs/{runId}" not in readme
    assert "/agent/threads/{threadId}/messages" in readme
    assert "Thread / Message / Run" in readme
    assert "Python 3.12" in readme
    assert "LangGraph 1.1.6" in readme
    assert "uv run uvicorn app.api.app:app --reload" in readme
    assert "uv run uvicorn app.main:app --reload" not in readme
    assert "after 游标回放历史事件" in readme
    assert "input.content" in readme
    assert "outputMessageId" in readme
    assert "tool_call.waiting_human" in readme
    assert "resume / cancel 接口按同一请求做幂等处理" in readme
