from pathlib import Path

from app.shared.config import get_settings


def test_shared_and_integrations_modules_exist():
    assert Path("app/shared/config.py").is_file()
    assert Path("app/shared/errors.py").is_file()
    assert Path("app/shared/ids.py").is_file()
    assert Path("app/shared/cursor.py").is_file()
    assert Path("app/integrations/mcp/registry.py").is_file()
    assert Path("app/integrations/mcp/interceptor.py").is_file()
    assert Path("app/integrations/mcp/execution_context.py").is_file()
    assert Path("app/integrations/knowledge/service.py").is_file()
    assert Path("app/integrations/storage/mysql.py").is_file()
    assert Path("app/integrations/storage/redis.py").is_file()
    assert Path("app/agents/llm.py").is_file()
    assert not Path("app/config.py").exists()
    assert not Path("app/common/errors.py").exists()
    assert not Path("app/llm/client.py").exists()


def test_settings_exposes_customer_runtime_fields(monkeypatch):
    monkeypatch.setenv("OPENAI_MODEL", "gpt-4.1-mini")
    get_settings.cache_clear()

    settings = get_settings()

    assert settings.max_tool_steps == 3
    assert settings.lightrag_base_url == "http://127.0.0.1:9621"
    assert settings.checkpoint_key_prefix == "agents:langgraph"

    get_settings.cache_clear()


def test_settings_default_mcp_endpoints_match_local_service_ports():
    get_settings.cache_clear()

    settings = get_settings()

    assert settings.activity_mcp_endpoint == "http://127.0.0.1:9083/message"
    assert settings.order_mcp_endpoint == "http://127.0.0.1:9082/message"

    get_settings.cache_clear()


def test_agents_mysql_defaults_are_configured(monkeypatch, tmp_path):
    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("AGENTS_MYSQL_HOST", raising=False)
    get_settings.cache_clear()

    settings = get_settings()

    assert settings.agents_mysql_host == "127.0.0.1"
    assert settings.agents_mysql_port == 3306
    assert settings.agents_mysql_database == "livepass_agents"
    assert settings.agents_thread_default_title == "新会话"

    get_settings.cache_clear()
