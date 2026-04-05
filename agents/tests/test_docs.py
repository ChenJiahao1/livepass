from pathlib import Path


def test_docs_describe_go_order_provider_and_python_handoff_provider():
    readme = Path("README.md").read_text(encoding="utf-8")

    assert "/agent/chat" in readme
    assert "Go `order` MCP provider" in readme
    assert "Python `handoff` provider" in readme
    assert "ORDER_MCP_ENDPOINT" in readme
