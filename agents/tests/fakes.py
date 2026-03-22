class ScriptedChatModel:
    def __init__(self, *, structured_responses: list[dict] | None = None, responses: list[str] | None = None):
        self.structured_responses = list(structured_responses or [])
        self.responses = list(responses or [])

    def with_structured_output(self, schema):
        return _StructuredScriptedChatModel(self, schema)


class _StructuredScriptedChatModel:
    def __init__(self, parent: ScriptedChatModel, schema):
        self._parent = parent
        self._schema = schema

    def invoke(self, _messages):
        payload = self._parent.structured_responses.pop(0)
        return self._schema.model_validate(payload)
