// Package errors_test 提供错误处理包的单元测试
package errors_test

import (
	"errors"
	"fmt"
	"testing"

	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

// ==================== AWError 基础测试 ====================

// TestAWErrorError 测试错误字符串格式
func TestAWErrorError(t *testing.T) {
	err := awerrors.New(100, awerrors.CategoryAPI, "test error", 400)

	expected := "[100] test error"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestAWErrorErrorWithCause 测试带原因的错误字符串
func TestAWErrorErrorWithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := awerrors.Wrap(200, awerrors.CategoryAuth, "auth failed", 401, cause)

	expected := "[200] auth failed: underlying error"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestAWErrorUnwrap 测试错误解包
func TestAWErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := awerrors.Wrap(300, awerrors.CategoryStorage, "not found", 404, cause)

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("Unwrap 应返回原始错误")
	}

	// 使用 errors.Is
	if !errors.Is(err, cause) {
		t.Error("errors.Is 应识别底层错误")
	}
}

// TestAWErrorWithRetry 测试设置可重试标志
func TestAWErrorWithRetry(t *testing.T) {
	err := awerrors.New(500, awerrors.CategorySync, "sync failed", 500)

	// 默认不可重试
	if err.Retryable {
		t.Error("默认 Retryable 应为 false")
	}

	// 设置为可重试
	err = err.WithRetry(true)
	if !err.Retryable {
		t.Error("WithRetry(true) 应设置 Retryable 为 true")
	}
}

// ==================== 预定义错误测试 ====================

// TestPredefinedErrors 测试预定义错误值
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *awerrors.AWError
		code       int
		httpStatus int
	}{
		{"ErrInternal", awerrors.ErrInternal, 0, 500},
		{"ErrInvalidParams", awerrors.ErrInvalidParams, 100, 400},
		{"ErrMissingAuth", awerrors.ErrMissingAuth, 200, 401},
		{"ErrEntryNotFound", awerrors.ErrEntryNotFound, 300, 404},
		{"ErrSyncFailed", awerrors.ErrSyncFailed, 500, 500},
		{"ErrSearchFailed", awerrors.ErrSearchFailed, 600, 500},
		{"ErrRatingNotFound", awerrors.ErrRatingNotFound, 700, 404},
		{"ErrUserAlreadyExists", awerrors.ErrUserAlreadyExists, 800, 409},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.code {
				t.Errorf("Code = %d, want %d", tc.err.Code, tc.code)
			}
			if tc.err.HTTPStatus != tc.httpStatus {
				t.Errorf("HTTPStatus = %d, want %d", tc.err.HTTPStatus, tc.httpStatus)
			}
		})
	}
}

// ==================== 错误类别测试 ====================

// TestErrorCategories 测试错误类别常量
func TestErrorCategories(t *testing.T) {
	categories := []awerrors.ErrorCategory{
		awerrors.CategorySystem,
		awerrors.CategoryAPI,
		awerrors.CategoryAuth,
		awerrors.CategoryStorage,
		awerrors.CategoryNetwork,
		awerrors.CategorySync,
		awerrors.CategorySearch,
		awerrors.CategoryRating,
		awerrors.CategoryUser,
		awerrors.CategoryConfig,
	}

	// 验证类别值唯一
	seen := make(map[awerrors.ErrorCategory]bool)
	for i, cat := range categories {
		if seen[cat] {
			t.Errorf("类别重复: %d", cat)
		}
		seen[cat] = true

		// 验证类别值等于索引
		if int(cat) != i {
			t.Errorf("类别 %d 索引应为 %d", cat, i)
		}
	}
}

// ==================== New 和 Wrap 函数测试 ====================

// TestNewError 测试创建新错误
func TestNewError(t *testing.T) {
	err := awerrors.New(999, awerrors.CategoryAPI, "custom error", 418)

	if err.Code != 999 {
		t.Errorf("Code = %d, want 999", err.Code)
	}
	if err.Category != awerrors.CategoryAPI {
		t.Errorf("Category 错误")
	}
	if err.Message != "custom error" {
		t.Errorf("Message = %q, want %q", err.Message, "custom error")
	}
	if err.HTTPStatus != 418 {
		t.Errorf("HTTPStatus = %d, want 418", err.HTTPStatus)
	}
	if err.Cause != nil {
		t.Error("Cause 应为 nil")
	}
}

// TestWrapError 测试包装错误
func TestWrapError(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := awerrors.Wrap(401, awerrors.CategoryNetwork, "peer connect failed", 502, cause)

	if err.Cause != cause {
		t.Error("Cause 应为原始错误")
	}
	if err.Unwrap() != cause {
		t.Error("Unwrap 应返回原始错误")
	}
}

// ==================== 错误链测试 ====================

// TestErrorChain 测试错误链操作
func TestErrorChain(t *testing.T) {
	// 创建错误链: err3 -> err2 -> err1
	err1 := errors.New("level 1 error")
	err2 := awerrors.Wrap(300, awerrors.CategoryStorage, "level 2", 500, err1)
	err3 := awerrors.Wrap(400, awerrors.CategoryNetwork, "level 3", 502, err2)

	// 验证错误链
	if !errors.Is(err3, err2) {
		t.Error("errors.Is 应识别 err2")
	}
	if !errors.Is(err3, err1) {
		t.Error("errors.Is 应识别 err1")
	}

	// 解包
	unwrapped := errors.Unwrap(err3)
	if unwrapped != err2 {
		t.Error("Unwrap 应返回 err2")
	}
}

// ==================== HTTP 状态码测试 ====================

// TestHTTPStatusCodes 测试 HTTP 状态码分配
func TestHTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		err    *awerrors.AWError
		min    int
		max    int
	}{
		{"客户端错误", awerrors.ErrInvalidParams, 400, 499},
		{"服务端错误", awerrors.ErrInternal, 500, 599},
		{"认证错误", awerrors.ErrMissingAuth, 400, 499},
		{"未找到", awerrors.ErrEntryNotFound, 400, 499},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.HTTPStatus < tc.min || tc.err.HTTPStatus > tc.max {
				t.Errorf("HTTPStatus %d 不在范围 [%d, %d]", tc.err.HTTPStatus, tc.min, tc.max)
			}
		})
	}
}

// ==================== 错误码结构测试 ====================

// TestErrorCodeStructure 测试错误码结构 (AABBB)
func TestErrorCodeStructure(t *testing.T) {
	// 系统错误: 0xxxx
	if awerrors.ErrInternal.Code/10000 != 0 {
		t.Error("系统错误码应以 0 开头")
	}

	// API 错误: 1xxxx
	if awerrors.ErrInvalidParams.Code/100 != 1 {
		t.Error("API 错误码应以 1 开头")
	}

	// 认证错误: 2xxxx
	if awerrors.ErrMissingAuth.Code/100 != 2 {
		t.Error("认证错误码应以 2 开头")
	}

	// 存储错误: 3xxxx
	if awerrors.ErrEntryNotFound.Code/100 != 3 {
		t.Error("存储错误码应以 3 开头")
	}
}
