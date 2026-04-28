from pathlib import Path

from app.agents.llm import CoordinatorDecision, SupervisorDecision
from app.shared.prompt_loader import PromptLoader


def test_prompt_files_align_with_role_filenames():
    prompt_dir = Path("prompts")
    expected_names = {
        "coordinator.md",
        "supervisor.md",
        "activity_specialist.md",
        "order_specialist.md",
    }

    assert {path.name for path in prompt_dir.glob("*.md")} == expected_names
    assert not list(prompt_dir.glob("*/system.md"))


def test_prompt_loader_loads_coordinator_template():
    loader = PromptLoader()
    prompt = loader.render(
        "coordinator",
        selected_order_id=None,
        last_intent="unknown",
        current_user_id=1001,
    )
    assert "coordinator" in prompt.lower()


def test_coordinator_prompt_uses_delegate_without_business_ready():
    prompt = PromptLoader().render(
        "coordinator",
        selected_order_id=None,
        last_intent="unknown",
        current_user_id=3001,
    )

    assert "business_ready" not in prompt
    assert "delegated" not in prompt
    assert '"delegate"' in prompt
    assert "缺订单号不阻止" in prompt


def test_order_specialist_prompt_mentions_autonomous_tool_usage():
    content = PromptLoader().render(
        "order_specialist",
        selected_order_id=None,
        current_user_id=1001,
    )

    assert "不要编造订单" in content
    assert "缺少事实时优先调用工具确认" in content
    assert "写操作工具在真正执行前会被人工确认" in content


def test_supervisor_decision_schema_accepts_finish():
    decision = SupervisorDecision.model_validate({"next_agent": "finish"})
    assert decision.next_agent == "finish"


def test_coordinator_decision_schema_accepts_delegate():
    decision = CoordinatorDecision.model_validate(
        {
            "action": "delegate",
            "reply": "",
            "route": "order",
            "selected_order_id": None,
            "selected_program_id": None,
            "reason": "user asked to check orders",
        }
    )
    assert decision.action == "delegate"
