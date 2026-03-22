"""Thin async wrapper around user-rpc stubs."""

from app.clients.rpc import generated
from app.clients.rpc.channel import create_rpc_channel


class UserRpcClient:
    def __init__(self, *, target: str, timeout_seconds: float = 5.0) -> None:
        self.timeout_seconds = timeout_seconds
        self.channel = create_rpc_channel(target)
        self.stub = generated.user_pb2_grpc.UserRpcStub(self.channel)

    async def get_user_by_id(self, *, user_id: int):
        request = generated.user_pb2.GetUserByIdReq(id=user_id)
        response = await self.stub.GetUserById(request, timeout=self.timeout_seconds)
        return {
            "id": response.id,
            "name": response.name,
            "mobile": response.mobile,
        }
