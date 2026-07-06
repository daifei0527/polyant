package admin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fetchIndex 拉取 /admin/ 的 index.html 内容（SPA 入口）。
func fetchIndex(t *testing.T) string {
	t.Helper()
	h := NewStaticHandler()
	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(body)
}

func TestStaticHandler_ServeHTTP_Index(t *testing.T) {
	body := fetchIndex(t)
	// Vue 挂载点，跨构建稳定
	assert.Contains(t, body, `<div id="app">`)
	assert.True(t, strings.Contains(body, "Polyant"))
}

func TestStaticHandler_ServeHTTP_Root(t *testing.T) {
	h := NewStaticHandler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `<div id="app">`)
}

func TestStaticHandler_ServeHTTP_SPARoute(t *testing.T) {
	h := NewStaticHandler()
	// 任意未匹配的子路径走 SPA fallback，返回 index.html
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/settings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `<div id="app">`)
}

func TestStaticHandler_ServeHTTP_StaticFile(t *testing.T) {
	// 从 index.html 动态解析 JS 资源路径（构建后文件名带 hash，硬编码会漂移）
	body := fetchIndex(t)
	re := regexp.MustCompile(`src="(/admin/assets/[^"]+\.js)"`)
	m := re.FindStringSubmatch(body)
	require.Len(t, m, 2, "index.html must reference a JS asset: %s", body)
	assetPath := m[1]

	h := NewStaticHandler()
	req := httptest.NewRequest(http.MethodGet, assetPath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	ct := rec.Header().Get("Content-Type")
	assert.True(t, strings.Contains(ct, "javascript"), "JS asset content-type should be javascript, got %q", ct)
}

// TestStaticHandler_ServeHTTP_DeepRefresh: 深层 SPA 路由刷新（如被删的内容审核入口
// /admin/entries）必须回 index.html（200，非 404）。锁死 R3-C 清理后仍能正常 fallback。
func TestStaticHandler_ServeHTTP_DeepRefresh(t *testing.T) {
	h := NewStaticHandler()
	req := httptest.NewRequest(http.MethodGet, "/admin/entries", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `<div id="app">`, "深层 SPA 路由刷新应回 index.html")
}
