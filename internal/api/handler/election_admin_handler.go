package handler

import (
	"encoding/json"
	"net/http"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/election"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// AdminElectionHandler exposes election management to the admin SPA (session-token).
type AdminElectionHandler struct {
	svc *election.ElectionService
}

// NewAdminElectionHandler creates an admin election handler backed by the given service.
func NewAdminElectionHandler(svc *election.ElectionService) *AdminElectionHandler {
	return &AdminElectionHandler{svc: svc}
}

// CreateElectionHandler handles POST /api/v1/admin/elections.
// It reads the caller's public key from the session context (mw.PublicKeyKey).
func (h *AdminElectionHandler) CreateElectionHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateElectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: "invalid request body"})
		return
	}
	createdBy, _ := r.Context().Value(mw.PublicKeyKey).(string)
	el, err := h.svc.CreateElection(r.Context(), req.Title, req.Description, createdBy, req.VoteThreshold, req.DurationDays, req.AutoElect)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &APIResponse{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"election_id": el.ID, "auto_elect": el.AutoElect,
	}})
}

// ListElectionsHandler handles GET /api/v1/admin/elections?status=...
func (h *AdminElectionHandler) ListElectionsHandler(w http.ResponseWriter, r *http.Request) {
	status := model.ElectionStatus(r.URL.Query().Get("status"))
	elections, err := h.svc.ListElections(r.Context(), status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &APIResponse{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{"elections": elections}})
}

// GetElectionHandler handles GET /api/v1/admin/elections/{id}.
func (h *AdminElectionHandler) GetElectionHandler(w http.ResponseWriter, r *http.Request) {
	electionID := extractPathParam(r.URL.Path, "/api/v1/admin/elections/", "")
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: "missing election_id"})
		return
	}
	el, err := h.svc.GetElection(r.Context(), electionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, &APIResponse{Code: 404, Message: "election not found"})
		return
	}
	candidates, _ := h.svc.ListCandidates(r.Context(), electionID)
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"election": el, "candidates": candidates,
	}})
}

// CloseElectionHandler handles POST /api/v1/admin/elections/{id}/close.
func (h *AdminElectionHandler) CloseElectionHandler(w http.ResponseWriter, r *http.Request) {
	electionID := extractPathParam(r.URL.Path, "/api/v1/admin/elections/", "/close")
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: "missing election_id"})
		return
	}
	elected, err := h.svc.CloseElection(r.Context(), electionID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{"elected": elected}})
}
