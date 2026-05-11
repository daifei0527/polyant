package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	awerrors "github.com/daifei0527/polyant/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== extractPathVar tests ====================

func TestExtractPathVar_ID(t *testing.T) {
	tests := []struct {
		name     string
		urlPath  string
		expected string
	}{
		{
			name:     "standard entry URL /api/v1/entry/{id}",
			urlPath:  "/api/v1/entry/abc-123-def",
			expected: "abc-123-def",
		},
		{
			name:     "UUID id",
			urlPath:  "/api/v1/entry/550e8400-e29b-41d4-a716-446655440000",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "entry update URL /api/v1/entry/update/{id}",
			urlPath:  "/api/v1/entry/update/my-id-456",
			expected: "my-id-456",
		},
		{
			name:     "entry with backlinks /api/v1/entry/{id}/backlinks",
			urlPath:  "/api/v1/entry/test-id-789/backlinks",
			expected: "test-id-789",
		},
		{
			name:     "trailing slash stripped",
			urlPath:  "/api/v1/entry/trailing-id/",
			expected: "trailing-id",
		},
		{
			name:     "short path returns empty",
			urlPath:  "/api/v1/entry",
			expected: "",
		},
		{
			name:     "excluded last segment rate",
			urlPath:  "/api/v1/entry/xyz/rate",
			expected: "xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.urlPath, nil)
			result := extractPathVar(req, "id")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPathVar_Path(t *testing.T) {
	tests := []struct {
		name     string
		urlPath  string
		expected string
	}{
		{
			name:     "single category /api/v1/categories/programming/entries",
			urlPath:  "/api/v1/categories/programming/entries",
			expected: "programming",
		},
		{
			name:     "nested category /api/v1/categories/tech/ai/entries",
			urlPath:  "/api/v1/categories/tech/ai/entries",
			expected: "tech/ai",
		},
		{
			name:     "deep nested category",
			urlPath:  "/api/v1/categories/a/b/c/entries",
			expected: "a/b/c",
		},
		{
			name:     "missing entries segment returns empty",
			urlPath:  "/api/v1/categories/programming",
			expected: "",
		},
		{
			name:     "no categories segment returns empty",
			urlPath:  "/api/v1/other/programming/entries",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.urlPath, nil)
			result := extractPathVar(req, "path")
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== generateUUID tests ====================

func TestGenerateUUID(t *testing.T) {
	// UUID v4 format: 8-4-4-4-12 hex chars = 36 chars total with dashes
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	uuid := generateUUID()
	assert.Len(t, uuid, 36, "UUID should be 36 characters long")
	assert.Regexp(t, uuidRegex, uuid, "UUID should match v4 format")

	// Verify uniqueness: generate 100 UUIDs and ensure no duplicates
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateUUID()
		assert.Len(t, id, 36)
		assert.False(t, seen[id], "UUID should be unique, got duplicate: %s", id)
		seen[id] = true
	}
}

// ==================== parseInt tests ====================

func TestParseInt(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  int
		expectErr bool
	}{
		{name: "positive integer", input: "42", expected: 42, expectErr: false},
		{name: "zero", input: "0", expected: 0, expectErr: false},
		{name: "negative integer", input: "-7", expected: -7, expectErr: false},
		{name: "large positive", input: "999999", expected: 999999, expectErr: false},
		{name: "empty string", input: "", expectErr: true},
		{name: "non-numeric string", input: "abc", expectErr: true},
		{name: "float string", input: "3.14", expectErr: true},
		{name: "mixed alphanumeric", input: "12ab", expectErr: true},
		{name: "whitespace", input: " ", expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInt(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// ==================== writeJSON tests ====================

func TestWriteJSON(t *testing.T) {
	t.Run("writes status code and content type", func(t *testing.T) {
		rec := httptest.NewRecorder()
		data := map[string]string{"key": "value"}

		writeJSON(rec, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

		var parsed map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &parsed)
		require.NoError(t, err)
		assert.Equal(t, "value", parsed["key"])
	})

	t.Run("writes 201 status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeJSON(rec, http.StatusCreated, nil)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
		// nil data should not write a body (or write empty)
	})

	t.Run("writes error status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeJSON(rec, http.StatusNotFound, map[string]string{"error": "not found"})

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("writes APIResponse struct", func(t *testing.T) {
		rec := httptest.NewRecorder()
		resp := &APIResponse{Code: 0, Message: "ok", Data: "test"}
		writeJSON(rec, http.StatusOK, resp)

		var parsed APIResponse
		err := json.Unmarshal(rec.Body.Bytes(), &parsed)
		require.NoError(t, err)
		assert.Equal(t, 0, parsed.Code)
		assert.Equal(t, "ok", parsed.Message)
		assert.Equal(t, "test", parsed.Data)
	})
}

// ==================== writeError tests ====================

func TestWriteError(t *testing.T) {
	t.Run("writes error with HTTP status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := awerrors.New(100, awerrors.CategoryAPI, "invalid params", http.StatusBadRequest)

		writeError(rec, err)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

		var parsed APIResponse
		err2 := json.Unmarshal(rec.Body.Bytes(), &parsed)
		require.NoError(t, err2)
		assert.Equal(t, 100, parsed.Code)
		assert.Equal(t, "invalid params", parsed.Message)
		assert.Nil(t, parsed.Data)
	})

	t.Run("defaults to 500 when HTTPStatus is 0", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := &awerrors.AWError{Code: 999, Message: "unknown error", HTTPStatus: 0}

		writeError(rec, err)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var parsed APIResponse
		err2 := json.Unmarshal(rec.Body.Bytes(), &parsed)
		require.NoError(t, err2)
		assert.Equal(t, 999, parsed.Code)
		assert.Equal(t, "unknown error", parsed.Message)
	})

	t.Run("writes predefined error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeError(rec, awerrors.ErrEntryNotFound)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var parsed APIResponse
		err := json.Unmarshal(rec.Body.Bytes(), &parsed)
		require.NoError(t, err)
		assert.Equal(t, 300, parsed.Code)
		assert.Equal(t, "entry not found", parsed.Message)
	})
}

// ==================== isValidEmail tests ====================

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{name: "valid email", email: "user@example.com", expected: true},
		{name: "valid with subdomain", email: "user@mail.example.com", expected: true},
		{name: "valid with plus", email: "user+tag@example.com", expected: true},
		{name: "valid with numbers", email: "user123@example.com", expected: true},
		{name: "missing @", email: "userexample.com", expected: false},
		{name: "missing domain dot", email: "user@examplecom", expected: false},
		{name: "missing everything", email: "user", expected: false},
		{name: "empty string", email: "", expected: false},
		{name: "only at sign", email: "@", expected: false},
		{name: "only dot", email: ".", expected: false},
		{name: "at and dot reversed", email: ".user@com", expected: true}, // simple check: has @ and .
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}
