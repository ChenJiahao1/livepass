package integration_test

import (
	"testing"

	"livepass/services/user-rpc/pb"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestLoginReqSupportsLegacyFieldNumbers(t *testing.T) {
	var payload []byte
	payload = appendLegacyStringField(payload, 2, "13800000010")
	payload = appendLegacyStringField(payload, 3, "legacy@example.com")
	payload = appendLegacyStringField(payload, 4, "123456")

	var req pb.LoginReq
	if err := proto.Unmarshal(payload, &req); err != nil {
		t.Fatalf("unmarshal login req: %v", err)
	}

	if req.Mobile != "13800000010" || req.Email != "legacy@example.com" || req.Password != "123456" {
		t.Fatalf("expected legacy fields decoded into login req, got %+v", req)
	}
}

func TestLogoutReqSupportsLegacyFieldNumbers(t *testing.T) {
	payload := appendLegacyStringField(nil, 2, "legacy-token")

	var req pb.LogoutReq
	if err := proto.Unmarshal(payload, &req); err != nil {
		t.Fatalf("unmarshal logout req: %v", err)
	}

	if req.Token != "legacy-token" {
		t.Fatalf("expected legacy token decoded, got %+v", req)
	}
}

func appendLegacyStringField(payload []byte, fieldNumber protowire.Number, value string) []byte {
	payload = protowire.AppendTag(payload, fieldNumber, protowire.BytesType)
	return protowire.AppendString(payload, value)
}
