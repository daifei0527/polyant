package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
)

// TestRouter_CloseExists asserts the router exposes a Close hook so callers
// can release the auth middleware's background goroutine on shutdown, and that
// *Router still satisfies http.Handler (P1.9).
func TestRouter_CloseExists(t *testing.T) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	r, err := NewRouterWithDeps(&Dependencies{
		Store:      store,
		EntryStore: store.Entry,
		UserStore:  store.User,
		NodeID:     "test",
		NodeType:   "local",
	})
	if err != nil {
		t.Fatalf("NewRouterWithDeps: %v", err)
	}
	if r == nil {
		t.Fatal("nil router")
	}

	// *Router must be usable as an http.Handler.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Must be closeable without panic.
	r.Close()
}
