package audit

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskSensitiveFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "mask password",
			input:    `{"username":"test","password":"secret123"}`,
			expected: `{"username":"test","password": "***"}`,
		},
		{
			name:     "mask private_key",
			input:    `{"public_key":"abc","private_key":"secret"}`,
			expected: `{"public_key":"abc","private_key": "***"}`,
		},
		{
			name:     "mask verification_code",
			input:    `{"email":"test@example.com","code":"123456"}`,
			expected: `{"email": "***","code": "***"}`, // R3-A: email 也脱敏
		},
		{
			name:     "no sensitive fields",
			input:    `{"name":"test","value":"data"}`,
			expected: `{"name":"test","value":"data"}`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		// R3-A: 标量值（数字/布尔/null）也要掩
		{
			name:     "mask numeric token",
			input:    `{"token":123456789}`,
			expected: `{"token": "***"}`,
		},
		{
			name:     "mask boolean secret",
			input:    `{"secret":true}`,
			expected: `{"secret": "***"}`,
		},
		{
			name:     "mask null api_key",
			input:    `{"api_key":null}`,
			expected: `{"api_key": "***"}`,
		},
		// R3-A: 新增字段表项
		{
			name:     "mask new_password",
			input:    `{"new_password":"abc"}`,
			expected: `{"new_password": "***"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string",
			input:    "hello world this is a long string",
			maxLen:   10,
			expected: "hello worl...[TRUNCATED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetActionType(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"POST", "/api/v1/entry/create", "entry.create"},
		{"POST", "/api/v1/entry/update/entry-123", "entry.update"},
		{"POST", "/api/v1/entry/delete/entry-123", "entry.delete"},
		{"POST", "/api/v1/entry/rate/entry-123", "entry.rate"},
		{"POST", "/api/v1/user/register", "user.register"},
		{"POST", "/api/v1/admin/users/user-pk/ban", "admin.user_ban"},
		{"POST", "/api/v1/admin/users/user-pk/unban", "admin.user_unban"},
		{"PUT", "/api/v1/admin/users/user-pk/level", "admin.user_level"},
		{"GET", "/api/v1/admin/export", "admin.export"},
		{"GET", "/api/v1/search", ""},          // 非敏感操作
		{"GET", "/api/v1/entry/entry-123", ""}, // 非敏感操作
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			result := GetActionType(tt.method, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTargetID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		// /api/v1/entry/update/entry-123 -> parts[4] = "update"
		{"/api/v1/entry/update/entry-123", "update"},
		// /api/v1/entry/delete/entry-456 -> parts[4] = "delete"
		{"/api/v1/entry/delete/entry-456", "delete"},
		// /api/v1/admin/users/user-pk/ban -> parts[5] = "user-pk"
		{"/api/v1/admin/users/user-pk/ban", "user-pk"},
		// /api/v1/elections/election-1/vote -> parts[4] = "election-1"
		{"/api/v1/elections/election-1/vote", "election-1"},
		// /api/v1/entry/create -> parts[4] = "create"
		{"/api/v1/entry/create", "create"},
		// /api/v1/search -> len(parts) < 5
		{"/api/v1/search", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ExtractTargetID(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTargetType(t *testing.T) {
	tests := []struct {
		actionType string
		expected   string
	}{
		{"entry.create", "entry"},
		{"entry.update", "entry"},
		{"user.register", "user"},
		{"admin.user_ban", "admin"},
		{"election.vote", "election"},
		{"batch.create", "batch"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.actionType, func(t *testing.T) {
			result := GetTargetType(tt.actionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSensitiveOperation(t *testing.T) {
	assert.True(t, IsSensitiveOperation("POST", "/api/v1/entry/create"))
	assert.True(t, IsSensitiveOperation("POST", "/api/v1/admin/users/user-pk/ban"))
	assert.False(t, IsSensitiveOperation("GET", "/api/v1/search"))
	assert.False(t, IsSensitiveOperation("GET", "/api/v1/entry/entry-123"))
}

// fakeAuditStore 捕获 Create 收到的 AuditLog，用于断言脱敏后的 body。
type fakeAuditStore struct {
	got *model.AuditLog
}

func (s *fakeAuditStore) Create(ctx context.Context, log *model.AuditLog) error {
	s.got = log
	return nil
}
func (s *fakeAuditStore) Get(ctx context.Context, id string) (*model.AuditLog, error) {
	return nil, nil
}
func (s *fakeAuditStore) List(ctx context.Context, filter model.AuditFilter) ([]*model.AuditLog, int64, error) {
	return nil, 0, nil
}
func (s *fakeAuditStore) DeleteBefore(ctx context.Context, ts int64) (int64, error) { return 0, nil }
func (s *fakeAuditStore) GetStats(ctx context.Context) (*model.AuditStats, error)   { return nil, nil }

// TestService_Log_MasksBothBodies: RequestBody 与 ResponseBody 都必须脱敏。
func TestService_Log_MasksBothBodies(t *testing.T) {
	store := &fakeAuditStore{}
	svc := NewService(store)

	err := svc.Log(context.Background(), &model.AuditLog{
		RequestBody:  `{"password":"secret","email":"u@example.com"}`,
		ResponseBody: `{"token":"abc123","api_key":"k"}`,
	})
	require.NoError(t, err)

	assert.Contains(t, store.got.RequestBody, `"password": "***"`)
	assert.NotContains(t, store.got.RequestBody, "secret")
	assert.Contains(t, store.got.RequestBody, `"email": "***"`)
	assert.NotContains(t, store.got.RequestBody, "u@example.com")

	assert.Contains(t, store.got.ResponseBody, `"token": "***"`)
	assert.NotContains(t, store.got.ResponseBody, "abc123")
	assert.Contains(t, store.got.ResponseBody, `"api_key": "***"`)
}
