package election

import (
	"context"
	"fmt"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

var (
	ErrElectionNotFound   = fmt.Errorf("选举不存在")
	ErrElectionClosed     = fmt.Errorf("选举已关闭")
	ErrAlreadyNominated   = fmt.Errorf("已被提名")
	ErrAlreadyVoted       = fmt.Errorf("已投票")
	ErrCandidateNotFound  = fmt.Errorf("候选人不存在")
	ErrCandidateNotReady  = fmt.Errorf("候选人尚未确认接受提名")
)

// VoteResult 投票结果
type VoteResult struct {
	Success     bool   `json:"success"`
	VoteCount   int32  `json:"voteCount"`
	AutoElected bool   `json:"autoElected,omitempty"`
	Message     string `json:"message,omitempty"`
}

// ElectionService 选举服务
type ElectionService struct {
	electionStore  kv.ElectionStore
	candidateStore kv.CandidateStore
	voteStore      kv.VoteStore
}

// NewElectionService 创建选举服务
func NewElectionService(
	electionStore kv.ElectionStore,
	candidateStore kv.CandidateStore,
	voteStore kv.VoteStore,
) *ElectionService {
	return &ElectionService{
		electionStore:  electionStore,
		candidateStore: candidateStore,
		voteStore:      voteStore,
	}
}

// CreateElection 创建选举
func (s *ElectionService) CreateElection(ctx context.Context, title, description, createdBy string, voteThreshold int32, durationDays int, autoElect bool) (*model.Election, error) {
	duration := time.Duration(durationDays) * 24 * time.Hour
	if duration == 0 {
		duration = 7 * 24 * time.Hour // 默认7天
	}

	election := model.NewElection(title, description, createdBy, voteThreshold, duration, autoElect)
	if err := s.electionStore.Create(ctx, election); err != nil {
		return nil, fmt.Errorf("create election: %w", err)
	}

	return election, nil
}

// GetElection 获取选举
func (s *ElectionService) GetElection(ctx context.Context, id string) (*model.Election, error) {
	election, err := s.electionStore.Get(ctx, id)
	if err != nil {
		return nil, ErrElectionNotFound
	}
	return election, nil
}

// ListElections 列出选举
func (s *ElectionService) ListElections(ctx context.Context, status model.ElectionStatus) ([]*model.Election, error) {
	return s.electionStore.List(ctx, status)
}

// NominateCandidate 提名候选人
func (s *ElectionService) NominateCandidate(ctx context.Context, electionID, userID, userName, nominatedBy string) error {
	election, err := s.electionStore.Get(ctx, electionID)
	if err != nil {
		return ErrElectionNotFound
	}

	if election.Status != model.ElectionStatusActive {
		return ErrElectionClosed
	}

	// 检查是否已被提名
	existing, _ := s.candidateStore.Get(ctx, electionID, userID)
	if existing != nil {
		return ErrAlreadyNominated
	}

	candidate := &model.Candidate{
		ElectionID:  electionID,
		UserID:      userID,
		UserName:    userName,
		NominatedBy: nominatedBy,
		VoteCount:   0,
		Status:      model.CandidateStatusNominated,
		NominatedAt: time.Now().UnixMilli(),
	}

	return s.candidateStore.Add(ctx, candidate)
}

// Vote 投票
func (s *ElectionService) Vote(ctx context.Context, electionID, voterID, candidateID string) (*VoteResult, error) {
	election, err := s.electionStore.Get(ctx, electionID)
	if err != nil {
		return nil, ErrElectionNotFound
	}

	if election.Status != model.ElectionStatusActive {
		return nil, ErrElectionClosed
	}

	// 检查候选人是否存在且已确认
	candidate, err := s.candidateStore.Get(ctx, electionID, candidateID)
	if err != nil {
		return nil, ErrCandidateNotFound
	}

	if !candidate.IsReady() {
		return nil, ErrCandidateNotReady
	}

	// 检查是否已投票
	hasVoted, err := s.voteStore.HasVoted(ctx, voterID, electionID)
	if err != nil {
		return nil, err
	}
	if hasVoted {
		return nil, ErrAlreadyVoted
	}

	// 创建投票记录
	vote := &model.Vote{
		ID:          generateVoteID(),
		ElectionID:  electionID,
		VoterID:     voterID,
		CandidateID: candidateID,
		VotedAt:     time.Now().UnixMilli(),
	}

	if err := s.voteStore.Create(ctx, vote); err != nil {
		return nil, fmt.Errorf("create vote: %w", err)
	}

	// 更新候选人票数
	newVoteCount := candidate.VoteCount + 1
	if err := s.candidateStore.UpdateVoteCount(ctx, electionID, candidateID, 1); err != nil {
		return nil, fmt.Errorf("update vote count: %w", err)
	}

	result := &VoteResult{
		Success:   true,
		VoteCount: newVoteCount,
	}

	// 检查是否自动当选
	if election.ShouldAutoElect() && newVoteCount >= election.VoteThreshold {
		s.candidateStore.UpdateStatus(ctx, electionID, candidateID, model.CandidateStatusElected)
		result.AutoElected = true
		result.Message = "恭喜！候选人已自动当选"
	}

	return result, nil
}

// HasVoted 检查是否已投票
func (s *ElectionService) HasVoted(ctx context.Context, voterID, electionID string) (bool, error) {
	return s.voteStore.HasVoted(ctx, voterID, electionID)
}

// ListCandidates 列出候选人
func (s *ElectionService) ListCandidates(ctx context.Context, electionID string) ([]*model.Candidate, error) {
	return s.candidateStore.ListByElection(ctx, electionID)
}

// CloseElection 关闭选举
func (s *ElectionService) CloseElection(ctx context.Context, electionID string) ([]*model.Candidate, error) {
	election, err := s.electionStore.Get(ctx, electionID)
	if err != nil {
		return nil, ErrElectionNotFound
	}

	if election.Status == model.ElectionStatusClosed {
		return nil, ErrElectionClosed
	}

	// 获取所有候选人
	candidates, err := s.candidateStore.ListByElection(ctx, electionID)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}

	// 找出达到阈值的候选人
	var elected []*model.Candidate
	for _, c := range candidates {
		if c.VoteCount >= election.VoteThreshold {
			c.Status = model.CandidateStatusElected
			s.candidateStore.UpdateStatus(ctx, electionID, c.UserID, model.CandidateStatusElected)
			elected = append(elected, c)
		} else {
			c.Status = model.CandidateStatusRejected
			s.candidateStore.UpdateStatus(ctx, electionID, c.UserID, model.CandidateStatusRejected)
		}
	}

	// 更新选举状态
	election.Status = model.ElectionStatusClosed
	s.electionStore.Update(ctx, election)

	return elected, nil
}

func generateVoteID() string {
	return fmt.Sprintf("vote_%d", time.Now().UnixNano())
}
