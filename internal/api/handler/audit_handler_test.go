package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/core/audit"
	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestAuditHandler(t *testing.T) *AuditHandler {
	store := kv.NewMemoryStore()
	auditStore := kv.NewAuditStore(store)
	return NewAuditHandler(auditStore)
}

func newTestAuditHandlerWithLogs(t *testing.T, count int) *AuditHandler {
	store := kv.NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	// Create some test logs
	svc := audit.NewService(auditStore)
	for i := 0; i < count; i++ {
		svc.Log(context.Background(), &model.AuditLog{
			ActionType:     "entry.create",
			OperatorPubkey: "operator-" + string(rune('a'+i%3)),
			TargetID:       "target-" + string(rune('a'+i%5)),
			Success:        true,
			Timestamp:      time.Now().Add(-time.Duration(i) * time.Hour).UnixMilli(),
		})
	}

	return NewAuditHandler(auditStore)
}

func TestAuditHandler_ListAuditLogsHandler(t *testing.T) {
	handler := newTestAuditHandlerWithLogs(t, 10)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/logs?limit=5", nil)
	rec := httptest.NewRecorder()

	handler.ListAuditLogsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["total_count"] == nil {
		t.Error("Expected total_count in response")
	}
	if data["items"] == nil {
		t.Error("Expected items in response")
	}
}

func TestAuditHandler_ListAuditLogsHandler_WithFilters(t *testing.T) {
	handler := newTestAuditHandlerWithLogs(t, 10)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/logs?operator=operator-a&action=entry.create&success=true", nil)
	rec := httptest.NewRecorder()

	handler.ListAuditLogsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestAuditHandler_ListAuditLogsHandler_MethodNotAllowed(t *testing.T) {
	handler := newTestAuditHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/audit/logs", nil)
	rec := httptest.NewRecorder()

	handler.ListAuditLogsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAuditHandler_GetAuditStatsHandler(t *testing.T) {
	handler := newTestAuditHandlerWithLogs(t, 10)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetAuditStatsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	// Stats should be returned
	if resp.Data == nil {
		t.Error("Expected stats in response")
	}
}

func TestAuditHandler_GetAuditStatsHandler_MethodNotAllowed(t *testing.T) {
	handler := newTestAuditHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/audit/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetAuditStatsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestAuditHandler_DeleteAuditLogsHandler(t *testing.T) {
	handler := newTestAuditHandlerWithLogs(t, 10)

	// Delete logs older than 5 hours ago
	before := time.Now().Add(-5 * time.Hour).UnixMilli()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/audit/logs?before="+fmt.Sprintf("%d", before), nil)
	rec := httptest.NewRecorder()

	handler.DeleteAuditLogsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["deleted_count"] == nil {
		t.Error("Expected deleted_count in response")
	}
}

func TestAuditHandler_DeleteAuditLogsHandler_MissingBefore(t *testing.T) {
	handler := newTestAuditHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/audit/logs", nil)
	rec := httptest.NewRecorder()

	handler.DeleteAuditLogsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAuditHandler_DeleteAuditLogsHandler_MethodNotAllowed(t *testing.T) {
	handler := newTestAuditHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/logs?before=123456", nil)
	rec := httptest.NewRecorder()

	handler.DeleteAuditLogsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
