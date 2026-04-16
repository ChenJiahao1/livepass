package testkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTestConfigDefersGatewayPortAllocation(t *testing.T) {
	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()
	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()
	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()
	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	c := NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000)

	if c.Port != 0 {
		t.Fatalf("expected gateway port allocation to be deferred until start, got %d", c.Port)
	}
}
