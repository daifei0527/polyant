package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBodyLimitMiddleware_RejectsOversize(t *testing.T) {
	handled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handled = true; w.WriteHeader(http.StatusOK) })

	// 2MB body 超过 1MB 限制 → 413，且下游 handler 不得被调用
	body := bytes.NewBufferString(strings.Repeat("a", 2<<20))
	req := httptest.NewRequest(http.MethodPost, "/x", body)
	rec := httptest.NewRecorder()

	BodyLimitMiddleware(1<<20)(inner).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.False(t, handled, "downstream handler must not run when body is oversize")
}

func TestBodyLimitMiddleware_AllowsWithinLimit(t *testing.T) {
	handled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handled = true; w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(strings.Repeat("a", 100)))
	rec := httptest.NewRecorder()

	BodyLimitMiddleware(1<<20)(inner).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, handled, "downstream handler must run when body is within limit")
}

func TestBodyLimitMiddleware_DisabledWhenZero(t *testing.T) {
	// maxBytes<=0 表示不限制：超大 body 也放行（由 ContentLength 判定）
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(strings.Repeat("a", 2<<20)))
	rec := httptest.NewRecorder()

	BodyLimitMiddleware(0)(inner).ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimitMiddleware_TruncatesStreamingOversize(t *testing.T) {
	// ContentLength=-1（chunked）时，MaxBytesReader 让下游读到超限时 Read 报错。
	var readErr error
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	// 手工构造 ContentLength=-1 的请求（chunked 语义），body 大于限制
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(make([]byte, 4<<20)))
	req.ContentLength = -1
	rec := httptest.NewRecorder()

	BodyLimitMiddleware(1<<20)(inner).ServeHTTP(rec, req)
	assert.NotNil(t, readErr, "reading past the limit must error")
}
