import asyncio

from app.tools.activity import build_activity_tools


class FakeProgramClient:
    def __init__(self):
        self.calls: list[tuple[str, dict[str, int]]] = []

    async def page_programs(self, *, page_number: int = 1, page_size: int = 10):
        self.calls.append(
            (
                "page_programs",
                {"page_number": page_number, "page_size": page_size},
            )
        )
        return {
            "list": [
                {"id": 2001, "title": "银河剧场", "showTime": "2026-04-01 19:30:00"},
            ]
        }

    async def get_program_detail(self, *, program_id: int):
        self.calls.append(("get_program_detail", {"program_id": program_id}))
        return {
            "id": program_id,
            "title": "银河剧场",
            "showTime": "2026-04-01 19:30:00",
        }


def _tool_by_name(tools, name: str):
    return next(tool for tool in tools if tool.name == name)


def test_activity_tools_call_page_programs_and_get_program_detail():
    client = FakeProgramClient()
    tools = build_activity_tools(client)

    page_result = asyncio.run(_tool_by_name(tools, "page_programs").ainvoke({}))
    detail_result = asyncio.run(
        _tool_by_name(tools, "get_program_detail").ainvoke({"program_id": 2001})
    )

    assert page_result["list"][0]["id"] == 2001
    assert detail_result["id"] == 2001
    assert client.calls == [
        ("page_programs", {"page_number": 1, "page_size": 10}),
        ("get_program_detail", {"program_id": 2001}),
    ]
