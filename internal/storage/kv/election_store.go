package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/daifei0527/polyant/internal/storage/model"
)

const (
	electionPrefix  = "election:"
	candidatePrefix = "candidate:"
)

// ElectionStore 选举存储接口
type ElectionStore interface {
	Create(ctx context.Context, election *model.Election) error
	Get(ctx context.Context, id string) (*model.Election, error)
	Update(ctx context.Context, election *model.Election) error
	List(ctx context.Context, status model.ElectionStatus) ([]*model.Election, error)
	Delete(ctx context.Context, id string) error
}

// KVElectionStore KV 选举存储实现
type KVElectionStore struct {
	kv Store
}

// NewElectionStore 创建选举存储
func NewElectionStore(kv Store) *KVElectionStore {
	return &KVElectionStore{kv: kv}
}

func (s *KVElectionStore) Create(ctx context.Context, election *model.Election) error {
	data, err := json.Marshal(election)
	if err != nil {
		return fmt.Errorf("marshal election: %w", err)
	}
	key := []byte(electionPrefix + election.ID)
	return s.kv.Put(key, data)
}

func (s *KVElectionStore) Get(ctx context.Context, id string) (*model.Election, error) {
	key := []byte(electionPrefix + id)
	data, err := s.kv.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get election: %w", err)
	}
	var election model.Election
	if err := json.Unmarshal(data, &election); err != nil {
		return nil, fmt.Errorf("unmarshal election: %w", err)
	}
	return &election, nil
}

func (s *KVElectionStore) Update(ctx context.Context, election *model.Election) error {
	return s.Create(ctx, election)
}

func (s *KVElectionStore) List(ctx context.Context, status model.ElectionStatus) ([]*model.Election, error) {
	prefix := []byte(electionPrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan elections: %w", err)
	}

	var elections []*model.Election
	for _, data := range items {
		var election model.Election
		if err := json.Unmarshal(data, &election); err != nil {
			continue
		}
		if status == "" || election.Status == status {
			elections = append(elections, &election)
		}
	}
	return elections, nil
}

func (s *KVElectionStore) Delete(ctx context.Context, id string) error {
	key := []byte(electionPrefix + id)
	return s.kv.Delete(key)
}

// CandidateStore 候选人存储接口
type CandidateStore interface {
	Add(ctx context.Context, candidate *model.Candidate) error
	Get(ctx context.Context, electionID, userID string) (*model.Candidate, error)
	ListByElection(ctx context.Context, electionID string) ([]*model.Candidate, error)
	UpdateVoteCount(ctx context.Context, electionID, userID string, delta int32) error
	UpdateStatus(ctx context.Context, electionID, userID string, status model.CandidateStatus) error
}

// KVCandidateStore KV 候选人存储实现
type KVCandidateStore struct {
	kv Store
	mu sync.Map // electionID -> *sync.Mutex，保证计票原子性
}

// NewCandidateStore 创建候选人存储
func NewCandidateStore(kv Store) *KVCandidateStore {
	return &KVCandidateStore{kv: kv}
}

func (s *KVCandidateStore) Add(ctx context.Context, candidate *model.Candidate) error {
	data, err := json.Marshal(candidate)
	if err != nil {
		return fmt.Errorf("marshal candidate: %w", err)
	}
	key := []byte(fmt.Sprintf("%s%s:%s", candidatePrefix, candidate.ElectionID, candidate.UserID))
	return s.kv.Put(key, data)
}

func (s *KVCandidateStore) Get(ctx context.Context, electionID, userID string) (*model.Candidate, error) {
	key := []byte(fmt.Sprintf("%s%s:%s", candidatePrefix, electionID, userID))
	data, err := s.kv.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get candidate: %w", err)
	}
	var candidate model.Candidate
	if err := json.Unmarshal(data, &candidate); err != nil {
		return nil, fmt.Errorf("unmarshal candidate: %w", err)
	}
	return &candidate, nil
}

func (s *KVCandidateStore) ListByElection(ctx context.Context, electionID string) ([]*model.Candidate, error) {
	prefix := []byte(fmt.Sprintf("%s%s:", candidatePrefix, electionID))
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan candidates: %w", err)
	}

	var candidates []*model.Candidate
	for _, data := range items {
		var candidate model.Candidate
		if err := json.Unmarshal(data, &candidate); err != nil {
			continue
		}
		candidates = append(candidates, &candidate)
	}
	return candidates, nil
}

// lockFor 返回某选举的计票互斥锁（惰性创建），保证 UpdateVoteCount 的
// 读取-累加-写回是原子的，避免并发投票丢票。
func (s *KVCandidateStore) lockFor(electionID string) *sync.Mutex {
	actual, _ := s.mu.LoadOrStore(electionID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func (s *KVCandidateStore) UpdateVoteCount(ctx context.Context, electionID, userID string, delta int32) error {
	s.lockFor(electionID).Lock()
	defer s.lockFor(electionID).Unlock()

	candidate, err := s.Get(ctx, electionID, userID)
	if err != nil {
		return err
	}
	candidate.VoteCount += delta
	return s.Add(ctx, candidate)
}

func (s *KVCandidateStore) UpdateStatus(ctx context.Context, electionID, userID string, status model.CandidateStatus) error {
	// R2-D3：复用 lockFor(electionID)，与 UpdateVoteCount 共用同一把锁，
	// 串行化"读取-改状态-写回"，避免与并发计票交错产生 lost-update。
	s.lockFor(electionID).Lock()
	defer s.lockFor(electionID).Unlock()

	candidate, err := s.Get(ctx, electionID, userID)
	if err != nil {
		return err
	}
	candidate.Status = status
	return s.Add(ctx, candidate)
}
