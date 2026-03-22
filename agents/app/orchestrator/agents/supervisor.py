"""Supervisor that maps intent to specialist agent names."""


class SupervisorAgent:
    def next_agent(self, intent: str) -> str:
        if intent in {"activity", "order", "refund", "handoff"}:
            return intent
        return "handoff"
