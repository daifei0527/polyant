package election

import (
	"context"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MemoryStore 内存存储实现（用于测试）
type MemoryStore struct {
	data map[string][]byte
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string][]byte),
	}
}

func (s *MemoryStore) Put(key, value []byte) error {
	s.data[string(key)] = make([]byte, len(value))
	copy(s.data[string(key)], value)
	return nil
}

func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	val, ok := s.data[string(key)]
	if !ok {
		return nil, kv.ErrKeyNotFound
	}
	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

func (s *MemoryStore) Delete(key []byte) error {
	delete(s.data, string(key))
	return nil
}

func (s *MemoryStore) Scan(prefix []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	prefixStr := string(prefix)
	for k, v := range s.data {
		if len(k) >= len(prefixStr) && k[:len(prefixStr)] == prefixStr {
			result[k] = v
		}
	}
	return result, nil
}

func (s *MemoryStore) Close() error {
	return nil
}

// 确保实现 kv.Store 接口
var _ kv.Store = (*MemoryStore)(nil)

func TestElectionService_CreateElection(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, err := service.CreateElection(ctx, "Test Election", "Description", "creator1", 5, 0)

	require.NoError(t, err)
	assert.NotEmpty(t, election.ID)
	assert.Equal(t, "Test Election", election.Title)
	assert.Equal(t, model.ElectionStatusActive, election.Status)
}

func TestElectionService_NominateCandidate(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 3, 0)

	err := service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1")
	require.NoError(t, err)

	candidates, _ := service.ListCandidates(ctx, election.ID)
	assert.Len(t, candidates, 1)
	assert.Equal(t, "candidate1", candidates[0].UserID)
}

func TestElectionService_Vote(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 2, 0)
	service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1")

	err := service.Vote(ctx, election.ID, "voter1", "candidate1")
	require.NoError(t, err)

	hasVoted, _ := service.HasVoted(ctx, "voter1", election.ID)
	assert.True(t, hasVoted)

	// 重复投票应该失败
	err = service.Vote(ctx, election.ID, "voter1", "candidate1")
	assert.Error(t, err)
}

func TestElectionService_CloseElection(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 2, 0)
	service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1")
	service.NominateCandidate(ctx, election.ID, "candidate2", "Candidate Two", "nominator1")

	// 投票
	service.Vote(ctx, election.ID, "voter1", "candidate1")
	service.Vote(ctx, election.ID, "voter2", "candidate1")
	service.Vote(ctx, election.ID, "voter3", "candidate2")

	// 关闭选举
	elected, err := service.CloseElection(ctx, election.ID)
	require.NoError(t, err)
	assert.Len(t, elected, 1)
	assert.Equal(t, "candidate1", elected[0].UserID)
	assert.Equal(t, model.CandidateStatusElected, elected[0].Status)
}

func TestElectionService_NominateCandidate_ClosedElection(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 3, 0)

	// 关闭选举
	service.CloseElection(ctx, election.ID)

	// 尝试提名应该失败
	err := service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1")
	assert.Error(t, err)
	assert.Equal(t, ErrElectionClosed, err)
}

func TestElectionService_Vote_ClosedElection(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 3, 0)
	service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1")

	// 关闭选举
	service.CloseElection(ctx, election.ID)

	// 尝试投票应该失败
	err := service.Vote(ctx, election.ID, "voter1", "candidate1")
	assert.Error(t, err)
	assert.Equal(t, ErrElectionClosed, err)
}

func TestElectionService_NominateCandidate_Duplicate(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 3, 0)

	// 第一次提名
	err := service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator1")
	require.NoError(t, err)

	// 重复提名应该失败
	err = service.NominateCandidate(ctx, election.ID, "candidate1", "Candidate One", "nominator2")
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyNominated, err)
}

func TestElectionService_Vote_NonExistentCandidate(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 3, 0)

	// 投票给不存在的候选人应该失败
	err := service.Vote(ctx, election.ID, "voter1", "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrCandidateNotFound, err)
}

func TestElectionService_GetElection_NotFound(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()

	_, err := service.GetElection(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrElectionNotFound, err)
}

func TestElectionService_ListElections(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()

	// 创建多个选举
	service.CreateElection(ctx, "Election 1", "Desc 1", "creator1", 3, 0)
	service.CreateElection(ctx, "Election 2", "Desc 2", "creator2", 5, 0)

	// 列出所有活跃选举
	elections, err := service.ListElections(ctx, model.ElectionStatusActive)
	require.NoError(t, err)
	assert.Len(t, elections, 2)
}

func TestElectionService_CloseElection_AlreadyClosed(t *testing.T) {
	store := NewMemoryStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
	)

	ctx := context.Background()
	election, _ := service.CreateElection(ctx, "Test", "Desc", "creator1", 3, 0)

	// 第一次关闭
	_, err := service.CloseElection(ctx, election.ID)
	require.NoError(t, err)

	// 再次关闭应该失败
	_, err = service.CloseElection(ctx, election.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrElectionClosed, err)
}
