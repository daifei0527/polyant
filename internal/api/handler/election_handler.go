package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/daifei0527/polyant/internal/core/election"
	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
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
	AutoElect     bool   `json:"auto_elect"` // 是否自动当选
}

// NominateRequest 提名请求
type NominateRequest struct {
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	SelfNominated bool   `json:"self_nominated"` // true=自荐, false=他荐
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
	election, err := h.electionSvc.CreateElection(ctx, req.Title, req.Description, publicKey, req.VoteThreshold, req.DurationDays, req.AutoElect)
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
		Data: map[string]interface{}{
			"election_id": election.ID,
			"auto_elect":  election.AutoElect,
		},
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

	// 如果是自荐，UserID 应该是自己的公钥
	if req.SelfNominated {
		req.UserID = publicKey
	}

	ctx := r.Context()
	if err := h.electionSvc.NominateCandidate(ctx, electionID, req.UserID, req.UserName, publicKey, req.SelfNominated); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"success":        true,
			"self_nominated": req.SelfNominated,
			"confirmed":      req.SelfNominated, // 自荐自动确认
		},
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
	result, err := h.electionSvc.Vote(ctx, electionID, publicKey, req.CandidateID)
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
		Data:    result,
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

// ConfirmNominationHandler 确认接受提名
// POST /api/v1/elections/{id}/candidates/{user_id}/confirm
func (h *ElectionHandler) ConfirmNominationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, &APIResponse{
			Code:    405,
			Message: "method not allowed",
		})
		return
	}

	// 解析路径: /api/v1/elections/{election_id}/candidates/{user_id}/confirm
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 8 {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: "invalid path",
		})
		return
	}
	electionID := parts[4]
	userID := parts[6]

	// 验证权限：只有被提名人自己可以确认
	publicKey, _ := r.Context().Value("public_key").(string)
	if publicKey != userID {
		writeJSON(w, http.StatusForbidden, &APIResponse{
			Code:    403,
			Message: "只有被提名人自己可以确认",
		})
		return
	}

	ctx := r.Context()
	if err := h.electionSvc.ConfirmNomination(ctx, electionID, userID); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    map[string]bool{"confirmed": true},
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
