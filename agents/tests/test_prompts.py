from pathlib import Path

from app.llm.schemas import CoordinatorDecision, SupervisorDecision
from app.shared.prompt_loader import PromptLoader


def test_prompt_files_align_with_role_filenames():
    prompt_dir = Path("prompts")
    expected_names = {
        "coordinator.md",
        "supervisor.md",
        "activity_specialist.md",
        "order_specialist.md",
        "refund_specialist.md",
        "handoff_specialist.md",
        "knowledge_specialist.md",
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


def test_supervisor_decision_schema_accepts_finish():
    decision = SupervisorDecision.model_validate({"next_agent": "finish", "need_handoff": False})
    assert decision.next_agent == "finish"


def test_coordinator_decision_schema_accepts_delegate():
    decision = CoordinatorDecision.model_validate(
        {
            "action": "delegate",
            "reply": "",
            "selected_order_id": None,
            "business_ready": True,
            "reason": "user asked to check orders",
        }
    )
    assert decision.action == "delegate"
