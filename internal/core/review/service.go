package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/daifei0527/polyant/internal/api/handler"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

var (
	ErrEntryNotFound     = errors.New("entry not found")
	ErrIllegalTransition = errors.New("illegal status transition")
)

// Service implements the entry content-review workflow.
type Service struct {
	store  *storage.Store
	pusher handler.EntryPusher
}

// NewService creates a review service. pusher may be nil (push is skipped).
func NewService(store *storage.Store, pusher handler.EntryPusher) *Service {
	return &Service{store: store, pusher: pusher}
}

// ListQueue lists entries by status (typically "review") with pagination.
func (s *Service) ListQueue(ctx context.Context, status string, limit, offset int) ([]*model.KnowledgeEntry, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	return s.store.Entry.List(ctx, storage.EntryFilter{Status: status, Limit: limit, Offset: offset})
}

// Approve moves a review entry to published and indexes it.
func (s *Service) Approve(ctx context.Context, entryID, reviewerPubkey string) (*model.KnowledgeEntry, error) {
	return s.transition(ctx, entryID, model.EntryStatusReview, model.EntryStatusPublished, reviewerPubkey, "")
}

// Reject moves a review entry to archived (terminal) with a reason.
func (s *Service) Reject(ctx context.Context, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error) {
	return s.transition(ctx, entryID, model.EntryStatusReview, model.EntryStatusArchived, reviewerPubkey, reason)
}

// Takedown moves a published entry back to review with a reason.
func (s *Service) Takedown(ctx context.Context, entryID, reviewerPubkey, reason string) (*model.KnowledgeEntry, error) {
	return s.transition(ctx, entryID, model.EntryStatusPublished, model.EntryStatusReview, reviewerPubkey, reason)
}

func (s *Service) transition(ctx context.Context, entryID, fromStatus, toStatus string, reviewerPubkey, reason string) (*model.KnowledgeEntry, error) {
	entry, err := s.store.Entry.Get(ctx, entryID)
	if err != nil || entry == nil {
		return nil, ErrEntryNotFound
	}
	if entry.Status != fromStatus {
		return nil, fmt.Errorf("%w: entry %s is %q, expected %q", ErrIllegalTransition, entryID, entry.Status, fromStatus)
	}

	entry.Status = toStatus
	entry.ReviewedBy = reviewerPubkey
	entry.ReviewedAt = model.NowMillis()
	entry.ReviewReason = reason
	entry.Version++ // bump so LWW sync accepts the new state
	entry.UpdatedAt = model.NowMillis()

	updated, err := s.store.Entry.Update(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("update entry %s: %w", entryID, err)
	}

	// keep the search index in sync with published state
	s.syncIndex(ctx, updated)

	// propagate to peers (status carries as-is via existing sync/LWW)
	if s.pusher != nil {
		_ = s.pusher.PushEntry(updated, updated.Signature)
	}
	return updated, nil
}

// syncIndex adds the entry to the search index on publish, removes it otherwise.
func (s *Service) syncIndex(ctx context.Context, entry *model.KnowledgeEntry) {
	if s.store.Search == nil {
		return
	}
	if entry.Status == model.EntryStatusPublished {
		if err := s.store.Search.IndexEntry(entry); err != nil {
			fmt.Printf("[review] index entry %s failed: %v\n", entry.ID, err)
		}
	} else {
		if err := s.store.Search.DeleteIndex(entry.ID); err != nil {
			fmt.Printf("[review] de-index entry %s failed: %v\n", entry.ID, err)
		}
	}
}
