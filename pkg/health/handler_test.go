package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Health_Healthy(t *testing.T) {
	e := echo.New()
	checker := NewChecker(ModeFull)
	handler := NewHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Health(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp Response
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, StatusOK, resp.Status)
	assert.Equal(t, "full", resp.Mode)
}

func TestHandler_Health_NotHealthy(t *testing.T) {
	e := echo.New()
	checker := NewChecker(ModeInit)
	handler := NewHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Health(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp Response
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, resp.Status)
}

func TestHandler_Liveness(t *testing.T) {
	e := echo.New()
	checker := NewChecker(ModeInit) // Even in init mode, liveness should return OK
	handler := NewHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Liveness(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp Response
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, StatusOK, resp.Status)
}

func TestHandler_Readiness_Ready(t *testing.T) {
	e := echo.New()
	checker := NewChecker(ModeFull)
	handler := NewHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Readiness(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp Response
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, StatusOK, resp.Status)
}

func TestHandler_Readiness_NotReady(t *testing.T) {
	e := echo.New()
	checker := NewChecker(ModeInit)
	handler := NewHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Readiness(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp Response
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, resp.Status)
}

func TestHandler_RegisterRoutes(t *testing.T) {
	e := echo.New()
	checker := NewChecker(ModeFull)
	handler := NewHandler(checker)

	handler.RegisterRoutes(e)

	routes := e.Routes()
	paths := make([]string, len(routes))
	for i, r := range routes {
		paths[i] = r.Path
	}

	assert.Contains(t, paths, "/healthz")
	assert.Contains(t, paths, "/livez")
	assert.Contains(t, paths, "/readyz")
}
