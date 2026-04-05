from app.orchestrator.parent_agent import ParentAgent


def test_parent_agent_creates_order_list_task_for_recent_refund_request():
    agent = ParentAgent()

    task = agent.build_task(
        user_message="帮我退最近那单",
        session_state={"user_id": 3001},
        llm=None,
    )

    assert task.task_type == "order_list_recent"
    assert task.skill_id == "order.list_recent"
    assert task.input_slots["user_id"] == 3001
