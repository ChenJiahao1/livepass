package integration_test

import (
	"reflect"
	"testing"

	"livepass/services/user-api/internal/types"
)

func TestProtectedRequestTypesDoNotExposeIdentityFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		target    interface{}
		forbidden []string
	}{
		{name: "GetUserByIDReq", target: types.GetUserByIDReq{}, forbidden: []string{"ID"}},
		{name: "GetUserByMobileReq", target: types.GetUserByMobileReq{}, forbidden: []string{"Mobile"}},
		{name: "UpdateUserReq", target: types.UpdateUserReq{}, forbidden: []string{"ID"}},
		{name: "UpdatePasswordReq", target: types.UpdatePasswordReq{}, forbidden: []string{"ID"}},
		{name: "UpdateEmailReq", target: types.UpdateEmailReq{}, forbidden: []string{"ID"}},
		{name: "UpdateMobileReq", target: types.UpdateMobileReq{}, forbidden: []string{"ID"}},
		{name: "AuthenticationReq", target: types.AuthenticationReq{}, forbidden: []string{"ID"}},
		{name: "GetUserAndTicketUserListReq", target: types.GetUserAndTicketUserListReq{}, forbidden: []string{"UserID"}},
		{name: "ListTicketUsersReq", target: types.ListTicketUsersReq{}, forbidden: []string{"UserID"}},
		{name: "AddTicketUserReq", target: types.AddTicketUserReq{}, forbidden: []string{"UserID"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			targetType := reflect.TypeOf(tc.target)
			for _, field := range tc.forbidden {
				if _, ok := targetType.FieldByName(field); ok {
					t.Fatalf("expected %s not to expose %s", tc.name, field)
				}
			}
		})
	}
}
