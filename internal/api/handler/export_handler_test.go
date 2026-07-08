package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/export"
	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
)

func newTestExportHandler(t *testing.T) (*ExportHandler, *storage.Store) {
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	handler := NewExportHandler(store, "test-node-1")
	return handler, store
}

func TestExportHandler_ExportHandler(t *testing.T) {
	handler, store := newTestExportHandler(t)

	// Create some test data
	entry := &model.KnowledgeEntry{
		ID:       "test-entry-1",
		Title:    "Test Entry",
		Content:  "Test Content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	store.Entry.Create(context.Background(), entry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/export?include=entries,categories", nil)
	rec := httptest.NewRecorder()

	handler.ExportHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Check response headers
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("Expected Content-Type 'application/zip', got '%s'", contentType)
	}

	contentDisposition := rec.Header().Get("Content-Disposition")
	if contentDisposition == "" {
		t.Error("Expected Content-Disposition header")
	}
}

func TestExportHandler_ExportHandler_DefaultInclude(t *testing.T) {
	handler, _ := newTestExportHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/export", nil)
	rec := httptest.NewRecorder()

	handler.ExportHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestExportHandler_ExportHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestExportHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/export", nil)
	rec := httptest.NewRecorder()

	handler.ExportHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestExportHandler_ImportHandler(t *testing.T) {
	handler, store := newTestExportHandler(t)

	// Create test data and export it first
	entry := &model.KnowledgeEntry{
		ID:       "test-entry-import",
		Title:    "Test Entry for Import",
		Content:  "Test Content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	store.Entry.Create(context.Background(), entry)

	// Export
	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/export?include=entries", nil)
	exportRec := httptest.NewRecorder()
	handler.ExportHandler(exportRec, exportReq)

	zipData := exportRec.Body.Bytes()

	// Create multipart form with the ZIP file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile("file", "test.zip")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write(zipData)

	// Add conflict strategy field
	writer.WriteField("conflict", "skip")

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.ImportHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["success"] == nil {
		t.Error("Expected success in response")
	}
}

func TestExportHandler_ImportHandler_MissingFile(t *testing.T) {
	handler, _ := newTestExportHandler(t)

	// Create multipart form without file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("conflict", "skip")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.ImportHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestExportHandler_ImportHandler_MethodNotAllowed(t *testing.T) {
	handler, _ := newTestExportHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/import", nil)
	rec := httptest.NewRecorder()

	handler.ImportHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// TestImportHandler_OperatorLevelFromSession verifies that Import sources
// OperatorLevel from the session context (admin publicKey -> UserStore -> level)
// by round-tripping an export ZIP under a session-auth context.
func TestImportHandler_OperatorLevelFromSession(t *testing.T) {
	// setup: memory store with an admin user (Lv4) and one entry
	store, err := storage.NewMemoryStore()
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	admin := &model.User{
		PublicKey: "pk-admin-session-test",
		UserLevel: model.UserLevelLv4,
		Status:    model.UserStatusActive,
		AgentName: "admin",
	}
	if _, err := store.User.Create(context.Background(), admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := store.Entry.Create(context.Background(), &model.KnowledgeEntry{
		ID:       "e1",
		Title:    "T",
		Content:  "C",
		Category: "x",
		Status:   model.EntryStatusPublished,
	}); err != nil {
		t.Fatalf("create entry: %v", err)
	}

	// build an export ZIP via ExportHandler (include users so OperatorLevel is exercised)
	exh := NewExportHandler(store, "test-node")
	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/export?include=entries,users", nil)
	exportReq = exportReq.WithContext(context.WithValue(exportReq.Context(), mw.PublicKeyKey, "pk-admin-session-test"))
	exportRec := httptest.NewRecorder()
	exh.ExportHandler(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export: code=%d body=%s", exportRec.Code, exportRec.Body.String())
	}
	if exportRec.Body.Len() == 0 {
		t.Fatal("export: empty body")
	}
	zipBytes := exportRec.Body.Bytes()

	// import the ZIP via ImportHandler with a session context (admin publicKey)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", "export.zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fw.Write(zipBytes)
	_ = writer.WriteField("conflict", "skip")
	writer.Close()

	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/import", body)
	importReq.Header.Set("Content-Type", writer.FormDataContentType())
	importReq = importReq.WithContext(context.WithValue(importReq.Context(), mw.PublicKeyKey, "pk-admin-session-test"))
	importRec := httptest.NewRecorder()
	exh.ImportHandler(importRec, importReq)

	if importRec.Code != http.StatusOK {
		t.Fatalf("import: code=%d body=%s", importRec.Code, importRec.Body.String())
	}
	var resp struct {
		Code int                  `json:"code"`
		Data *export.ImportResult `json:"data"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, importRec.Body.String())
	}
	if resp.Data == nil {
		t.Fatal("import: data is nil")
	}
	if !resp.Data.Success {
		t.Errorf("import not successful: %+v", resp.Data.Errors)
	}
	// Verify that the admin user (Lv4) was NOT skipped due to level check.
	// With the session-sourced OperatorLevel (Lv4), 4 > 4 is false, so the
	// admin user passes the gate. Before the fix (OperatorLevel=0), every
	// user would be skipped with a "cannot import user with higher level"
	// warning.
	for _, e := range resp.Data.Errors {
		if e.Message == "cannot import user with higher level" {
			t.Errorf("admin user (Lv4) was incorrectly skipped by level gate; OperatorLevel was likely 0 instead of admin's Lv4")
		}
	}
}
