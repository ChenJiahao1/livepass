from pathlib import Path


def test_readme_mentions_mcp_server_and_fastapi_entrypoints():
    readme = Path("README.md").read_text(encoding="utf-8")

    assert "damai-mcp-server" in readme
    assert "/agent/chat" in readme
