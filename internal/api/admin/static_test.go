package admin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticHandler_ServeHTTP_Index(t *testing.T) {
	h := NewStaticHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "AgentWiki Admin")
	assert.True(t, strings.Contains(rec.Header().Get("Content-Type"), "text/html"))
}

func TestStaticHandler_ServeHTTP_Root(t *testing.T) {
	h := NewStaticHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "AgentWiki Admin")
}

func TestStaticHandler_ServeHTTP_SPARoute(t *testing.T) {
	h := NewStaticHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/settings", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	// SPA fallback should return index.html content
	assert.Contains(t, string(body), "AgentWiki Admin")
}

func TestStaticHandler_ServeHTTP_StaticFile(t *testing.T) {
	h := NewStaticHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/assets/app.js", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "console.log")
	assert.True(t, strings.Contains(rec.Header().Get("Content-Type"), "javascript"))
}
