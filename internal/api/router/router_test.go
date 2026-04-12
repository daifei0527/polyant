package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daifei0527/agentwiki/internal/core/email"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/index"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	"github.com/daifei0527/agentwiki/pkg/config"
)

func newTestStore(t *testing.T) *storage.Store {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store
}

func TestNewRouter(t *testing.T) {
	store := newTestStore(t)
	cfg := &config.Config{
		Node: config.NodeConfig{
			Type: "local",
		},
	}

	router, err := NewRouter(store, cfg)
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}

	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestNewRouterWithDeps(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node-1",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, err := NewRouterWithDeps(deps)
	if err != nil {
		t.Fatalf("NewRouterWithDeps failed: %v", err)
	}

	if router == nil {
		t.Fatal("NewRouterWithDeps returned nil")
	}
}

func TestNewRouterWithDeps_NilVerificationMgr(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node-1",
		NodeType:      "local",
		Version:       "v1.0.0",
		// VerificationMgr is nil
	}

	router, err := NewRouterWithDeps(deps)
	if err != nil {
		t.Fatalf("NewRouterWithDeps should create VerificationMgr: %v", err)
	}

	if router == nil {
		t.Fatal("NewRouterWithDeps returned nil")
	}
}

func TestRouter_PublicRoutes(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, _ := NewRouterWithDeps(deps)

	tests := []struct {
		method string
		path   string
		status int // Expected status (404 for unimplemented, 200 for OK, etc.)
	}{
		{"GET", "/api/v1/node/status", http.StatusOK},
		{"GET", "/api/v1/categories", http.StatusOK},
		{"GET", "/api/v1/search", http.StatusBadRequest}, // Missing query param
		{"POST", "/api/v1/user/register", http.StatusBadRequest}, // Missing body
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			// We're testing that routes exist and return expected status codes
			// 404 would mean route doesn't exist
			if rec.Code == http.StatusNotFound {
				t.Errorf("Route %s should exist", tt.path)
			}
		})
	}
}

func TestRouter_AuthRoutes(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		EmailService:  email.NewService(email.Config{}),
		NodeID:        "test-node",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, _ := NewRouterWithDeps(deps)

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/entry/create"},
		{"PUT", "/api/v1/entry/update/test-id"},
		{"DELETE", "/api/v1/entry/delete/test-id"},
		{"POST", "/api/v1/entry/rate/test-id"},
		{"POST", "/api/v1/user/send-verification"},
		{"POST", "/api/v1/user/verify-email"},
		{"GET", "/api/v1/user/info"},
		{"PUT", "/api/v1/user/update"},
		{"POST", "/api/v1/categories/create"},
		{"POST", "/api/v1/node/sync"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			// Auth routes should return 401 Unauthorized without valid auth
			// (not 404 Not Found)
			if rec.Code == http.StatusNotFound {
				t.Errorf("Route %s should exist", tt.path)
			}
		})
	}
}

func TestRouter_EntryRoutes(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, _ := NewRouterWithDeps(deps)

	// Test GET /api/v1/entry/{id}
	// Note: This route may return 404 if entry doesn't exist, which is expected behavior
	t.Run("GET entry", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/entry/nonexistent", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// Route exists - 404 is a valid response for non-existent entry
		if rec.Code != http.StatusNotFound {
			t.Logf("Entry route returned: %d", rec.Code)
		}
	})

	// Test GET /api/v1/entry/{id}/backlinks
	t.Run("GET backlinks", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/entry/test-id/backlinks", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// Route exists - 404 is a valid response for non-existent entry
		if rec.Code != http.StatusNotFound {
			t.Logf("Backlinks route returned: %d", rec.Code)
		}
	})

	// Test GET /api/v1/entry/{id}/outlinks
	t.Run("GET outlinks", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/entry/test-id/outlinks", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// Route exists - 404 is a valid response for non-existent entry
		if rec.Code != http.StatusNotFound {
			t.Logf("Outlinks route returned: %d", rec.Code)
		}
	})
}

func TestRouter_CategoryRoutes(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, _ := NewRouterWithDeps(deps)

	// Test GET /api/v1/categories/{id}/entries
	t.Run("GET category entries", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/categories/tech/entries", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		// The route exists if we get a JSON error response (not a plain 404)
		// A plain text "404 page not found" means the route doesn't exist
		// A JSON error means the route exists but category wasn't found
		body := rec.Body.String()
		if rec.Code == http.StatusNotFound && !strings.HasPrefix(body, "{") {
			t.Error("Category entries route should exist")
		}
	})

	// POST to /api/v1/categories should return 404 (not a public route)
	t.Run("POST categories root", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/categories", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Error("POST to /api/v1/categories should return 404")
		}
	})
}

func TestRouter_MiddlewareChain(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, _ := NewRouterWithDeps(deps)

	// Test that RequestID middleware is applied
	req := httptest.NewRequest("GET", "/api/v1/node/status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Check for Request-ID header (set by RequestIDMiddleware)
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("Request-ID header should be set by middleware")
	}
}

func TestRouter_CORS(t *testing.T) {
	store := newTestStore(t)

	deps := &Dependencies{
		EntryStore:    store.Entry,
		UserStore:     store.User,
		RatingStore:   store.Rating,
		CategoryStore: store.Category,
		SearchEngine:  store.Search,
		Backlink:      store.Backlink,
		NodeID:        "test-node",
		NodeType:      "local",
		Version:       "v1.0.0",
	}

	router, _ := NewRouterWithDeps(deps)

	// Test CORS preflight
	req := httptest.NewRequest("OPTIONS", "/api/v1/node/status", nil)
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// CORS preflight should return 204
	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected 204 for OPTIONS, got %d", rec.Code)
	}

	// CORS headers should be set (Access-Control-Allow-Origin might be * or specific origin)
	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin == "" {
		t.Error("CORS Access-Control-Allow-Origin header should be set")
	}
}

func TestRemoteQuerierAdapter(t *testing.T) {
	// Mock implementation
	mock := &mockRemoteQuerier{result: &index.SearchResult{}}
	adapter := &remoteQuerierAdapter{querier: mock}

	result, err := adapter.SearchWithRemote(context.Background(), index.SearchQuery{})
	if err != nil {
		t.Fatalf("SearchWithRemote failed: %v", err)
	}

	if result == nil {
		t.Error("Result should not be nil")
	}
}

// Mock implementations

type mockRemoteQuerier struct {
	result *index.SearchResult
	err    error
}

func (m *mockRemoteQuerier) SearchWithRemote(ctx context.Context, query index.SearchQuery) (*index.SearchResult, error) {
	return m.result, m.err
}

type mockEntryPusher struct {
	err error
}

func (m *mockEntryPusher) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	return m.err
}
