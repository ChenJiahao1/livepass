package logic

import (
	"context"
	"testing"

	"damai-go/services/order-rpc/internal/svc"
	programrpc "damai-go/services/program-rpc/programrpc"

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

func TestCompensateOrderCreateSendFailureDoesNotReleaseSeatFreeze(t *testing.T) {
	programRPC := &noopOrderCreateProgramRPC{}

	compensateOrderCreateSendFailure(context.Background(), &svc.ServiceContext{
		ProgramRpc: programRPC,
	}, 3001, 10001, 9001, "freeze-send-failed")

	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected send failure compensation to keep seat freeze, got %d release calls", programRPC.releaseSeatFreezeCalls)
	}
	if programRPC.lastReleaseReq != nil {
		t.Fatalf("expected no release request, got %+v", programRPC.lastReleaseReq)
	}
}
