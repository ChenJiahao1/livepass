package logic

import (
	"context"

	"livepass/pkg/xerr"
	"livepass/pkg/xmiddleware"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func requireCurrentUserID(ctx context.Context) (int64, error) {
	userID, ok := xmiddleware.UserIDFromContext(ctx)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	return userID, nil
}
