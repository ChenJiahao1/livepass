def test_new_agent_runtime_packages_importable():
    import app.agents  # noqa: F401
    import app.llm  # noqa: F401
    import app.mcp_client  # noqa: F401
    import app.mcp_server  # noqa: F401
    import app.rpc  # noqa: F401
