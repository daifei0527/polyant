// Package errors 定义了 Polyant 项目的统一错误类型和预定义错误码。
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

// AWError Polyant 统一错误类型
type AWError struct {
	Code       int           `json:"code"`
	Category   ErrorCategory `json:"-"`
	Message    string        `json:"message"`      // 默认英文消息
	I18nCode   string        `json:"i18n_code"`    // i18n 消息码
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

// NewWithI18n 创建带多语言支持的错误
func NewWithI18n(code int, category ErrorCategory, i18nCode, defaultMessage string, httpStatus int) *AWError {
	return &AWError{
		Code:       code,
		Category:   category,
		Message:    defaultMessage,
		I18nCode:   i18nCode,
		HTTPStatus: httpStatus,
	}
}

// WrapWithI18n 创建带多语言支持和底层错误的错误
func WrapWithI18n(code int, category ErrorCategory, i18nCode, defaultMessage string, httpStatus int, cause error) *AWError {
	return &AWError{
		Code:       code,
		Category:   category,
		Message:    defaultMessage,
		I18nCode:   i18nCode,
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
	ErrInternal    = NewWithI18n(0, CategorySystem, "common.internal_error", "internal error", 500)
	ErrUnavailable = NewWithI18n(1, CategorySystem, "common.internal_error", "service unavailable", 503)
	ErrRateLimited = NewWithI18n(2, CategorySystem, "common.internal_error", "rate limited", 429)

	// API错误 (1xxxx)
	ErrInvalidParams    = NewWithI18n(100, CategoryAPI, "common.invalid_params", "invalid params", 400)
	ErrJSONParse        = NewWithI18n(102, CategoryAPI, "common.invalid_params", "json parse failed", 400)
	ErrScoreOutOfRange  = NewWithI18n(103, CategoryAPI, "common.invalid_params", "score must be between 1.0 and 5.0", 400)

	// 认证错误 (2xxxx)
	ErrMissingAuth      = NewWithI18n(200, CategoryAuth, "api.auth.missing", "missing auth info", 401)
	ErrInvalidSignature = NewWithI18n(201, CategoryAuth, "api.auth.invalid_signature", "invalid signature", 401)
	ErrTimestampExpired = NewWithI18n(202, CategoryAuth, "api.auth.expired", "timestamp expired", 401)
	ErrPermissionDenied = NewWithI18n(203, CategoryAuth, "api.auth.permission_denied", "permission denied", 403)
	ErrBasicUserDenied  = NewWithI18n(204, CategoryAuth, "api.auth.permission_denied", "basic user cannot perform this action", 403)
	ErrUserSuspended    = NewWithI18n(205, CategoryAuth, "api.auth.user_suspended", "user is suspended", 403)

	// 存储错误 (3xxxx)
	ErrEntryNotFound    = NewWithI18n(300, CategoryStorage, "api.entry.not_found", "entry not found", 404)
	ErrUserNotFound     = NewWithI18n(301, CategoryStorage, "api.user.not_found", "user not found", 404)
	ErrCategoryNotFound = NewWithI18n(302, CategoryStorage, "api.category.not_found", "category not found", 404)
	ErrDuplicateRating  = NewWithI18n(303, CategoryStorage, "common.internal_error", "duplicate rating", 409)
	ErrEntryExists      = NewWithI18n(304, CategoryStorage, "common.internal_error", "entry already exists", 409)
	ErrWriteFailed      = NewWithI18n(305, CategoryStorage, "common.internal_error", "storage write failed", 500)

	// 网络错误 (4xxxx)
	ErrPeerConnectFailed = NewWithI18n(400, CategoryNetwork, "common.internal_error", "peer connect failed", 502)

	// 同步错误 (5xxxx)
	ErrSyncFailed   = NewWithI18n(500, CategorySync, "common.internal_error", "sync failed", 500)
	ErrHashMismatch = NewWithI18n(502, CategorySync, "common.internal_error", "hash mismatch", 500)

	// 搜索错误 (6xxxx)
	ErrSearchFailed    = NewWithI18n(600, CategorySearch, "common.internal_error", "search failed", 500)
	ErrKeywordTooShort = NewWithI18n(601, CategorySearch, "api.search.keyword_too_short", "keyword too short", 400)

	// 评分错误 (7xxxx)
	ErrRatingNotFound = NewWithI18n(700, CategoryRating, "common.not_found", "rating not found", 404)

	// 用户错误 (8xxxx)
	ErrUserAlreadyExists   = NewWithI18n(800, CategoryUser, "common.internal_error", "user already exists", 409)
	ErrEmailNotVerified    = NewWithI18n(801, CategoryUser, "common.invalid_params", "email not verified", 403)
	ErrInvalidEmailToken   = NewWithI18n(802, CategoryUser, "common.invalid_params", "invalid email token", 400)
	ErrVerificationExpired = NewWithI18n(803, CategoryUser, "common.invalid_params", "verification code expired", 400)
	ErrVerificationSent    = NewWithI18n(804, CategoryUser, "common.internal_error", "verification code already sent, please check your email", 429)
	ErrEmailAlreadyUsed    = NewWithI18n(805, CategoryUser, "common.internal_error", "email already used by another user", 409)
)
