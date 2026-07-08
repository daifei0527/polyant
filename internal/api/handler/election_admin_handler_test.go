package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/election"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

// memStore is a minimal in-memory kv.Store for tests (same package so
// unexported — avoids pulling in test helpers from other packages).
type memStore struct {
	data map[string][]byte
}

func newMemStore() *memStore { return &memStore{data: make(map[string][]byte)} }

func (s *memStore) Put(key, value []byte) error {
	s.data[string(key)] = make([]byte, len(value))
	copy(s.data[string(key)], value)
	return nil
}

func (s *memStore) Get(key []byte) ([]byte, error) {
	val, ok := s.data[string(key)]
	if !ok {
		return nil, kv.ErrKeyNotFound
	}
	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

func (s *memStore) Delete(key []byte) error { delete(s.data, string(key)); return nil }

func (s *memStore) Scan(prefix []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	pfx := string(prefix)
	for k, v := range s.data {
		if len(k) >= len(pfx) && k[:len(pfx)] == pfx {
			result[k] = v
		}
	}
	return result, nil
}

func (s *memStore) Close() error                { return nil }
func (s *memStore) Backup(destDir string) error { return nil }
func (s *memStore) RunGC() error                { return nil }

var _ kv.Store = (*memStore)(nil)

func newAdminElectionHandler(t *testing.T) (*AdminElectionHandler, kv.Store) {
	t.Helper()
	store := newMemStore()
	svc := election.NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)
	return NewAdminElectionHandler(svc), store
}

func TestAdminCreateElection_UsesSessionPubkey(t *testing.T) {
	h, _ := newAdminElectionHandler(t)
	body := strings.NewReader(`{"title":"T","description":"D","vote_threshold":1,"duration_days":1,"auto_elect":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/elections", body)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "pk-admin"))
	rec := httptest.NewRecorder()
	h.CreateElectionHandler(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminListElections(t *testing.T) {
	h, _ := newAdminElectionHandler(t)
	// Seed via handler's Create endpoint
	body := strings.NewReader(`{"title":"T","description":"D","vote_threshold":1,"duration_days":1,"auto_elect":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/elections", body)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "pk-admin"))
	rec := httptest.NewRecorder()
	h.CreateElectionHandler(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create status=%d body=%s", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/elections", nil)
	rec2 := httptest.NewRecorder()
	h.ListElectionsHandler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	var resp struct {
		Data struct {
			Elections []json.RawMessage `json:"elections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Elections) == 0 {
		t.Error("expected >=1 election")
	}
}

func TestAdminGetElection(t *testing.T) {
	h, _ := newAdminElectionHandler(t)
	// Seed
	body := strings.NewReader(`{"title":"T","description":"D","vote_threshold":1,"duration_days":1,"auto_elect":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/elections", body)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "pk-admin"))
	rec := httptest.NewRecorder()
	h.CreateElectionHandler(rec, req)
	var createResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	eid, _ := createResp.Data["election_id"].(string)

	// Get
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/elections/"+eid, nil)
	rec2 := httptest.NewRecorder()
	h.GetElectionHandler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	var getResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(rec2.Body.Bytes(), &getResp)
	if getResp.Data["election"] == nil {
		t.Error("expected election in response data")
	}
}

func TestAdminCloseElection(t *testing.T) {
	h, _ := newAdminElectionHandler(t)
	// Seed
	body := strings.NewReader(`{"title":"T","description":"D","vote_threshold":1,"duration_days":1,"auto_elect":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/elections", body)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "pk-admin"))
	rec := httptest.NewRecorder()
	h.CreateElectionHandler(rec, req)
	var createResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	eid, _ := createResp.Data["election_id"].(string)

	// Close
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/elections/"+eid+"/close", nil)
	rec2 := httptest.NewRecorder()
	h.CloseElectionHandler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("close status=%d body=%s", rec2.Code, rec2.Body.String())
	}
}
