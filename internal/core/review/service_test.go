package review

import (
	"context"
	"sync"
	"testing"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

type mockPusher struct {
	mu  sync.Mutex
	got []*model.KnowledgeEntry
}

func (m *mockPusher) PushEntry(entry *model.KnowledgeEntry, signature []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.got = append(m.got, entry)
	return nil
}

func newTestService(t *testing.T) (*Service, *storage.Store, *mockPusher) {
	t.Helper()
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	p := &mockPusher{}
	return NewService(store, p), store, p
}

func seed(t *testing.T, store *storage.Store, id, status string) *model.KnowledgeEntry {
	t.Helper()
	e := &model.KnowledgeEntry{ID: id, Title: "T", Content: "C", Category: "x", Status: status}
	created, err := store.Entry.Create(context.Background(), e)
	if err != nil {
		t.Fatalf("seed Create %s: %v", id, err)
	}
	return created
}

func TestApprove_ReviewToPublished(t *testing.T) {
	svc, store, p := newTestService(t)
	seed(t, store, "e1", model.EntryStatusReview)

	got, err := svc.Approve(context.Background(), "e1", "reviewer-pk")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if got.Status != model.EntryStatusPublished {
		t.Errorf("status = %q, want published", got.Status)
	}
	if got.ReviewedBy != "reviewer-pk" || got.ReviewedAt == 0 {
		t.Errorf("reviewer fields not set: by=%q at=%d", got.ReviewedBy, got.ReviewedAt)
	}
	if len(p.got) != 1 {
		t.Errorf("push not called once: got %d", len(p.got))
	}
}

func TestReject_ReviewToArchived(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "e2", model.EntryStatusReview)

	got, err := svc.Reject(context.Background(), "e2", "reviewer-pk", "spam")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if got.Status != model.EntryStatusArchived {
		t.Errorf("status = %q, want archived", got.Status)
	}
	if got.ReviewReason != "spam" {
		t.Errorf("reason = %q, want spam", got.ReviewReason)
	}
}

func TestTakedown_PublishedToReview(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "e3", model.EntryStatusPublished)

	got, err := svc.Takedown(context.Background(), "e3", "reviewer-pk", "policy")
	if err != nil {
		t.Fatalf("Takedown: %v", err)
	}
	if got.Status != model.EntryStatusReview {
		t.Errorf("status = %q, want review", got.Status)
	}
	if got.ReviewReason != "policy" {
		t.Errorf("reason = %q, want policy", got.ReviewReason)
	}
}

func TestApprove_IllegalTransition(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "e4", model.EntryStatusPublished) // already published
	if _, err := svc.Approve(context.Background(), "e4", "r"); err == nil {
		t.Error("Approve on published should error (illegal transition)")
	}
}

func TestListQueue_FiltersByStatus(t *testing.T) {
	svc, store, _ := newTestService(t)
	seed(t, store, "r1", model.EntryStatusReview)
	seed(t, store, "r2", model.EntryStatusReview)
	seed(t, store, "p1", model.EntryStatusPublished)

	entries, total, err := svc.ListQueue(context.Background(), model.EntryStatusReview, 10, 0)
	if err != nil {
		t.Fatalf("ListQueue: %v", err)
	}
	if total != 2 || len(entries) != 2 {
		t.Errorf("review queue: total=%d len=%d, want 2/2", total, len(entries))
	}
}
