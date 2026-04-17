import asyncio
from pathlib import Path

import httpx
from langchain_core.messages import HumanMessage

from app.agents.specialists.knowledge_specialist import KnowledgeAgent
from app.shared.config import get_settings
from app.integrations.knowledge.service import KnowledgeService


def _set_lightrag_env(monkeypatch, tmp_path: Path, *, api_key: str | None = "test-rag-key"):
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("LIGHTRAG_BASE_URL", "http://127.0.0.1:9621")
    monkeypatch.setenv("LIGHTRAG_TIMEOUT_SECONDS", "45")
    if api_key is None:
        monkeypatch.delenv("LIGHTRAG_API_KEY", raising=False)
    else:
        monkeypatch.setenv("LIGHTRAG_API_KEY", api_key)
    get_settings.cache_clear()


def test_knowledge_service_returns_lightrag_response(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path)
    requests = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(
            200,
            json={"response": "周杰伦是中国台湾男歌手、音乐人、演员。"},
        )

    agent = KnowledgeService(http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]}))

    assert result["agent"] == "knowledge"
    assert "周杰伦" in result["reply"]
    assert result["trace"] == ["knowledge:lightrag"]


def test_knowledge_service_returns_boundary_message_for_realtime_query(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path)
    requests = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(200, json={"response": "should not happen"})

    agent = KnowledgeService(http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦最近有什么新闻")]}))

    assert "基础百科" in result["reply"]
    assert result["trace"] == ["knowledge:out_of_scope"]
    assert requests == []


def test_knowledge_service_returns_config_error_when_api_key_missing(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path, api_key=None)
    requests = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(200, json={"response": "should not happen"})

    agent = KnowledgeService(http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]}))

    assert "API Key" in result["reply"]
    assert result["trace"] == ["knowledge:config_error"]
    assert requests == []


def test_knowledge_service_returns_fallback_for_timeout(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path)

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout", request=request)

    agent = KnowledgeService(http_client=httpx.AsyncClient(transport=httpx.MockTransport(handler)))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]}))

    assert "稍后重试" in result["reply"]
    assert result["trace"] == ["knowledge:lightrag_timeout"]


def test_knowledge_service_returns_fallback_for_bad_json(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path)
    transport = httpx.MockTransport(lambda request: httpx.Response(200, text="not json"))
    agent = KnowledgeService(http_client=httpx.AsyncClient(transport=transport))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]}))

    assert "稍后重试" in result["reply"]
    assert result["trace"] == ["knowledge:lightrag_bad_json"]


def test_knowledge_service_returns_fallback_for_empty_response(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path)
    transport = httpx.MockTransport(lambda request: httpx.Response(200, json={"response": "   "}))
    agent = KnowledgeService(http_client=httpx.AsyncClient(transport=transport))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]}))

    assert "稍后重试" in result["reply"]
    assert result["trace"] == ["knowledge:lightrag_empty"]


def test_knowledge_agent_wraps_service_result(monkeypatch, tmp_path: Path):
    _set_lightrag_env(monkeypatch, tmp_path)
    transport = httpx.MockTransport(
        lambda request: httpx.Response(200, json={"response": "周杰伦是华语流行音乐代表人物。"})
    )
    agent = KnowledgeAgent(service=KnowledgeService(http_client=httpx.AsyncClient(transport=transport)))

    result = asyncio.run(agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]}))

    assert result["agent"] == "knowledge"
    assert "周杰伦" in result["reply"]
