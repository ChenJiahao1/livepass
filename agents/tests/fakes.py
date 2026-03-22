from langchain_core.tools import StructuredTool


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


class StubRegistry:
    def __init__(self, *, tools_by_toolset: dict[str, list] | None = None):
        self.tools_by_toolset = tools_by_toolset or {}

    async def get_tools(self, toolset: str) -> list:
        return list(self.tools_by_toolset.get(toolset, []))


def build_async_tool(*, name: str, description: str, coroutine):
    return StructuredTool.from_function(
        coroutine=coroutine,
        name=name,
        description=description,
    )
