"""Activity tools backed by program-rpc."""

from langchain_core.tools import StructuredTool


def build_activity_tools(program_client):
    async def page_programs(page_number: int = 1, page_size: int = 10):
        return await program_client.page_programs(page_number=page_number, page_size=page_size)

    async def get_program_detail(program_id: int):
        return await program_client.get_program_detail(program_id=program_id)

    return [
        StructuredTool.from_function(
            coroutine=page_programs,
            name="page_programs",
            description="分页查询活动列表",
        ),
        StructuredTool.from_function(
            coroutine=get_program_detail,
            name="get_program_detail",
            description="查询活动详情",
        ),
    ]
