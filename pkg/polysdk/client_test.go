package polysdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeJSON is a test helper to write a JSON response with the given status code
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// apiSuccessResponse wraps data in the standard API response format
func apiSuccessResponse(data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    data,
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8080")
	assert.NotNil(t, c)
	assert.Equal(t, "http://localhost:8080", c.baseURL)
	assert.NotNil(t, c.httpClient)
	assert.Equal(t, 30*time.Second, c.httpClient.Timeout)
	assert.False(t, c.HasKeys())
	assert.Empty(t, c.apiKey)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8080/", "http://localhost:8080"},
		{"http://localhost:8080///", "http://localhost:8080"},
		{"http://localhost:8080", "http://localhost:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c := NewClient(tt.input)
			assert.Equal(t, tt.want, c.baseURL)
		})
	}
}

func TestSetAPIKey(t *testing.T) {
	c := NewClient("http://localhost:8080")
	assert.Empty(t, c.apiKey)

	c.SetAPIKey("test-api-key-123")
	assert.Equal(t, "test-api-key-123", c.apiKey)
}

func TestSearch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/search", r.URL.Path)
		assert.Equal(t, "golang", r.URL.Query().Get("q"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))

		writeJSON(w, http.StatusOK, apiSuccessResponse(map[string]interface{}{
			"total_count": 1,
			"items": []map[string]interface{}{
				{
					"id":          "entry-1",
					"title":       "Go Programming",
					"content":     "Learn Go",
					"category":    "tech",
					"tags":        []string{"go", "programming"},
					"score":       4.5,
					"score_count": 10,
					"created_at":  time.Now().Format(time.RFC3339),
					"updated_at":  time.Now().Format(time.RFC3339),
					"created_by":  "user-1",
				},
			},
		}))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	result, err := c.Search(context.Background(), "golang", "", nil, 10, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	require.Len(t, result.Entries, 1)
	assert.Equal(t, "entry-1", result.Entries[0].ID)
	assert.Equal(t, "Go Programming", result.Entries[0].Title)
	assert.Equal(t, "tech", result.Entries[0].Category)
	assert.Equal(t, []string{"go", "programming"}, result.Entries[0].Tags)
	assert.Equal(t, 4.5, result.Entries[0].Score)
	assert.Equal(t, "user-1", result.Entries[0].CreatedBy)
}

func TestSearch_WithCategoryAndTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ai", r.URL.Query().Get("cat"))
		assert.Equal(t, "ml,deep-learning", r.URL.Query().Get("tag"))

		writeJSON(w, http.StatusOK, apiSuccessResponse(map[string]interface{}{
			"total_count": 2,
			"items": []map[string]interface{}{
				{"id": "1", "title": "Machine Learning", "category": "ai", "tags": []string{"ml"}},
				{"id": "2", "title": "Deep Learning", "category": "ai", "tags": []string{"ml", "deep-learning"}},
			},
		}))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	result, err := c.Search(context.Background(), "learning", "ai", []string{"ml", "deep-learning"}, 20, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
	assert.Len(t, result.Entries, 2)
}

func TestGetEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/entry/abc123", r.URL.Path)

		writeJSON(w, http.StatusOK, apiSuccessResponse(map[string]interface{}{
			"id":          "abc123",
			"title":       "Test Entry",
			"content":     "Test Content",
			"category":    "tech",
			"tags":        []string{"test"},
			"score":       3.5,
			"score_count": 5,
			"created_at":  time.Now().Format(time.RFC3339),
			"updated_at":  time.Now().Format(time.RFC3339),
			"created_by":  "user-1",
		}))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	entry, err := c.GetEntry(context.Background(), "abc123", "")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "abc123", entry.ID)
	assert.Equal(t, "Test Entry", entry.Title)
	assert.Equal(t, "Test Content", entry.Content)
	assert.Equal(t, "tech", entry.Category)
	assert.Equal(t, 3.5, entry.Score)
	assert.Equal(t, 5, entry.ScoreCount)
}

func TestCreateEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/entry/create", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req CreateEntryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "New Entry", req.Title)
		assert.Equal(t, "Content here", req.Content)
		assert.Equal(t, "tech", req.Category)

		writeJSON(w, http.StatusCreated, apiSuccessResponse(map[string]interface{}{
			"id":          "new-id",
			"title":       req.Title,
			"content":     req.Content,
			"category":    req.Category,
			"tags":        req.Tags,
			"score":       0,
			"score_count": 0,
			"created_at":  time.Now().Format(time.RFC3339),
			"updated_at":  time.Now().Format(time.RFC3339),
			"created_by":  "user-1",
		}))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	entry, err := c.CreateEntry(context.Background(), &CreateEntryRequest{
		Title:    "New Entry",
		Content:  "Content here",
		Category: "tech",
		Tags:     []string{"new"},
	})
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "new-id", entry.ID)
	assert.Equal(t, "New Entry", entry.Title)
}

func TestRateEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/entry/rate/entry-123", r.URL.Path)

		var req RatingRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, 4.5, req.Score)
		assert.Equal(t, "Great entry!", req.Comment)

		writeJSON(w, http.StatusCreated, apiSuccessResponse(nil))
	}))
	defer server.Close()

	c := NewClient(server.URL)
	err := c.RateEntry(context.Background(), "entry-123", 4.5, "Great entry!")
	require.NoError(t, err)
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    404,
			"message": "entry not found",
		})
	}))
	defer server.Close()

	c := NewClient(server.URL)
	_, err := c.GetEntry(context.Background(), "nonexistent", "")
	require.Error(t, err)

	var polyErr *Error
	require.ErrorAs(t, err, &polyErr)
	assert.Equal(t, 404, polyErr.Code)
	assert.Equal(t, "entry not found", polyErr.Message)
	assert.True(t, IsNotFoundError(err))
}
