// Package errors 定义了 AgentWiki 项目的统一错误类型和预定义错误码。
// 错误码结构: AABBB (AA=模块, BBB=序号)
// 00=系统错误 01=API错误 02=认证错误
// 03=存储错误 04=网络错误 05=同步错误
// 06=搜索错误 07=评分错误 08=用户错误
// 09=配置错误
package errors

import (
	"fmt"
)

// ErrorCategory 错误类别枚举
type ErrorCategory int

const (
	CategorySystem  ErrorCategory = 0 // 系统错误
	CategoryAPI     ErrorCategory = 1 // API错误
	CategoryAuth    ErrorCategory = 2 // 认证错误
	CategoryStorage ErrorCategory = 3 // 存储错误
	CategoryNetwork ErrorCategory = 4 // 网络错误
	CategorySync    ErrorCategory = 5 // 同步错误
	CategorySearch  ErrorCategory = 6 // 搜索错误
	CategoryRating  ErrorCategory = 7 // 评分错误
	CategoryUser    ErrorCategory = 8 // 用户错误
	CategoryConfig  ErrorCategory = 9 // 配置错误
)

// AWError AgentWiki 统一错误类型
type AWError struct {
	Code       int           `json:"code"`
	Category   ErrorCategory `json:"-"`
	Message    string        `json:"message"`
	HTTPStatus int           `json:"-"`
	Cause      error         `json:"-"`
	Retryable  bool          `json:"-"`
}

// Error 实现 error 接口
func (e *AWError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持错误链解包
func (e *AWError) Unwrap() error { return e.Cause }

// New 创建一个新的 AWError
func New(code int, category ErrorCategory, message string, httpStatus int) *AWError {
	return &AWError{
		Code:       code,
		Category:   category,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap 创建一个包装了底层错误的 AWError
func Wrap(code int, category ErrorCategory, message string, httpStatus int, cause error) *AWError {
	return &AWError{
		Code:       code,
		Category:   category,
		Message:    message,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}

// WithRetry 设置错误是否可重试
func (e *AWError) WithRetry(retryable bool) *AWError {
	e.Retryable = retryable
	return e
}

// 预定义错误
var (
	// 系统错误 (0xxxx)
	ErrInternal    = New(0, CategorySystem, "internal error", 500)
	ErrUnavailable = New(1, CategorySystem, "service unavailable", 503)
	ErrRateLimited = New(2, CategorySystem, "rate limited", 429)

	// API错误 (1xxxx)
	ErrInvalidParams  = New(100, CategoryAPI, "invalid params", 400)
	ErrJSONParse      = New(102, CategoryAPI, "json parse failed", 400)
	ErrScoreOutOfRange = New(103, CategoryAPI, "score must be between 1.0 and 5.0", 400)

	// 认证错误 (2xxxx)
	ErrMissingAuth      = New(200, CategoryAuth, "missing auth info", 401)
	ErrInvalidSignature = New(201, CategoryAuth, "invalid signature", 401)
	ErrTimestampExpired = New(202, CategoryAuth, "timestamp expired", 401)
	ErrPermissionDenied = New(203, CategoryAuth, "permission denied", 403)
	ErrBasicUserDenied  = New(204, CategoryAuth, "basic user cannot perform this action", 403)
	ErrUserSuspended    = New(205, CategoryAuth, "user is suspended", 403)

	// 存储错误 (3xxxx)
	ErrEntryNotFound    = New(300, CategoryStorage, "entry not found", 404)
	ErrUserNotFound     = New(301, CategoryStorage, "user not found", 404)
	ErrCategoryNotFound = New(302, CategoryStorage, "category not found", 404)
	ErrDuplicateRating  = New(303, CategoryStorage, "duplicate rating", 409)
	ErrEntryExists      = New(304, CategoryStorage, "entry already exists", 409)
	ErrWriteFailed      = New(305, CategoryStorage, "storage write failed", 500)

	// 网络错误 (4xxxx)
	ErrPeerConnectFailed = New(400, CategoryNetwork, "peer connect failed", 502)

	// 同步错误 (5xxxx)
	ErrSyncFailed   = New(500, CategorySync, "sync failed", 500)
	ErrHashMismatch = New(502, CategorySync, "hash mismatch", 500)

	// 搜索错误 (6xxxx)
	ErrSearchFailed   = New(600, CategorySearch, "search failed", 500)
	ErrKeywordTooShort = New(601, CategorySearch, "keyword too short", 400)

	// 评分错误 (7xxxx)
	ErrRatingNotFound = New(700, CategoryRating, "rating not found", 404)

	// 用户错误 (8xxxx)
	ErrUserAlreadyExists   = New(800, CategoryUser, "user already exists", 409)
	ErrEmailNotVerified    = New(801, CategoryUser, "email not verified", 403)
	ErrInvalidEmailToken   = New(802, CategoryUser, "invalid email token", 400)
	ErrVerificationExpired = New(803, CategoryUser, "verification code expired", 400)
	ErrVerificationSent    = New(804, CategoryUser, "verification code already sent, please check your email", 429)
	ErrEmailAlreadyUsed    = New(805, CategoryUser, "email already used by another user", 409)
)
