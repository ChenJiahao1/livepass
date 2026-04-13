package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/server"
	"damai-go/services/order-rpc/pb"
)

func TestPrimeAdmissionQuotaUsesInternalAdmissionQuota(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	_, showTimeID, ticketCategoryID, _, _ := nextRushTestIDs()
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(showTimeID, ticketCategoryID, 2, 4, 299)
	programRPC.getProgramPreorderResp.TicketCategoryVoList[0].RemainNumber = 1
	programRPC.getProgramPreorderResp.TicketCategoryVoList[0].AdmissionQuota = 7

	if err := logicpkg.PrimeAdmissionQuota(context.Background(), svcCtx, showTimeID); err != nil {
		t.Fatalf("PrimeAdmissionQuota() error = %v", err)
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

	_, showTimeID, ticketCategoryID, _, _ := nextRushTestIDs()
	programRPC.getProgramPreorderResp = buildTestProgramPreorder(showTimeID, ticketCategoryID, 2, 4, 299)
	programRPC.getProgramPreorderResp.TicketCategoryVoList[0].RemainNumber = 1
	programRPC.getProgramPreorderResp.TicketCategoryVoList[0].AdmissionQuota = 9

	resp, err := server.NewOrderRpcServer(svcCtx).PrimeAdmissionQuota(context.Background(), &pb.PrimeAdmissionQuotaReq{
		ShowTimeId: showTimeID,
	})
	if err != nil {
		t.Fatalf("PrimeAdmissionQuota RPC error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected PrimeAdmissionQuota RPC success, got %+v", resp)
	}

	available, ok, err := svcCtx.AttemptStore.GetQuotaAvailable(context.Background(), showTimeID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 9 {
		t.Fatalf("expected RPC preheated quota=9 from admission quota, got ok=%t available=%d", ok, available)
	}
}
