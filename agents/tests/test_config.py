from app.config import get_settings


def test_settings_exposes_customer_runtime_fields(monkeypatch):
    monkeypatch.setenv("OPENAI_MODEL", "gpt-4.1-mini")
    get_settings.cache_clear()

    settings = get_settings()

    assert settings.max_tool_steps == 3
    assert settings.lightrag_base_url == "http://127.0.0.1:9621"
    assert settings.checkpoint_key_prefix == "agents:langgraph"

    get_settings.cache_clear()


def test_settings_default_rpc_targets_match_local_service_ports():
    get_settings.cache_clear()

    settings = get_settings()

    assert settings.user_rpc_target == "127.0.0.1:8080"
    assert settings.program_rpc_target == "127.0.0.1:8083"
    assert settings.order_rpc_target == "127.0.0.1:8082"

    get_settings.cache_clear()
