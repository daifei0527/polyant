package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			expected: `{"email":"test@example.com","code": "***"}`,
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
		{"GET", "/api/v1/search", ""},           // 非敏感操作
		{"GET", "/api/v1/entry/entry-123", ""},  // 非敏感操作
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
