package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/server"
	"damai-go/services/order-rpc/pb"
	programrpc "damai-go/services/program-rpc/programrpc"
)

func TestPrimeAdmissionQuotaUsesInternalAdmissionQuota(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programID, showTimeID, ticketCategoryID, _, _ := nextRushTestIDs()
	programRPC.listProgramShowTimesForRushResp = &programrpc.ListProgramShowTimesForRushResp{
		List: []*programrpc.ProgramShowTimeForRushInfo{{ShowTimeId: showTimeID}},
	}
	programRPC.getProgramPreorderRespByProgramID = map[int64]*programrpc.ProgramPreorderInfo{
		showTimeID: func() *programrpc.ProgramPreorderInfo {
			resp := buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
			resp.ShowTimeId = showTimeID
			resp.TicketCategoryVoList[0].RemainNumber = 1
			resp.TicketCategoryVoList[0].AdmissionQuota = 7
			return resp
		}(),
	}

	if err := logicpkg.PrimeRushRuntime(context.Background(), svcCtx, programID); err != nil {
		t.Fatalf("PrimeRushRuntime() error = %v", err)
	}
	if programRPC.lastGetProgramPreorderReq == nil || programRPC.lastGetProgramPreorderReq.ShowTimeId != showTimeID {
		t.Fatalf("expected GetProgramPreorder to load showTimeId=%d, got %+v", showTimeID, programRPC.lastGetProgramPreorderReq)
	}

	available, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(context.Background(), showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 7 {
		t.Fatalf("expected preheated quota=7 from admission quota, got ok=%t available=%d", ok, available)
	}
}

func TestPrimeAdmissionQuotaRPCUsesInternalAdmissionQuota(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	programID, showTimeID, ticketCategoryID, _, _ := nextRushTestIDs()
	programRPC.listProgramShowTimesForRushResp = &programrpc.ListProgramShowTimesForRushResp{
		List: []*programrpc.ProgramShowTimeForRushInfo{{ShowTimeId: showTimeID}},
	}
	programRPC.getProgramPreorderRespByProgramID = map[int64]*programrpc.ProgramPreorderInfo{
		showTimeID: func() *programrpc.ProgramPreorderInfo {
			resp := buildTestProgramPreorder(programID, ticketCategoryID, 2, 4, 299)
			resp.ShowTimeId = showTimeID
			resp.TicketCategoryVoList[0].RemainNumber = 1
			resp.TicketCategoryVoList[0].AdmissionQuota = 9
			return resp
		}(),
	}

	resp, err := server.NewOrderRpcServer(svcCtx).PrimeRushRuntime(context.Background(), &pb.PrimeRushRuntimeReq{
		ProgramId: programID,
	})
	if err != nil {
		t.Fatalf("PrimeRushRuntime RPC error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected PrimeRushRuntime RPC success, got %+v", resp)
	}

	available, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(context.Background(), showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 9 {
		t.Fatalf("expected RPC preheated quota=9 from admission quota, got ok=%t available=%d", ok, available)
	}
}
