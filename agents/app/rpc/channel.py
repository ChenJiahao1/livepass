"""Shared gRPC channel helpers."""

import grpc


def create_rpc_channel(target: str) -> grpc.aio.Channel:
    return grpc.aio.insecure_channel(target)
