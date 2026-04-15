from app.llm.schemas import CoordinatorDecision, SupervisorDecision
from app.prompts import PromptRenderer


def test_prompt_renderer_loads_coordinator_template():
    renderer = PromptRenderer()
    prompt = renderer.render(
        "coordinator/system.md",
        selected_order_id=None,
        last_intent="unknown",
        current_user_id="1001",
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
