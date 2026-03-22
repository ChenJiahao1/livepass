from app.prompts import PromptRenderer


def test_prompts_include_customer_style_routing_rules():
    renderer = PromptRenderer()

    coordinator_prompt = renderer.render(
        "coordinator/system.md",
        selected_order_id=None,
        last_intent="unknown",
        current_user_id=None,
    )
    supervisor_prompt = renderer.render(
        "supervisor/system.md",
        selected_order_id=None,
        route=None,
        specialist_result=None,
        current_user_id=None,
    )

    assert "明星基础百科" in coordinator_prompt
    assert "knowledge" in supervisor_prompt
    assert "json" in supervisor_prompt.lower()
