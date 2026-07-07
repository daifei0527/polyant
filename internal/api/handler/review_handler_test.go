package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daifei0527/polyant/internal/api/handler"
	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/review"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newReviewHandler(t *testing.T) (*handler.ReviewHandler, *storage.Store) {
	t.Helper()
	store, _ := storage.NewMemoryStore()
	svc := review.NewService(store, nil)
	return handler.NewReviewHandler(svc), store
}

func TestApproveEntryHandler_OK(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusReview})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/e1/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.ApproveEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	e, _ := store.Entry.Get(context.Background(), "e1")
	if e.Status != model.EntryStatusPublished {
		t.Errorf("entry status = %q, want published", e.Status)
	}
}

func TestRejectEntryHandler_WithReason(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e2", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusReview})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/e2/reject", strings.NewReader(`{"reason":"spam"}`))
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.RejectEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	e, _ := store.Entry.Get(context.Background(), "e2")
	if e.Status != model.EntryStatusArchived || e.ReviewReason != "spam" {
		t.Errorf("entry = status %q reason %q, want archived/spam", e.Status, e.ReviewReason)
	}
}

func TestTakedownEntryHandler_OK(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e3", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusPublished})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/e3/takedown", strings.NewReader(`{"reason":"violation"}`))
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.TakedownEntryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	e, _ := store.Entry.Get(context.Background(), "e3")
	if e.Status != model.EntryStatusReview || e.ReviewReason != "violation" {
		t.Errorf("entry = status %q reason %q, want review/violation", e.Status, e.ReviewReason)
	}
}

func TestApproveEntryHandler_NotFound(t *testing.T) {
	h, _ := newReviewHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/nonexistent/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.ApproveEntryHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRejectEntryHandler_IllegalTransition(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e4", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusPublished})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/entries/e4/reject", strings.NewReader(`{"reason":"spam"}`))
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "reviewer-pk"))
	rec := httptest.NewRecorder()
	h.RejectEntryHandler(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

func TestListReviewQueueHandler_OK(t *testing.T) {
	h, store := newReviewHandler(t)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e5", Title: "t", Content: "c", Category: "x", Status: model.EntryStatusReview})
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e6", Title: "t2", Content: "c2", Category: "x", Status: model.EntryStatusPublished})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/entries?status=review&page=1&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ListReviewQueueHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestEntryIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/admin/entries/e1/approve", "e1"},
		{"/api/v1/admin/entries/e2/reject", "e2"},
		{"/api/v1/admin/entries/e3/takedown", "e3"},
		{"/api/v1/admin/entries/", ""},
		{"/api/v1/admin/entries", ""},
	}
	for _, tt := range tests {
		got := handler.EntryIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("EntryIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
