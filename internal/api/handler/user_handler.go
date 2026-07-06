package handler

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/daifei0527/polyant/internal/core/email"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/safeconv"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// UserHandler 用户 HTTP 处理器
// 负责用户注册、邮箱验证、用户信息查询和条目评分
type UserHandler struct {
	store           *storage.Store
	userStore       storage.UserStore
	entryStore      storage.EntryStore
	ratingStore     storage.RatingStore
	emailService    *email.Service
	verificationMgr *email.VerificationManager
	devReturnCode   bool     // 仅 dev/测试：SendVerificationCodeHandler 是否在响应中回传验证码
	entryLocks      sync.Map // entryID -> *sync.Mutex，保证评分查重+写入原子（R2-D1）
}

// lockForEntry 返回某条目的评分互斥锁（惰性创建）。
// R2-D1：串行化同一 entry 的并发评分，使 GetByRater 查重与 Create 成为原子操作。
func (h *UserHandler) lockForEntry(entryID string) *sync.Mutex {
	actual, _ := h.entryLocks.LoadOrStore(entryID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// NewUserHandler 创建新的 UserHandler 实例
func NewUserHandler(
	store *storage.Store,
	userStore storage.UserStore,
	entryStore storage.EntryStore,
	ratingStore storage.RatingStore,
	emailService *email.Service,
	verificationMgr *email.VerificationManager,
) *UserHandler {
	return &UserHandler{
		store:           store,
		userStore:       userStore,
		entryStore:      entryStore,
		ratingStore:     ratingStore,
		emailService:    emailService,
		verificationMgr: verificationMgr,
	}
}

// SetDevReturnVerificationCode 控制发送验证码接口是否在响应中返回明文验证码。
// 默认 false（安全：验证码只通过邮件下发）。仅在开发/测试环境启用。
func (h *UserHandler) SetDevReturnVerificationCode(v bool) {
	h.devReturnCode = v
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

	// 验证必填字段
	if req.AgentName == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	var publicKey []byte
	if req.PublicKey != "" {
		// 使用客户端发送的公钥
		var err error
		publicKey, err = base64.StdEncoding.DecodeString(req.PublicKey)
		if err != nil || len(publicKey) != ed25519.PublicKeySize {
			writeError(w, awerrors.ErrInvalidParams)
			return
		}
	} else {
		// 兼容旧版本：服务器端生成密钥对
		var err error
		publicKey, _, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			writeError(w, awerrors.Wrap(0, awerrors.CategorySystem, "failed to generate keypair", 500, err))
			return
		}
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

	// 创建用户（默认为基础用户 Lv0，需要验证邮箱升级到 Lv1）
	user := &model.User{
		PublicKey:       base64.StdEncoding.EncodeToString(publicKey),
		AgentName:       req.AgentName,
		UserLevel:       model.UserLevelLv0,
		Email:           "",
		EmailVerified:   false,
		RegisteredAt:    now,
		LastActive:      now,
		ContributionCnt: 0,
		RatingCnt:       0,
		NodeId:          req.NodeID,
		Status:          model.UserStatusActive,
	}

	created, err := h.userStore.Create(r.Context(), user)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to create user", 500, err))
		return
	}

	// 构造响应
	respData := map[string]interface{}{
		"public_key":      created.PublicKey,
		"public_key_hash": pubKeyHash,
		"agent_name":      created.AgentName,
		"user_level":      created.UserLevel,
		"email_verified":  created.EmailVerified,
	}

	writeJSON(w, http.StatusCreated, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    respData,
	})
}

