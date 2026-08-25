package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func do(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	newRouter(zap.NewNop()).ServeHTTP(rec, req)
	return rec
}

func TestHealthzReturnsOK(t *testing.T) {
	rec := do(t, http.MethodGet, "/healthz")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestMetricsEndpointIsExposed(t *testing.T) {
	rec := do(t, http.MethodGet, "/metrics")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "go_goroutines")
}

func TestUnknownRouteIs404(t *testing.T) {
	rec := do(t, http.MethodGet, "/nope")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
