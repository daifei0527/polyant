package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/daifei0527/agentwiki/internal/core/election"
	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ElectionHandler 选举 API 处理器
type ElectionHandler struct {
	electionSvc *election.ElectionService
}

// NewElectionHandler 创建选举处理器
func NewElectionHandler(store kv.Store) *ElectionHandler {
	return &ElectionHandler{
		electionSvc: election.NewElectionService(
			kv.NewElectionStore(store),
			kv.NewCandidateStore(store),
			kv.NewVoteStore(store),
		),
	}
}

// CreateElectionRequest 创建选举请求
type CreateElectionRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	VoteThreshold int32  `json:"vote_threshold"`
	DurationDays  int    `json:"duration_days"`
}

// NominateRequest 提名请求
type NominateRequest struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
}

// VoteRequest 投票请求
type VoteRequest struct {
	CandidateID string `json:"candidate_id"`
}

// CreateElectionHandler 创建选举
// POST /api/v1/elections
func (h *ElectionHandler) CreateElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	var req CreateElectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "invalid request body",
		})
		return
	}

	publicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	election, err := h.electionSvc.CreateElection(ctx, req.Title, req.Description, publicKey, req.VoteThreshold, req.DurationDays)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"election_id": election.ID},
	})
}

// ListElectionsHandler 列出选举
// GET /api/v1/elections?status=active
func (h *ElectionHandler) ListElectionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	status := model.ElectionStatus(r.URL.Query().Get("status"))

	ctx := r.Context()
	elections, err := h.electionSvc.ListElections(ctx, status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"elections": elections},
	})
}

// GetElectionHandler 获取选举详情
// GET /api/v1/elections/{id}
func (h *ElectionHandler) GetElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	electionID := extractLastPathParam(r.URL.Path)
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "missing election_id",
		})
		return
	}

	ctx := r.Context()
	election, err := h.electionSvc.GetElection(ctx, electionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, &APIResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	candidates, _ := h.electionSvc.ListCandidates(ctx, electionID)

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"election":   election,
			"candidates": candidates,
		},
	})
}

// NominateCandidateHandler 提名候选人
// POST /api/v1/elections/{id}/candidates
func (h *ElectionHandler) NominateCandidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/candidates")
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "missing election_id",
		})
		return
	}

	var req NominateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "invalid request body",
		})
		return
	}

	publicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.electionSvc.NominateCandidate(ctx, electionID, req.UserID, req.UserName, publicKey); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]bool{"success": true},
	})
}

// VoteHandler 投票
// POST /api/v1/elections/{id}/vote
func (h *ElectionHandler) VoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/vote")
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "missing election_id",
		})
		return
	}

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "invalid request body",
		})
		return
	}

	publicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.electionSvc.Vote(ctx, electionID, publicKey, req.CandidateID); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]bool{"success": true},
	})
}

// CloseElectionHandler 关闭选举
// POST /api/v1/elections/{id}/close
func (h *ElectionHandler) CloseElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/close")
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "missing election_id",
		})
		return
	}

	ctx := r.Context()
	elected, err := h.electionSvc.CloseElection(ctx, electionID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]interface{}{"elected": elected},
	})
}

// extractLastPathParam 从 URL 路径中提取最后一个路径参数
func extractLastPathParam(path string) string {
	path = strings.TrimSuffix(path, "/")
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// extractPathParam 从 URL 路径中提取指定前缀和后缀之间的参数
func extractPathParam(path, prefix, suffix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimSuffix(rest, suffix)
	rest = strings.TrimSuffix(rest, "/")
	return rest
}
