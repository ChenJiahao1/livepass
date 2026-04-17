package integration_test

import (
	"context"
	"testing"

	logicpkg "livepass/services/program-rpc/internal/logic"
	"livepass/services/program-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestSeatFreezeOwnerFieldsAreExposedInProtoRequests(t *testing.T) {
	t.Parallel()

	assertHasField := func(message protoreflect.ProtoMessage, fieldName protoreflect.Name) {
		t.Helper()

		fields := message.ProtoReflect().Descriptor().Fields()
		if fields.ByName(fieldName) == nil {
			t.Fatalf("expected %s to contain field %q", message.ProtoReflect().Descriptor().Name(), fieldName)
		}
	}

	assertHasField(&pb.AutoAssignAndFreezeSeatsReq{}, "ownerOrderNumber")
	assertHasField(&pb.AutoAssignAndFreezeSeatsReq{}, "ownerEpoch")
	assertHasField(&pb.ConfirmSeatFreezeReq{}, "ownerOrderNumber")
	assertHasField(&pb.ConfirmSeatFreezeReq{}, "ownerEpoch")
	assertHasField(&pb.ReleaseSeatFreezeReq{}, "ownerOrderNumber")
	assertHasField(&pb.ReleaseSeatFreezeReq{}, "ownerEpoch")
}

func TestStaleEpochCannotConfirmFreeze(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const (
		programID        int64 = 54101
		ticketCategoryID int64 = 64101
		ownerOrderNumber int64 = 95101
	)

	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 79101, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 79102, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	freezeResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-owner-confirm-stale",
		FreezeSeconds:    900,
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       2,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}

	logic := logicpkg.NewConfirmSeatFreezeLogic(context.Background(), svcCtx)
	_, err = logic.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{
		FreezeToken:      freezeResp.FreezeToken,
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       1,
	})
	if err == nil {
		t.Fatalf("expected stale owner epoch confirm to fail")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition for stale owner epoch, got %s", status.Code(err))
	}

	resp, err := logic.ConfirmSeatFreeze(&pb.ConfirmSeatFreezeReq{
		FreezeToken:      freezeResp.FreezeToken,
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       2,
	})
	if err != nil {
		t.Fatalf("ConfirmSeatFreeze with current owner returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected confirm success for current owner")
	}
}

func TestStaleEpochCannotReleaseNewFreeze(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	const (
		programID        int64 = 54102
		ticketCategoryID int64 = 64102
		ownerOrderNumber int64 = 95102
	)

	seedSeatInventoryProgram(t, svcCtx, programID, ticketCategoryID)
	seedSeatFixtures(t, svcCtx,
		seatFixture{ID: 79201, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 1, SeatStatus: testSeatStatusAvailable},
		seatFixture{ID: 79202, ProgramID: programID, TicketCategoryID: ticketCategoryID, RowCode: 1, ColCode: 2, SeatStatus: testSeatStatusAvailable},
	)
	primeProgramSeatLedgerFromDB(t, svcCtx, programID, ticketCategoryID)

	freezeResp, err := logicpkg.NewAutoAssignAndFreezeSeatsLogic(context.Background(), svcCtx).AutoAssignAndFreezeSeats(&pb.AutoAssignAndFreezeSeatsReq{
		ShowTimeId:       programID,
		TicketCategoryId: ticketCategoryID,
		Count:            2,
		RequestNo:        "req-owner-release-stale",
		FreezeSeconds:    900,
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       3,
	})
	if err != nil {
		t.Fatalf("AutoAssignAndFreezeSeats returned error: %v", err)
	}

	releaseLogic := logicpkg.NewReleaseSeatFreezeLogic(context.Background(), svcCtx)
	_, err = releaseLogic.ReleaseSeatFreeze(&pb.ReleaseSeatFreezeReq{
		FreezeToken:      freezeResp.FreezeToken,
		ReleaseReason:    "stale epoch release",
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       2,
	})
	if err == nil {
		t.Fatalf("expected stale owner epoch release to fail")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition for stale owner epoch release, got %s", status.Code(err))
	}

	resp, err := releaseLogic.ReleaseSeatFreeze(&pb.ReleaseSeatFreezeReq{
		FreezeToken:      freezeResp.FreezeToken,
		ReleaseReason:    "current epoch release",
		OwnerOrderNumber: ownerOrderNumber,
		OwnerEpoch:       3,
	})
	if err != nil {
		t.Fatalf("ReleaseSeatFreeze with current owner returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected release success for current owner")
	}
}
