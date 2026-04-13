package kv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

const (
	votePrefix      = "vote:"
	votesByVoterKey = "votes:voter:"
)

// VoteStore 投票存储接口
type VoteStore interface {
	Create(ctx context.Context, vote *model.Vote) error
	Get(ctx context.Context, id string) (*model.Vote, error)
	GetByVoterAndElection(ctx context.Context, voterID, electionID string) (*model.Vote, error)
	ListByElection(ctx context.Context, electionID string) ([]*model.Vote, error)
	HasVoted(ctx context.Context, voterID, electionID string) (bool, error)
	Delete(ctx context.Context, id string) error
}

// KVVoteStore KV 投票存储实现
type KVVoteStore struct {
	kv Store
}

// NewVoteStore 创建投票存储
func NewVoteStore(kv Store) *KVVoteStore {
	return &KVVoteStore{kv: kv}
}

func (s *KVVoteStore) Create(ctx context.Context, vote *model.Vote) error {
	data, err := json.Marshal(vote)
	if err != nil {
		return fmt.Errorf("marshal vote: %w", err)
	}

	// 主键
	key := []byte(votePrefix + vote.ID)
	if err := s.kv.Put(key, data); err != nil {
		return err
	}

	// 按投票人索引
	voterIndexKey := []byte(fmt.Sprintf("%s%s:%s", votesByVoterKey, vote.ElectionID, vote.VoterID))
	if err := s.kv.Put(voterIndexKey, []byte(vote.ID)); err != nil {
		return err
	}

	return nil
}

func (s *KVVoteStore) Get(ctx context.Context, id string) (*model.Vote, error) {
	key := []byte(votePrefix + id)
	data, err := s.kv.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get vote: %w", err)
	}
	var vote model.Vote
	if err := json.Unmarshal(data, &vote); err != nil {
		return nil, fmt.Errorf("unmarshal vote: %w", err)
	}
	return &vote, nil
}

func (s *KVVoteStore) GetByVoterAndElection(ctx context.Context, voterID, electionID string) (*model.Vote, error) {
	indexKey := []byte(fmt.Sprintf("%s%s:%s", votesByVoterKey, electionID, voterID))
	voteIDBytes, err := s.kv.Get(indexKey)
	if err != nil {
		return nil, fmt.Errorf("vote not found: %w", err)
	}
	return s.Get(ctx, string(voteIDBytes))
}

func (s *KVVoteStore) ListByElection(ctx context.Context, electionID string) ([]*model.Vote, error) {
	prefix := []byte(votePrefix)
	items, err := s.kv.Scan(prefix)
	if err != nil {
		return nil, fmt.Errorf("scan votes: %w", err)
	}

	var votes []*model.Vote
	for _, data := range items {
		var vote model.Vote
		if err := json.Unmarshal(data, &vote); err != nil {
			continue
		}
		if vote.ElectionID == electionID {
			votes = append(votes, &vote)
		}
	}
	return votes, nil
}

func (s *KVVoteStore) HasVoted(ctx context.Context, voterID, electionID string) (bool, error) {
	indexKey := []byte(fmt.Sprintf("%s%s:%s", votesByVoterKey, electionID, voterID))
	_, err := s.kv.Get(indexKey)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *KVVoteStore) Delete(ctx context.Context, id string) error {
	// First get the vote to find the voter index
	vote, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	// Delete main key
	key := []byte(votePrefix + id)
	if err := s.kv.Delete(key); err != nil {
		return err
	}

	// Delete voter index
	voterIndexKey := []byte(fmt.Sprintf("%s%s:%s", votesByVoterKey, vote.ElectionID, vote.VoterID))
	return s.kv.Delete(voterIndexKey)
}