// SendVerificationCodeHandler 发送邮箱验证码
// POST /api/v1/user/send-verification
// 需要认证，发送验证码到指定邮箱
func (h *UserHandler) SendVerificationCodeHandler(w http.ResponseWriter, r *http.Request) {
	// 获取当前用户
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	// 解析请求
	var req SendVerificationCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// 验证邮箱格式
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !isValidEmail(req.Email) {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 检查邮箱是否已被其他用户使用（防止抢占他人邮箱）
	if existing, _ := h.userStore.GetByEmail(r.Context(), req.Email); existing != nil && existing.PublicKey != user.PublicKey {
		writeError(w, awerrors.ErrEmailAlreadyUsed)
		return
	}

	// R1-C3：锁定窗口内拒绝再次发送验证码（防暴破轰炸）
	if h.verificationMgr != nil && h.verificationMgr.IsEmailLocked(req.Email) {
		writeError(w, awerrors.New(429, awerrors.CategoryUser, "too many verification attempts, try again later", http.StatusTooManyRequests))
		return
	}

	// 生成验证码
	var code string
	if h.verificationMgr != nil {
		code = h.verificationMgr.GenerateCode(req.Email)
	} else {
		// 如果没有验证管理器，使用简化实现
		code = generateSimpleCode(req.Email)
	}

	// 发送验证邮件
	if h.emailService != nil {
		verifyURL := fmt.Sprintf("https://polyant.org/verify?email=%s&code=%s", req.Email, code)
		if err := h.emailService.SendVerificationEmail(req.Email, code, verifyURL); err != nil {
			// 记录错误但不暴露给用户
			// 在开发环境可以返回具体错误
			writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to send verification email", 500, err))
			return
		}
	}

	respData := map[string]interface{}{
		"email":      req.Email,
		"expires_in": 1800, // 30 分钟
	}
	if h.devReturnCode {
		// 仅 dev/测试：回传验证码以便测试完成验证流程。生产环境必须保持关闭。
		respData["code"] = code
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "verification code sent to your email",
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

	// 验证必填字段
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Code == "" || !isValidEmail(req.Email) {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	// 获取当前用户（优先从context获取）
	user := getUserFromContext(r.Context())
	if user == nil {
		// 尝试从请求头获取公钥查找用户
		pubKeyStr := r.Header.Get("X-Polyant-PublicKey")
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

	// 验证验证码
	var valid bool
	if h.verificationMgr != nil {
		valid = h.verificationMgr.Verify(req.Code, req.Email)
	} else {
		// 简化实现：使用固定 token 验证
		valid = verifySimpleCode(req.Code, req.Email)
	}

	if !valid {
		writeError(w, awerrors.ErrVerificationFailed)
		return
	}

	// 检查邮箱是否已被"其他"用户使用（本人重新验证自己的邮箱应放行）
	if existingUser, _ := h.userStore.GetByEmail(r.Context(), req.Email); existingUser != nil && existingUser.PublicKey != user.PublicKey {
		writeError(w, awerrors.ErrEmailAlreadyUsed)
		return
	}

	// 更新用户邮箱和验证状态
	user.Email = req.Email
	user.EmailVerified = true
	if user.UserLevel < model.UserLevelLv1 {
		user.UserLevel = model.UserLevelLv1 // 升级为正式用户
	}
	user.LastActive = model.NowMillis()

	updated, err := h.userStore.Update(r.Context(), user)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to update user", 500, err))
		return
	}

	// 发送欢迎邮件
	if h.emailService != nil && updated.AgentName != "" {
		go h.emailService.SendWelcomeEmail(updated.Email, updated.AgentName)
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "email verified, upgraded to verified user",
		Data: map[string]interface{}{
			"public_key":     updated.PublicKey,
			"user_level":     updated.UserLevel,
			"email":          updated.Email,
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

	// 计算公钥哈希
	pubKeyBytes, err := base64.StdEncoding.DecodeString(user.PublicKey)
	if err != nil {
		writeError(w, awerrors.ErrInvalidSignature)
		return
	}
	pubKeyHashBytes := sha256.Sum256(pubKeyBytes)
	pubKeyHash := hex.EncodeToString(pubKeyHashBytes[:])

	// 从存储中获取最新的用户信息
	latest, err := h.userStore.Get(r.Context(), pubKeyHash)
	if err != nil {
		writeError(w, awerrors.ErrUserNotFound)
		return
	}

	// 构造响应数据
	respData := map[string]interface{}{
		"public_key":       latest.PublicKey,
		"public_key_hash":  pubKeyHash,
		"agent_name":       latest.AgentName,
		"user_level":       latest.UserLevel,
		"email":            latest.Email,
		"email_verified":   latest.EmailVerified,
		"registered_at":    latest.RegisteredAt,
		"last_active":      latest.LastActive,
		"contribution_cnt": latest.ContributionCnt,
		"rating_cnt":       latest.RatingCnt,
		"status":           latest.Status,
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    respData,
	})
}

// UpdateUserInfoHandler 更新用户信息
// PUT /api/v1/user/info
// 需要认证
func (h *UserHandler) UpdateUserInfoHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		writeError(w, awerrors.ErrMissingAuth)
		return
	}

	var req struct {
		AgentName string `json:"agent_name,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, awerrors.ErrJSONParse)
		return
	}

	// Validate agent_name if provided (empty string is invalid)
	if req.AgentName == "" {
		writeError(w, awerrors.ErrInvalidParams)
		return
	}

	user.AgentName = req.AgentName
	user.LastActive = model.NowMillis()

	updated, err := h.userStore.Update(r.Context(), user)
	if err != nil {
		writeError(w, awerrors.Wrap(800, awerrors.CategoryUser, "failed to update user", 500, err))
		return
	}

	writeJSON(w, http.StatusOK, &APIResponse{
		Code:    0,
		Message: "success",
		Data:    updated,
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

	// R2-D1：per-entry 锁 —— GetByRater 查重与 Create 必须原子，防并发双重投票。
	// 锁粒度为单个 entry，仅串行化同一 entry 的并发评分，不影响其他 entry。
	entryMu := h.lockForEntry(entryID)
	entryMu.Lock()
	defer entryMu.Unlock()

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
		ID:            generateUUID(),
		EntryId:       entryID,
		RaterPubkey:   user.PublicKey,
		Score:         req.Score,
		Weight:        weight,
		WeightedScore: weightedScore,
		RatedAt:       now,
		Comment:       req.Comment,
	}

	if h.ratingStore != nil {
		created, err := h.ratingStore.Create(r.Context(), rating)
		if err != nil {
			writeError(w, awerrors.Wrap(700, awerrors.CategoryRating, "failed to create rating", 500, err))
			return
		}

		// R2-D2：重算 entry.Score（加权平均 Σ WeightedScore / Σ Weight）与 ScoreCount 并落库。
		// 口径与 kv.RatingStore.ComputeEntryScore 一致；修复"评分后 entry.Score 永远陈旧"。
		// 位于 D1 临界区内，避免与并发评分交错产生 lost-update。
		if entry, gerr := h.entryStore.Get(r.Context(), entryID); gerr == nil && entry != nil {
			ratings, rerr := h.ratingStore.ListByEntry(r.Context(), entryID)
			if rerr == nil && len(ratings) > 0 {
				var sumW, sumWS float64
				for _, rt := range ratings {
					sumW += rt.Weight
					sumWS += rt.WeightedScore
				}
				if sumW > 0 {
					entry.Score = sumWS / sumW
				}
				entry.ScoreCount = safeconv.Int32FromInt(len(ratings))
				if _, uerr := h.entryStore.Update(r.Context(), entry); uerr != nil {
					log.Printf("[UserHandler] recompute entry %s score failed: %v", entryID, uerr)
				}
			}
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

// isValidEmail 简单验证邮箱格式。R1-C1：含 CR/LF 一律拒绝，防 SMTP 头注入。
// （深度防御：email/service.go 仍会在写头时二次消毒。）
func isValidEmail(email string) bool {
	if strings.ContainsAny(email, "\r\n") {
		return false
	}
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// generateSimpleCode 生成简化验证码（用于无邮件服务时测试）
func generateSimpleCode(email string) string {
	h := sha256.New()
	h.Write([]byte(email))
	h.Write([]byte(time.Now().Format("2006-01-02")))
	h.Write([]byte("polyant-verification-secret"))
	return hex.EncodeToString(h.Sum(nil))[:6]
}

// verifySimpleCode 验证简化验证码
func verifySimpleCode(code, email string) bool {
	expected := generateSimpleCode(email)
	return code == expected
}
