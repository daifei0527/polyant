package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEntryCounter implements EntryCounter for testing
type mockEntryCounter struct {
	count int64
	err   error
}

func (m *mockEntryCounter) Count(_ interface{}) (int64, error) {
	return m.count, m.err
}

// ========== GetNodeStatusHandler ==========

func TestNodeHandler_GetNodeStatusHandler(t *testing.T) {
	handler := NewNodeHandler("node-1", "seed", "1.0.0", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/status", nil)
	rec := httptest.NewRecorder()

	handler.GetNodeStatusHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp APIResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "success", resp.Message)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a map")
	assert.Equal(t, "node-1", data["node_id"])
	assert.Equal(t, "seed", data["node_type"])
	assert.Equal(t, "1.0.0", data["version"])
	assert.Equal(t, float64(0), data["entry_count"])
	assert.Equal(t, float64(0), data["last_sync"])
}

func TestNodeHandler_GetNodeStatusHandler_WithEntryStore(t *testing.T) {
	mock := &mockEntryCounter{count: 42}
	handler := NewNodeHandler("node-2", "user", "2.0.0", mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/status", nil)
	rec := httptest.NewRecorder()

	handler.GetNodeStatusHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp APIResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a map")
	assert.Equal(t, "node-2", data["node_id"])
	assert.Equal(t, "user", data["node_type"])
	assert.Equal(t, "2.0.0", data["version"])
	assert.Equal(t, float64(42), data["entry_count"])
}

// ========== TriggerSyncHandler ==========

func TestNodeHandler_TriggerSyncHandler(t *testing.T) {
	handler := NewNodeHandler("node-1", "seed", "1.0.0", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/sync", nil)
	rec := httptest.NewRecorder()

	handler.TriggerSyncHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp APIResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "sync triggered", resp.Message)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a map")
	assert.Equal(t, "syncing", data["status"])

	triggeredAt, exists := data["triggered_at"]
	assert.True(t, exists, "triggered_at should exist")
	assert.Greater(t, triggeredAt.(float64), float64(0), "triggered_at should be positive")
}

// ========== SetLastSync ==========

func TestNodeHandler_SetLastSync(t *testing.T) {
	handler := NewNodeHandler("node-1", "seed", "1.0.0", nil)

	// Set a known lastSync value
	handler.SetLastSync(1234567890)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/status", nil)
	rec := httptest.NewRecorder()

	handler.GetNodeStatusHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp APIResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a map")
	assert.Equal(t, float64(1234567890), data["last_sync"])
}
