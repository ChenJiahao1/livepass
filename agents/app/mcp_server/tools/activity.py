"""MCP tools for activity/program queries."""

from langchain_core.tools import StructuredTool


def build_page_programs_tool(program_client):
    async def page_programs(page_number: int = 1, page_size: int = 10):
        payload = await program_client.page_programs(page_number=page_number, page_size=page_size)
        return {
            "programs": [
                {
                    "program_id": str(item["id"]),
                    "title": item["title"],
                    "show_time": item["showTime"],
                }
                for item in payload.get("list", [])
            ]
        }

    return StructuredTool.from_function(
        coroutine=page_programs,
        name="page_programs",
        description="分页查询节目列表",
    )


def build_get_program_detail_tool(program_client):
    async def get_program_detail(program_id: str):
        payload = await program_client.get_program_detail(program_id=program_id)
        return {
            "program_id": str(payload["id"]),
            "title": payload["title"],
            "show_time": payload["showTime"],
            "place": payload["place"],
        }

    return StructuredTool.from_function(
        coroutine=get_program_detail,
        name="get_program_detail",
        description="查询节目详情",
    )


def build_activity_tools(program_client):
    return [
        build_page_programs_tool(program_client),
        build_get_program_detail_tool(program_client),
    ]
