package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

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
		ID:        "test-entry-1",
		Title:     "Test Entry",
		Content:   "Test Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
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
		ID:        "test-entry-import",
		Title:     "Test Entry for Import",
		Content:   "Test Content",
		Category:  "test",
		Status:    model.EntryStatusPublished,
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

// Helper function to create multipart body for testing
func createMultipartBody(fieldName, filename string, fileContent []byte, fields map[string]string) (io.Reader, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile(fieldName, filename)
	part.Write(fileContent)

	for key, value := range fields {
		writer.WriteField(key, value)
	}

	writer.Close()
	return body, writer.FormDataContentType()
}
