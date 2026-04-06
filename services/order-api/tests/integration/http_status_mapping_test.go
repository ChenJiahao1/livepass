package integration_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmiddleware"
	"damai-go/services/order-api/internal/handler"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateOrderHandlerMapsResourceExhaustedToTooManyRequests(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		createOrderErr: status.Error(codes.ResourceExhausted, xerr.ErrOrderSubmitTooFrequent.Error()),
	}

	req := httptest.NewRequest(http.MethodPost, "/order/create", strings.NewReader(`{"purchaseToken":"pt_91001"}`))
	req = req.WithContext(xmiddleware.WithUserID(req.Context(), 3001))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.CreateOrderHandler(newOrderAPIServiceContext(fakeRPC))(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateOrderHandlerMapsUnavailableToServiceUnavailable(t *testing.T) {
	fakeRPC := &fakeOrderRPC{
		createOrderErr: status.Error(codes.Unavailable, "repeat guard unavailable"),
	}

	req := httptest.NewRequest(http.MethodPost, "/order/create", strings.NewReader(`{"purchaseToken":"pt_91001"}`))
	req = req.WithContext(xmiddleware.WithUserID(req.Context(), 3001))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.CreateOrderHandler(newOrderAPIServiceContext(fakeRPC))(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}
