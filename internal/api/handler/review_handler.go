package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/review"
	"github.com/daifei0527/polyant/internal/storage/model"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// ReviewHandler exposes the content-review admin endpoints.
type ReviewHandler struct {
	svc *review.Service
}

// NewReviewHandler creates a ReviewHandler backed by a review.Service.
func NewReviewHandler(svc *review.Service) *ReviewHandler {
	return &ReviewHandler{svc: svc}
}

type reviewActionRequest struct {
	Reason string `json:"reason"`
}

// ListReviewQueueHandler lists entries awaiting review.
// GET /api/v1/admin/entries?status=review&page=1&limit=20
func (h *ReviewHandler) ListReviewQueueHandler(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = model.EntryStatusReview
	}

	page := 1
	limit := 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := (page - 1) * limit

	entries, total, err := h.svc.ListQueue(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, awerrors.Wrap(310, awerrors.CategoryStorage, "list review queue failed", 500, err))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"entries": entries, "total": total,
	}})
}

// ApproveEntryHandler approves a review entry.
// POST /api/v1/admin/entries/{id}/approve
func (h *ReviewHandler) ApproveEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.doTransition(w, r, func(id, reviewer string) error {
		_, err := h.svc.Approve(r.Context(), id, reviewer)
		return err
	})
}

// RejectEntryHandler rejects a review entry.
// POST /api/v1/admin/entries/{id}/reject
func (h *ReviewHandler) RejectEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.doTransitionWithReason(w, r, func(id, reviewer, reason string) error {
		_, err := h.svc.Reject(r.Context(), id, reviewer, reason)
		return err
	})
}

// TakedownEntryHandler takes down a published entry.
// POST /api/v1/admin/entries/{id}/takedown
func (h *ReviewHandler) TakedownEntryHandler(w http.ResponseWriter, r *http.Request) {
	h.doTransitionWithReason(w, r, func(id, reviewer, reason string) error {
		_, err := h.svc.Takedown(r.Context(), id, reviewer, reason)
		return err
	})
}

func (h *ReviewHandler) doTransition(w http.ResponseWriter, r *http.Request, fn func(id, reviewer string) error) {
	id := EntryIDFromPath(r.URL.Path)
	reviewer, _ := r.Context().Value(mw.PublicKeyKey).(string)
	if err := fn(id, reviewer); err != nil {
		writeError(w, awerrors.New(311, awerrors.CategoryAPI, err.Error(), httpStatusForReviewErr(err)))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success"})
}

func (h *ReviewHandler) doTransitionWithReason(w http.ResponseWriter, r *http.Request, fn func(id, reviewer, reason string) error) {
	id := EntryIDFromPath(r.URL.Path)
	reviewer, _ := r.Context().Value(mw.PublicKeyKey).(string)
	var body reviewActionRequest
	_ = json.NewDecoder(r.Body).Decode(&body) // reason optional
	if err := fn(id, reviewer, body.Reason); err != nil {
		writeError(w, awerrors.New(311, awerrors.CategoryAPI, err.Error(), httpStatusForReviewErr(err)))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success"})
}

// EntryIDFromPath parses /api/v1/admin/entries/{id}/<action> -> {id}.
// net/http mux doesn't do path vars; parse manually.
func EntryIDFromPath(path string) string {
	const prefix = "/api/v1/admin/entries/"
	if len(path) <= len(prefix) {
		return ""
	}
	rest := path[len(prefix):]
	for i, c := range rest {
		if c == '/' {
			return rest[:i]
		}
	}
	return rest
}

// httpStatusForReviewErr maps review service errors to HTTP status codes.
func httpStatusForReviewErr(err error) int {
	if errors.Is(err, review.ErrEntryNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, review.ErrIllegalTransition) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
