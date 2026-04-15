package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
)

func newTestElectionHandler(t *testing.T) *ElectionHandler {
	store := kv.NewMemoryStore()
	return NewElectionHandler(store)
}

func TestElectionHandler_CreateElectionHandler(t *testing.T) {
	handler := newTestElectionHandler(t)

	body := `{"title": "Test Election", "description": "Test Description", "vote_threshold": 5, "auto_elect": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateElectionHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["election_id"] == nil {
		t.Error("Expected election_id in response")
	}
	if data["auto_elect"] != true {
		t.Error("Expected auto_elect to be true")
	}
}

func TestElectionHandler_CreateElectionHandler_MethodNotAllowed(t *testing.T) {
	handler := newTestElectionHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/elections", nil)
	rec := httptest.NewRecorder()

	handler.CreateElectionHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestElectionHandler_ListElectionsHandler(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election first
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	// List elections
	req = httptest.NewRequest(http.MethodGet, "/api/v1/elections?status=active", nil)
	rec = httptest.NewRecorder()

	handler.ListElectionsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["elections"] == nil {
		t.Error("Expected elections in response")
	}
}

func TestElectionHandler_GetElectionHandler(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election first
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	createData := createResp.Data.(map[string]interface{})
	electionID := createData["election_id"].(string)

	// Get election
	req = httptest.NewRequest(http.MethodGet, "/api/v1/elections/"+electionID, nil)
	rec = httptest.NewRecorder()

	handler.GetElectionHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["election"] == nil {
		t.Error("Expected election in response")
	}
}

func TestElectionHandler_GetElectionHandler_NotFound(t *testing.T) {
	handler := newTestElectionHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/elections/nonexistent", nil)
	rec := httptest.NewRecorder()

	handler.GetElectionHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestElectionHandler_NominateCandidateHandler_SelfNomination(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election first
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	createData := createResp.Data.(map[string]interface{})
	electionID := createData["election_id"].(string)

	// Self-nominate
	nominateBody := `{"user_name": "Candidate Name", "self_nominated": true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(nominateBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "candidate-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.NominateCandidateHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["success"] != true {
		t.Error("Expected success to be true")
	}
	if data["self_nominated"] != true {
		t.Error("Expected self_nominated to be true")
	}
	if data["confirmed"] != true {
		t.Error("Self-nomination should be auto-confirmed")
	}
}

func TestElectionHandler_VoteHandler(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 2, "auto_elect": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	createData := createResp.Data.(map[string]interface{})
	electionID := createData["election_id"].(string)

	// Nominate a candidate (self-nomination)
	nominateBody := `{"user_name": "Candidate", "self_nominated": true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(nominateBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "candidate-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.NominateCandidateHandler(rec, req)

	// Vote
	voteBody := `{"candidate_id": "candidate-key"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(voteBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "voter-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.VoteHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestElectionHandler_CloseElectionHandler(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	createData := createResp.Data.(map[string]interface{})
	electionID := createData["election_id"].(string)

	// Close election
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/close", nil)
	ctx = context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.CloseElectionHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Response data is not a map, got: %v", resp.Data)
	}

	// "elected" should be an array (possibly empty)
	_, exists := data["elected"]
	if !exists {
		t.Errorf("Expected elected in response, got keys: %v", getKeys(data))
	}
}

// Helper function to get map keys
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestElectionHandler_ConfirmNominationHandler(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	createData := createResp.Data.(map[string]interface{})
	electionID := createData["election_id"].(string)

	// Peer nomination (not self-nominated)
	nominateBody := `{"user_id": "nominee-key", "user_name": "Nominee", "self_nominated": false}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(nominateBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "nominator-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.NominateCandidateHandler(rec, req)

	// Confirm nomination (by the nominee)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates/nominee-key/confirm", nil)
	ctx = context.WithValue(req.Context(), "public_key", "nominee-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.ConfirmNominationHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if data["confirmed"] != true {
		t.Error("Expected confirmed to be true")
	}
}

func TestElectionHandler_ConfirmNominationHandler_WrongUser(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create an election
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	createData := createResp.Data.(map[string]interface{})
	electionID := createData["election_id"].(string)

	// Peer nomination
	nominateBody := `{"user_id": "nominee-key", "user_name": "Nominee", "self_nominated": false}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(nominateBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "nominator-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.NominateCandidateHandler(rec, req)

	// Try to confirm with wrong user
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates/nominee-key/confirm", nil)
	ctx = context.WithValue(req.Context(), "public_key", "wrong-user")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.ConfirmNominationHandler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestExtractLastPathParam(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/v1/elections/abc123", "abc123"},
		{"/api/v1/elections/abc123/", "abc123"},
		{"/api/v1/elections/", "elections"}, // trailing slash is trimmed, last segment is "elections"
		{"elections/test", "test"},
		{"single", "single"},
	}

	for _, tt := range tests {
		result := extractLastPathParam(tt.path)
		if result != tt.expected {
			t.Errorf("extractLastPathParam(%q) = %q, expected %q", tt.path, result, tt.expected)
		}
	}
}

func TestExtractPathParam(t *testing.T) {
	tests := []struct {
		path     string
		prefix   string
		suffix   string
		expected string
	}{
		{"/api/v1/elections/abc123/vote", "/api/v1/elections/", "/vote", "abc123"},
		{"/api/v1/elections/abc123/candidates", "/api/v1/elections/", "/candidates", "abc123"},
		{"/api/v1/elections/abc123/close", "/api/v1/elections/", "/close", "abc123"},
		{"/wrong/prefix", "/api/v1/elections/", "/vote", ""},
	}

	for _, tt := range tests {
		result := extractPathParam(tt.path, tt.prefix, tt.suffix)
		if result != tt.expected {
			t.Errorf("extractPathParam(%q, %q, %q) = %q, expected %q",
				tt.path, tt.prefix, tt.suffix, result, tt.expected)
		}
	}
}
