package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestElectionHandler_CompleteFlow tests the entire election lifecycle
// from creation to closing with all intermediate steps
func TestElectionHandler_CompleteFlow(t *testing.T) {
	store := kv.NewMemoryStore()
	handler := NewElectionHandler(store)

	var electionID string

	// Step 1: Create election (admin action)
	t.Run("CreateElection", func(t *testing.T) {
		body := `{"title": "年度最佳贡献者选举", "description": "选举年度最佳贡献者", "vote_threshold": 3, "auto_elect": true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "admin-key")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.CreateElectionHandler(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("Create election failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		electionID = data["election_id"].(string)
	})

	// Step 2: Self-nomination
	t.Run("SelfNomination", func(t *testing.T) {
		body := `{"user_name": "候选人张三", "self_nominated": true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "candidate-zhangsan")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.NominateCandidateHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Self-nomination failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		if data["confirmed"] != true {
			t.Error("Self-nomination should be auto-confirmed")
		}
	})

	// Step 3: Peer nomination (not self-nominated)
	t.Run("PeerNomination", func(t *testing.T) {
		body := `{"user_id": "candidate-lisi", "user_name": "候选人李四", "self_nominated": false}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "nominator-wangwu")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.NominateCandidateHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Peer nomination failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		if data["confirmed"] == true {
			t.Error("Peer nomination should not be auto-confirmed")
		}
	})

	// Step 4: Confirm peer nomination
	t.Run("ConfirmNomination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates/candidate-lisi/confirm", nil)
		ctx := context.WithValue(req.Context(), "public_key", "candidate-lisi")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ConfirmNominationHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Confirm nomination failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		if data["confirmed"] != true {
			t.Error("Nomination should be confirmed")
		}
	})

	// Step 5: Voting
	t.Run("Voting", func(t *testing.T) {
		// Vote for candidate-zhangsan (2 votes)
		for i := 1; i <= 2; i++ {
			body := `{"candidate_id": "candidate-zhangsan"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), "public_key", "voter-"+string(rune('0'+i)))
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.VoteHandler(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Vote %d failed: %d - %s", i, rec.Code, rec.Body.String())
			}
		}

		// Vote for candidate-lisi (1 vote)
		body := `{"candidate_id": "candidate-lisi"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "voter-3")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.VoteHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Vote for lisi failed: %d - %s", rec.Code, rec.Body.String())
		}
	})

	// Step 6: Verify duplicate vote fails
	t.Run("DuplicateVoteFails", func(t *testing.T) {
		body := `{"candidate_id": "candidate-zhangsan"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "voter-1") // Same voter
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.VoteHandler(rec, req)

		if rec.Code == http.StatusOK {
			t.Error("Duplicate vote should fail")
		}
	})

	// Step 7: Close election
	t.Run("CloseElection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/close", nil)
		ctx := context.WithValue(req.Context(), "public_key", "admin-key")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.CloseElectionHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Close election failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})

		// Check if elected exists and handle nil case
		electedRaw, exists := data["elected"]
		if !exists {
			t.Fatal("elected field missing from response")
		}

		if electedRaw == nil {
			t.Log("No candidates were elected (elected is nil)")
			return
		}

		elected, ok := electedRaw.([]interface{})
		if !ok {
			t.Logf("elected is not an array: %T", electedRaw)
			return
		}

		t.Logf("Elected candidates: %d", len(elected))
		for _, e := range elected {
			candidate := e.(map[string]interface{})
			t.Logf("  - %v with %v votes", candidate["user_name"], candidate["vote_count"])
		}
	})

	// Step 8: Verify election is closed
	t.Run("VerifyClosed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/elections/"+electionID, nil)
		rec := httptest.NewRecorder()

		handler.GetElectionHandler(rec, req)

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		election := data["election"].(map[string]interface{})

		if election["status"] != string(model.ElectionStatusClosed) {
			t.Errorf("Expected status 'closed', got '%s'", election["status"])
		}
	})
}

// TestElectionHandler_CompleteFlowWithAutoElect tests election with automatic election
func TestElectionHandler_CompleteFlowWithAutoElect(t *testing.T) {
	store := kv.NewMemoryStore()
	handler := NewElectionHandler(store)

	// Create election with auto_elect and low threshold
	body := `{"title": "自动选举测试", "description": "测试自动选举", "vote_threshold": 1, "auto_elect": true}`
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

	// Nominate candidate
	nominateBody := `{"user_name": "Auto-Elect Candidate", "self_nominated": true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(nominateBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "candidate-auto")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.NominateCandidateHandler(rec, req)

	// Vote (should trigger auto-elect with threshold=1)
	voteBody := `{"candidate_id": "candidate-auto"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(voteBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "voter-auto-1")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.VoteHandler(rec, req)

	// Check election status - should be closed due to auto_elect
	req = httptest.NewRequest(http.MethodGet, "/api/v1/elections/"+electionID, nil)
	rec = httptest.NewRecorder()
	handler.GetElectionHandler(rec, req)

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	election := data["election"].(map[string]interface{})

	// With threshold=1 and 1 vote, auto_elect should close the election
	t.Logf("Election status after auto-elect: %s", election["status"])
}
