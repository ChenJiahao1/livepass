package logic

import (
	"context"
	"testing"

	"livepass/services/order-rpc/internal/svc"
	programrpc "livepass/services/program-rpc/programrpc"

	"google.golang.org/grpc"
)

type noopOrderCreateProgramRPC struct {
	programrpc.ProgramRpc
	releaseSeatFreezeCalls int
	lastReleaseReq         *programrpc.ReleaseSeatFreezeReq
}

func (f *noopOrderCreateProgramRPC) ReleaseSeatFreeze(ctx context.Context, in *programrpc.ReleaseSeatFreezeReq, opts ...grpc.CallOption) (*programrpc.ReleaseSeatFreezeResp, error) {
	f.releaseSeatFreezeCalls++
	f.lastReleaseReq = in
	return &programrpc.ReleaseSeatFreezeResp{Success: true}, nil
}

func TestReleaseOrderCreateFreezeSendsFreezeTokenAndReason(t *testing.T) {
	programRPC := &noopOrderCreateProgramRPC{}

	releaseOrderCreateFreeze(context.Background(), &svc.ServiceContext{
		ProgramRpc: programRPC,
	}, "freeze-owner-release", "worker_release")

	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one release request, got %d", programRPC.releaseSeatFreezeCalls)
	}
	if programRPC.lastReleaseReq == nil {
		t.Fatalf("expected release request to be captured")
	}
	if programRPC.lastReleaseReq.GetFreezeToken() != "freeze-owner-release" || programRPC.lastReleaseReq.GetReleaseReason() != "worker_release" {
		t.Fatalf("unexpected release request %+v", programRPC.lastReleaseReq)
	}
}
