package handler

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"

	awerrors "github.com/agentwiki/agentwiki/pkg/errors"
	"github.com/agentwiki/agentwiki/internal/storage/model"
	"github.com/agentwiki/agentwiki/internal/storage"
)

// UserHandler 用户 HTTP 处理器
// 负责用户注册、邮箱验证、用户信息查询和条目评分
type UserHandler struct {
	userStore   storage.UserStore
	entryStore  storage.EntryStore
	ratingStore storage.RatingStore
}

// NewUserHandler 创建新的 UserHandler 实例
func NewUserHandler(userStore storage.UserStore, entryStore storage.EntryStore, ratingStore storage.RatingStore) *UserHandler {
	return &UserHandler{
		userStore:   userStore,
		entryStore:  entryStore,
		ratingStore: ratingStore,
	}
}

// RegisterHandler 用户注册
// POST /api/v1/user/register
// 生成 Ed25519 密钥对，创建用户，返回公钥
func (h *UserHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 生成 Ed25519 密钥对
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		writeError(w, awerrors.Wrap(0, awerrors.CategorySystem, "failed to generate keypair", 500, err))
		return
	}

	// 计算公钥哈希（用作用户唯一标识）
	pubKeyBytes := sha256.Sum256(publicKey)
	pubKeyHash := hex.EncodeToString(pubKeyBytes[:])

	// 检查用户是否已存在
	existing, _ := h.userStore.Get(r.Context(), pubKeyHash)
	if existing != nil {
		writeError(w, awerrors.ErrUserAlreadyExists)
		return
	}

	now := model.NowMillis()

	// 创建用户（默认为基础用户 Lv0）
	user := &model.User{
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
		AgentName:      req.AgentName,
		UserLevel:      model.UserLevelLv0,
		Email:          req.Email,
		EmailVerified:  false,
		RegisteredAt:   now,
		LastActive:     now,
		ContributionCnt: 0,
		RatingCnt:       0,
		NodeId:         req.NodeID,
		Status:         model.UserStatusActive,
	}

	created, err := h.userStore.Create(r.Context(), user)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to create user", 500, err))
		return
	}

	// 构造响应，将私钥也返回给用户（仅注册时返回一次）
	respData := map[string]interface{}{
		"public_key":      created.PublicKey,
		"public_key_hash": pubKeyHash,
		"private_key":     base64.StdEncoding.EncodeToString(privateKey),
		"agent_name":      created.AgentName,
		"user_level":      created.UserLevel,
		"warning":         "please store your private key securely, it will not be shown again",
	}

	writeJSON(w, http.StatusCreated, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    respData,
	})
}

// VerifyEmailHandler 邮箱验证
// POST /api/v1/user/verify-email
// 验证用户邮箱，验证成功后升级为正式用户（Lv1）
func (h *UserHandler) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	if req.Email == "" || req.Token == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 获取当前用户（优先从context获取，否则从请求头公钥获取）
	user := getUserFromContext(r.Context())
	if user == nil {
		// 尝试从请求头获取公钥查找用户
		pubKeyStr := r.Header.Get("X-AgentWiki-PublicKey")
		if pubKeyStr != "" {
			pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyStr)
			if err == nil {
				hash := sha256.Sum256(pubKeyBytes)
				pubKeyHash := hex.EncodeToString(hash[:])
				user, _ = h.userStore.Get(r.Context(), pubKeyHash)
			}
		}
	}
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 验证 token（简化实现：token 需要与邮箱匹配的预共享密钥）
	expectedToken := generateEmailToken(req.Email)
	if req.Token != expectedToken {
		writeError(w, awerrors.ErrInvalidEmailToken)
		return
	}

	// 更新用户邮箱和验证状态
	user.Email = req.Email
	user.EmailVerified = true
	user.UserLevel = model.UserLevelLv1
	user.LastActive = model.NowMillis()

	updated, err := h.userStore.Update(r.Context(), user)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to update user", 500, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "email verified, upgraded to verified user",
		Data: map[string]interface{}{
			"public_key": updated.PublicKey,
			"user_level": updated.UserLevel,
			"email_verified": updated.EmailVerified,
		},
	})
}

// GetUserInfoHandler 获取当前用户信息
// GET /api/v1/user/info
// 需要认证
func (h *UserHandler) GetUserInfoHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 从存储中获取最新的用户信息
	latest, err := h.userStore.Get(r.Context(), user.PublicKey)
	if err != nil {
		writeError(w, awerrors.ErrUserNotFound)
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    latest,
	})
}

// RateEntryHandler 为知识条目评分
// POST /api/v1/entry/{id}/rate
// 需要认证（Lv1及以上权限）
func (h *UserHandler) RateEntryHandler(w http.ResponseWriter, r *http.Request) {
	entryID := extractPathVar(r, "id")
	if entryID == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 获取当前用户
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 检查权限（Lv1及以上可评分）
	if user.UserLevel < model.UserLevelLv1 {
		writeError(w, awerrors.ErrBasicUserDenied)
		return
	}

	// 解析评分请求
	var req RateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 验证评分范围
	if req.Score < 1.0 || req.Score > 5.0 {
		writeError(w, awerrors.ErrScoreOutOfRange)
		return
	}

	// 检查条目是否存在
	_, err := h.entryStore.Get(r.Context(), entryID)
	if err != nil {
		writeError(w, awerrors.ErrEntryNotFound)
		return
	}

	// 检查是否已评分（防止重复评分）
	if h.ratingStore != nil {
		existing, _ := h.ratingStore.GetByRater(r.Context(), entryID, user.PublicKey)
		if existing != nil {
			writeError(w, awerrors.ErrDuplicateRating)
			return
		}
	}

	// 计算加权评分
	weight := model.GetLevelWeight(user.UserLevel)
	weightedScore := req.Score * weight

	now := model.NowMillis()

	// 创建评分记录
	rating := &model.Rating{
		ID:           generateUUID(),
		EntryId:      entryID,
		RaterPubkey:  user.PublicKey,
		Score:        req.Score,
		Weight:       weight,
		WeightedScore: weightedScore,
		RatedAt:      now,
		Comment:      req.Comment,
	}

	if h.ratingStore != nil {
		created, err := h.ratingStore.Create(r.Context(), rating)
		if err != nil {
			writeError(w, awerrors.Wrap(700, awerrors.CategoryRating, "failed to create rating", 500, err))
			return
		}
		writeJSON(w, http.StatusCreated, &APIResponse{
			Code:    0,
			Message: "success",
			Data:    created,
		})
		return
	}

	// 如果没有 ratingStore，直接返回评分结果
	writeJSON(w, http.StatusCreated, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    rating,
	})
}

// generateEmailToken 生成邮箱验证 token（简化实现）
// 生产环境中应使用随机生成的 token 并通过邮件发送
func generateEmailToken(email string) string {
	h := sha256.New()
	h.Write([]byte(email))
	h.Write([]byte("agentwiki-email-verification-secret"))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
