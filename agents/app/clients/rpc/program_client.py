"""Thin async wrapper around program-rpc stubs."""

from app.clients.rpc import generated
from app.clients.rpc.channel import create_rpc_channel


class ProgramRpcClient:
    def __init__(self, *, target: str, timeout_seconds: float = 5.0) -> None:
        self.timeout_seconds = timeout_seconds
        self.channel = create_rpc_channel(target)
        self.stub = generated.program_pb2_grpc.ProgramRpcStub(self.channel)

    async def page_programs(self, *, page_number: int = 1, page_size: int = 10):
        request = generated.program_pb2.PageProgramsReq(
            pageNumber=page_number,
            pageSize=page_size,
        )
        response = await self.stub.PagePrograms(request, timeout=self.timeout_seconds)
        return {
            "pageNum": response.pageNum,
            "pageSize": response.pageSize,
            "totalSize": response.totalSize,
            "list": [
                {
                    "id": item.id,
                    "title": item.title,
                    "showTime": item.showTime,
                }
                for item in response.list
            ],
        }

    async def get_program_detail(self, *, program_id: int):
        request = generated.program_pb2.GetProgramDetailReq(id=program_id)
        response = await self.stub.GetProgramDetail(request, timeout=self.timeout_seconds)
        return {
            "id": response.id,
            "title": response.title,
            "showTime": response.showTime,
            "place": response.place,
        }
