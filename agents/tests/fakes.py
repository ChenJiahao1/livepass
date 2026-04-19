import inspect
from functools import wraps
from typing import Any

from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, BaseMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from langchain_core.runnables import RunnableLambda
from langchain_core.tools import StructuredTool
from pydantic import Field
from app.shared.runtime_constants import AGENT_ORDER


class ScriptedChatModel(BaseChatModel):
    responses: list[AIMessage | str] = Field(default_factory=list)
    structured_responses: list[Any] = Field(default_factory=list)
    calls: list[list[BaseMessage]] = Field(default_factory=list)
    structured_calls: list[Any] = Field(default_factory=list)
    bound_tool_names: list[list[str]] = Field(default_factory=list)
    _response_index: int = 0
    _structured_index: int = 0

    @property
    def _llm_type(self) -> str:
        return "scripted-chat-model"

    @property
    def _identifying_params(self) -> dict[str, Any]:
        return {}

    def _generate(
        self,
        messages: list[BaseMessage],
        stop: list[str] | None = None,
        run_manager=None,
        **kwargs: Any,
    ) -> ChatResult:
        self.calls.append(list(messages))
        response = self.responses[self._response_index]
        self._response_index += 1
        if not isinstance(response, AIMessage):
            response = AIMessage(content=str(response))
        return ChatResult(generations=[ChatGeneration(message=response)])

    def bind_tools(self, tools, *, tool_choice: str | None = None, **kwargs: Any):
        self.bound_tool_names.append([self._tool_name(tool) for tool in tools])
        return self

    def with_structured_output(
        self,
        schema,
        *,
        include_raw: bool = False,
        **kwargs: Any,
    ):
        def _invoke(messages: Any):
            self.structured_calls.append(messages)
            response = self.structured_responses[self._structured_index]
            self._structured_index += 1
            parsed = response
            if isinstance(schema, type) and hasattr(schema, "model_validate"):
                parsed = schema.model_validate(response)
            if include_raw:
                return {
                    "raw": AIMessage(content=str(response)),
                    "parsed": parsed,
                    "parsing_error": None,
                }
            return parsed

        return RunnableLambda(_invoke)

    def _tool_name(self, tool: Any) -> str:
        if hasattr(tool, "name"):
            return str(tool.name)
        if hasattr(tool, "__name__"):
            return str(tool.__name__)
        if isinstance(tool, dict):
            if "name" in tool:
                return str(tool["name"])
            if "function" in tool and isinstance(tool["function"], dict):
                return str(tool["function"].get("name", "unknown"))
        return "unknown"


class StubRegistry:
    def __init__(self, *, tools_by_toolset: dict[str, list] | None = None):
        self.tools_by_toolset = tools_by_toolset or {}

    async def get_tools(self, toolset: str) -> list:
        return list(self.tools_by_toolset.get(toolset, []))

    async def get_provider_tools(self, server_name: str) -> list:
        if server_name == AGENT_ORDER:
            combined = [
                *self.tools_by_toolset.get(AGENT_ORDER, []),
                *self.tools_by_toolset.get("refund", []),
            ]
            deduped: dict[str, Any] = {}
            for tool in combined:
                deduped[getattr(tool, "name", str(tool))] = tool
            return list(deduped.values())
        return list(self.tools_by_toolset.get(server_name, []))

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict[str, Any]):
        tools = await self.get_provider_tools(server_name)
        tool_by_name = {getattr(tool, "name", ""): tool for tool in tools}
        tool = tool_by_name[tool_name]
        allowed_args = payload
        args_schema = getattr(tool, "args_schema", None)
        if args_schema is not None and hasattr(args_schema, "model_fields"):
            allowed_keys = set(args_schema.model_fields.keys())
            allowed_args = {key: value for key, value in payload.items() if key in allowed_keys}
        return await tool.ainvoke(allowed_args)


def build_async_tool(*, name: str, description: str, coroutine):
    @wraps(coroutine)
    async def _wrapped(*args, **kwargs):
        return await coroutine(*args, **kwargs)

    _wrapped.__signature__ = inspect.signature(coroutine)
    return StructuredTool.from_function(
        coroutine=_wrapped,
        name=name,
        description=description,
    )


class FakeRedis:
    def __init__(self):
        self.values: dict[str, str] = {}
        self.hashes: dict[str, dict[str, str]] = {}
        self.expire_calls: list[tuple[str, int]] = []

    def get(self, key: str):
        return self.values.get(key)

    def set(self, key: str, value: str):
        self.values[key] = value
        return True

    def hget(self, key: str, field: str):
        return self.hashes.get(key, {}).get(field)

    def hset(self, key: str, field: str, value: str):
        self.hashes.setdefault(key, {})[field] = value
        return 1

    def hgetall(self, key: str):
        return dict(self.hashes.get(key, {}))

    def hkeys(self, key: str):
        return list(self.hashes.get(key, {}).keys())

    def expire(self, key: str, ttl_seconds: int):
        self.expire_calls.append((key, ttl_seconds))
        return True

    def scan_iter(self, match: str | None = None):
        if match is None:
            for key in [*self.values.keys(), *self.hashes.keys()]:
                yield key
            return

        prefix = match.rstrip("*")
        for key in [*self.values.keys(), *self.hashes.keys()]:
            if key.startswith(prefix):
                yield key

    def delete(self, *keys: str):
        deleted = 0
        for key in keys:
            if key in self.values:
                del self.values[key]
                deleted += 1
            if key in self.hashes:
                del self.hashes[key]
                deleted += 1
        return deleted
