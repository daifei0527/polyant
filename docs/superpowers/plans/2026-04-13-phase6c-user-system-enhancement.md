# Phase 6c: 用户体系完善实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现投票选举系统和用户管理功能，完善用户体系

**Architecture:** 新增选举模块（Election Service）和管理员服务（Admin Service），扩展存储层支持选举和投票数据，添加管理 API 端点

**Tech Stack:** Go 1.22, Pebble KV 存储

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/storage/model/election.go` | 创建 | 选举、候选人、投票数据模型 |
| `internal/storage/kv/election_store.go` | 创建 | 选举存储实现 |
| `internal/storage/kv/vote_store.go` | 创建 | 投票存储实现 |
| `internal/core/election/election.go` | 创建 | 选举服务逻辑 |
| `internal/core/election/election_test.go` | 创建 | 选举服务测试 |
| `internal/core/user/admin_service.go` | 创建 | 管理员服务 |
| `internal/core/user/admin_service_test.go` | 创建 | 管理员服务测试 |
| `internal/api/handler/admin_handler.go` | 创建 | 管理员 API 处理器 |
| `internal/api/handler/election_handler.go` | 创建 | 选举 API 处理器 |
| `internal/api/handler/user_handler.go` | 修改 | 添加用户列表端点 |
| `internal/api/router/router.go` | 修改 | 添加新路由 |

---

## Task 1: 添加选举数据模型

**Files:**
- Create: `internal/storage/model/election.go`

- [ ] **Step 1: 创建选举模型文件**

创建 `internal/storage/model/election.go`:

```go
package model

import "time"

// ElectionStatus 选举状态
type ElectionStatus string

const (
	ElectionStatusActive ElectionStatus = "active"
	ElectionStatusClosed ElectionStatus = "closed"
)

// CandidateStatus 候选人状态
type CandidateStatus string

const (
	CandidateStatusNominated CandidateStatus = "nominated"
	CandidateStatusElected   CandidateStatus = "elected"
	CandidateStatusRejected  CandidateStatus = "rejected"
)

// Election 选举
type Election struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Status        ElectionStatus `json:"status"`
	StartTime     int64          `json:"start_time"`
	EndTime       int64          `json:"end_time"`
	VoteThreshold int32          `json:"vote_threshold"` // 当选所需票数
	CreatedAt     int64          `json:"created_at"`
	CreatedBy     string         `json:"created_by"` // 创建者用户 ID
}

// Candidate 候选人
type Candidate struct {
	ElectionID  string          `json:"election_id"`
	UserID      string          `json:"user_id"`
	UserName    string          `json:"user_name"`
	NominatedBy string          `json:"nominated_by"` // 提名人 ID
	VoteCount   int32           `json:"vote_count"`
	Status      CandidateStatus `json:"status"`
	NominatedAt int64           `json:"nominated_at"`
}

// Vote 投票记录
type Vote struct {
	ID          string `json:"id"`
	ElectionID  string `json:"election_id"`
	VoterID     string `json:"voter_id"`
	CandidateID string `json:"candidate_id"` // 候选人用户 ID
	VotedAt     int64  `json:"voted_at"`
}

// UserStats 用户统计
type UserStats struct {
	TotalUsers    int64 `json:"total_users"`
	ActiveUsers   int64 `json:"active_users"` // 30天内活跃
	Lv0Count      int64 `json:"lv0_count"`
	Lv1Count      int64 `json:"lv1_count"`
	Lv2Count      int64 `json:"lv2_count"`
	Lv3Count      int64 `json:"lv3_count"`
	Lv4Count      int64 `json:"lv4_count"`
	Lv5Count      int64 `json:"lv5_count"`
	TotalContribs int64 `json:"total_contribs"`
	TotalRatings  int64 `json:"total_ratings"`
	BannedCount   int64 `json:"banned_count"`
}

// NewElection 创建新选举
func NewElection(title, description, createdBy string, voteThreshold int32, duration time.Duration) *Election {
	now := time.Now().UnixMilli()
	return &Election{
		ID:            generateID(),
		Title:         title,
		Description:   description,
		Status:        ElectionStatusActive,
		StartTime:     now,
		EndTime:       now + duration.Milliseconds(),
		VoteThreshold: voteThreshold,
		CreatedAt:     now,
		CreatedBy:     createdBy,
	}
}
```

- [ ] **Step 2: 提交数据模型**

```bash
git add internal/storage/model/election.go
git commit -m "feat(model): 添加选举和投票数据模型

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: 实现选举存储

**Files:**
- Create: `internal/storage/kv/election_store.go`

- [ ] **Step 1: 创建选举存储接口和实现**

创建 `internal/storage/kv/election_store.go`:

```go
package kv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/daifei0527/agentwiki/internal/storage/model"
)

const (
	electionPrefix    = "election:"
	electionListKey   = "elections:list"
	candidatePrefix   = "candidate:"
	candidatesListKey = "candidates:"
)

// ElectionStore 选举存储接口
type ElectionStore interface {
	Create(ctx context.Context, election *model.Election) error
	Get(ctx context.Context, id string) (*model.Election, error)
	Update(ctx context.Context, election *model.Election) error
	List(ctx context.Context, status model.ElectionStatus) ([]*model.Election, error)
	Delete(ctx context.Context, id string) error
}

// KVEventStore KV 选举存储实现
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

func (s *KVCandidateStore) UpdateVoteCount(ctx context.Context, electionID, userID string, delta int32) error {
	candidate, err := s.Get(ctx, electionID, userID)
	if err != nil {
		return err
	}
	candidate.VoteCount += delta
	return s.Add(ctx, candidate)
}

func (s *KVCandidateStore) UpdateStatus(ctx context.Context, electionID, userID string, status model.CandidateStatus) error {
	candidate, err := s.Get(ctx, electionID, userID)
	if err != nil {
		return err
	}
	candidate.Status = status
	return s.Add(ctx, candidate)
}
```

- [ ] **Step 2: 提交选举存储**

```bash
git add internal/storage/kv/election_store.go
git commit -m "feat(storage): 添加选举和候选人存储实现

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: 实现投票存储

**Files:**
- Create: `internal/storage/kv/vote_store.go`

- [ ] **Step 1: 创建投票存储**

创建 `internal/storage/kv/vote_store.go`:

```go
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
	votesByElection = "votes:election:"
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
	key := []byte(votePrefix + id)
	return s.kv.Delete(key)
}
```

- [ ] **Step 2: 提交投票存储**

```bash
git add internal/storage/kv/vote_store.go
git commit -m "feat(storage): 添加投票存储实现

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: 实现选举服务

**Files:**
- Create: `internal/core/election/election.go`
- Create: `internal/core/election/election_test.go`

- [ ] **Step 1: 创建选举服务测试**

创建 `internal/core/election/election_test.go`:

```go
package election

import (
	"context"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElectionService_CreateElection(t *testing.T) {
	store := kv.NewMemoryKVStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
		nil, // user store not needed for this test
	)

	ctx := context.Background()
	election, err := service.CreateElection(ctx, "Test Election", "Description", "creator1", 5, 0)

	require.NoError(t, err)
	assert.NotEmpty(t, election.ID)
	assert.Equal(t, "Test Election", election.Title)
	assert.Equal(t, model.ElectionStatusActive, election.Status)
}

func TestElectionService_NominateCandidate(t *testing.T) {
	store := kv.NewMemoryKVStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
		nil,
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
	store := kv.NewMemoryKVStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
		nil,
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
	store := kv.NewMemoryKVStore()
	service := NewElectionService(
		kv.NewElectionStore(store),
		kv.NewCandidateStore(store),
		kv.NewVoteStore(store),
		nil,
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
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/core/election/... -v`
Expected: FAIL - package not found

- [ ] **Step 3: 创建选举服务实现**

创建 `internal/core/election/election.go`:

```go
package election

import (
	"context"
	"fmt"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

var (
	ErrElectionNotFound    = fmt.Errorf("选举不存在")
	ErrElectionClosed      = fmt.Errorf("选举已关闭")
	ErrAlreadyNominated    = fmt.Errorf("已被提名")
	ErrAlreadyVoted        = fmt.Errorf("已投票")
	ErrCandidateNotFound   = fmt.Errorf("候选人不存在")
	ErrNotEligibleToVote   = fmt.Errorf("无投票资格")
	ErrNotEligibleToNominate = fmt.Errorf("无提名资格")
)

// ElectionService 选举服务
type ElectionService struct {
	electionStore  kv.ElectionStore
	candidateStore kv.CandidateStore
	voteStore      kv.VoteStore
	userStore      kv.UserStore
}

// NewElectionService 创建选举服务
func NewElectionService(
	electionStore kv.ElectionStore,
	candidateStore kv.CandidateStore,
	voteStore kv.VoteStore,
	userStore kv.UserStore,
) *ElectionService {
	return &ElectionService{
		electionStore:  electionStore,
		candidateStore: candidateStore,
		voteStore:      voteStore,
		userStore:      userStore,
	}
}

// CreateElection 创建选举
func (s *ElectionService) CreateElection(ctx context.Context, title, description, createdBy string, voteThreshold int32, durationDays int) (*model.Election, error) {
	duration := time.Duration(durationDays) * 24 * time.Hour
	if duration == 0 {
		duration = 7 * 24 * time.Hour // 默认7天
	}

	election := model.NewElection(title, description, createdBy, voteThreshold, duration)
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
func (s *ElectionService) Vote(ctx context.Context, electionID, voterID, candidateID string) error {
	election, err := s.electionStore.Get(ctx, electionID)
	if err != nil {
		return ErrElectionNotFound
	}

	if election.Status != model.ElectionStatusActive {
		return ErrElectionClosed
	}

	// 检查是否已投票
	hasVoted, err := s.voteStore.HasVoted(ctx, voterID, electionID)
	if err != nil {
		return err
	}
	if hasVoted {
		return ErrAlreadyVoted
	}

	// 检查候选人是否存在
	candidate, err := s.candidateStore.Get(ctx, electionID, candidateID)
	if err != nil {
		return ErrCandidateNotFound
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
		return fmt.Errorf("create vote: %w", err)
	}

	// 更新候选人票数
	if err := s.candidateStore.UpdateVoteCount(ctx, electionID, candidateID, 1); err != nil {
		return fmt.Errorf("update vote count: %w", err)
	}

	// 检查是否达到当选阈值
	if candidate.VoteCount+1 >= election.VoteThreshold {
		s.candidateStore.UpdateStatus(ctx, electionID, candidateID, model.CandidateStatusElected)
	}

	return nil
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
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/core/election/... -v`
Expected: PASS

- [ ] **Step 5: 提交选举服务**

```bash
git add internal/core/election/
git commit -m "feat(election): 实现选举服务

- 创建选举、提名候选人、投票、关闭选举
- 完整单元测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: 实现管理员服务

**Files:**
- Create: `internal/core/user/admin_service.go`
- Create: `internal/core/user/admin_service_test.go`

- [ ] **Step 1: 创建管理员服务测试**

创建 `internal/core/user/admin_service_test.go`:

```go
package user

import (
	"context"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminService_ListUsers(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	service := NewAdminService(store)

	ctx := context.Background()

	// 创建测试用户
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv2})

	users, total, err := service.ListUsers(ctx, 0, 10, 0, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, users, 2)
}

func TestAdminService_BanUser(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})

	err := service.BanUser(ctx, user.PublicKey, "admin1", "违规操作")
	require.NoError(t, err)

	updated, _ := store.User.Get(ctx, storage.HashPublicKey(user.PublicKey))
	assert.Equal(t, "banned", updated.Status)
}

func TestAdminService_UnbanUser(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1, Status: "banned"})

	err := service.UnbanUser(ctx, user.PublicKey, "admin1")
	require.NoError(t, err)

	updated, _ := store.User.Get(ctx, storage.HashPublicKey(user.PublicKey))
	assert.Equal(t, "active", updated.Status)
}

func TestAdminService_SetUserLevel(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	service := NewAdminService(store)

	ctx := context.Background()
	user, _ := store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv1})

	err := service.SetUserLevel(ctx, user.PublicKey, model.UserLevelLv3, "admin1", "贡献突出")
	require.NoError(t, err)

	updated, _ := store.User.Get(ctx, storage.HashPublicKey(user.PublicKey))
	assert.Equal(t, model.UserLevelLv3, updated.UserLevel)
}

func TestAdminService_GetUserStats(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	service := NewAdminService(store)

	ctx := context.Background()
	store.User.Create(ctx, &model.User{PublicKey: "pk1", AgentName: "User1", UserLevel: model.UserLevelLv0})
	store.User.Create(ctx, &model.User{PublicKey: "pk2", AgentName: "User2", UserLevel: model.UserLevelLv1})
	store.User.Create(ctx, &model.User{PublicKey: "pk3", AgentName: "User3", UserLevel: model.UserLevelLv2})

	stats, err := service.GetUserStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalUsers)
	assert.Equal(t, int64(1), stats.Lv0Count)
	assert.Equal(t, int64(1), stats.Lv1Count)
	assert.Equal(t, int64(1), stats.Lv2Count)
}
```

- [ ] **Step 2: 创建管理员服务实现**

创建 `internal/core/user/admin_service.go`:

```go
package user

import (
	"context"
	"fmt"
	"time"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

var (
	ErrUserNotFound     = fmt.Errorf("用户不存在")
	ErrCannotBanAdmin   = fmt.Errorf("无法封禁管理员")
	ErrCannotBanSelf    = fmt.Errorf("无法封禁自己")
	ErrInvalidLevel     = fmt.Errorf("无效的用户等级")
)

// AdminService 管理员服务
type AdminService struct {
	store *storage.Store
}

// NewAdminService 创建管理员服务
func NewAdminService(store *storage.Store) *AdminService {
	return &AdminService{store: store}
}

// ListUsers 列出用户
func (s *AdminService) ListUsers(ctx context.Context, offset, limit int, level int32, search string) ([]*model.User, int64, error) {
	filter := storage.UserFilter{
		Offset: offset,
		Limit:  limit,
		Level:  level,
		Search: search,
	}

	users, total, err := s.store.User.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// BanUser 封禁用户
func (s *AdminService) BanUser(ctx context.Context, targetPublicKey, adminPublicKey, reason string) error {
	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	// 不能封禁 Lv4+ 管理员
	if user.UserLevel >= model.UserLevelLv4 {
		return ErrCannotBanAdmin
	}

	user.Status = "banned"
	user.BanReason = reason
	user.BannedAt = time.Now().UnixMilli()
	user.BannedBy = adminPublicKey

	_, err = s.store.User.Update(ctx, user)
	return err
}

// UnbanUser 解封用户
func (s *AdminService) UnbanUser(ctx context.Context, targetPublicKey, adminPublicKey string) error {
	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	user.Status = "active"
	user.BanReason = ""
	user.BannedAt = 0
	user.BannedBy = ""
	user.UnbannedAt = time.Now().UnixMilli()
	user.UnbannedBy = adminPublicKey

	_, err = s.store.User.Update(ctx, user)
	return err
}

// SetUserLevel 设置用户等级
func (s *AdminService) SetUserLevel(ctx context.Context, targetPublicKey string, newLevel int32, adminPublicKey, reason string) error {
	if newLevel < model.UserLevelLv0 || newLevel > model.UserLevelLv5 {
		return ErrInvalidLevel
	}

	hash := HashPublicKey(targetPublicKey)
	user, err := s.store.User.Get(ctx, hash)
	if err != nil {
		return ErrUserNotFound
	}

	oldLevel := user.UserLevel
	user.UserLevel = newLevel
	user.LevelChangeReason = reason
	user.LevelChangedAt = time.Now().UnixMilli()
	user.LevelChangedBy = adminPublicKey

	_, err = s.store.User.Update(ctx, user)
	if err != nil {
		return err
	}

	// 记录等级变更日志
	fmt.Printf("[AdminService] User %s level changed from Lv%d to Lv%d by %s, reason: %s\n",
		user.AgentName, oldLevel, newLevel, adminPublicKey, reason)

	return nil
}

// GetUserStats 获取用户统计
func (s *AdminService) GetUserStats(ctx context.Context) (*model.UserStats, error) {
	users, total, err := s.store.User.List(ctx, storage.UserFilter{Limit: 100000})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	stats := &model.UserStats{
		TotalUsers: total,
	}

	now := time.Now().UnixMilli()
	thirtyDaysAgo := now - 30*24*60*60*1000

	for _, u := range users {
		// 统计各级别用户数
		switch u.UserLevel {
		case model.UserLevelLv0:
			stats.Lv0Count++
		case model.UserLevelLv1:
			stats.Lv1Count++
		case model.UserLevelLv2:
			stats.Lv2Count++
		case model.UserLevelLv3:
			stats.Lv3Count++
		case model.UserLevelLv4:
			stats.Lv4Count++
		case model.UserLevelLv5:
			stats.Lv5Count++
		}

		// 统计活跃用户
		if u.LastActive > thirtyDaysAgo {
			stats.ActiveUsers++
		}

		// 统计被封禁用户
		if u.Status == "banned" {
			stats.BannedCount++
		}

		// 统计总贡献和评分
		stats.TotalContribs += u.ContributionCnt
		stats.TotalRatings += u.RatingCnt
	}

	return stats, nil
}
```

- [ ] **Step 3: 更新 User 模型添加管理字段**

修改 `internal/storage/model/model.go`，在 User 结构体中添加:

```go
// 在 User 结构体中添加以下字段
BanReason        string `json:"ban_reason,omitempty"`
BannedAt         int64  `json:"banned_at,omitempty"`
BannedBy         string `json:"banned_by,omitempty"`
UnbannedAt       int64  `json:"unbanned_at,omitempty"`
UnbannedBy       string `json:"unbanned_by,omitempty"`
LevelChangeReason string `json:"level_change_reason,omitempty"`
LevelChangedAt   int64  `json:"level_changed_at,omitempty"`
LevelChangedBy   string `json:"level_changed_by,omitempty"`
```

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/core/user/... -v`
Expected: PASS

- [ ] **Step 5: 提交管理员服务**

```bash
git add internal/core/user/ internal/storage/model/model.go
git commit -m "feat(admin): 实现管理员服务

- 用户列表、封禁/解封、等级调整、用户统计
- 完整单元测试

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: 添加用户列表 API

**Files:**
- Modify: `internal/api/handler/user_handler.go`

- [ ] **Step 1: 添加用户列表处理函数**

在 `internal/api/handler/user_handler.go` 中添加:

```go
// ListUsersHandler 用户列表处理器
// GET /api/v1/admin/users?page=1&limit=20&level=1&search=keyword
func (h *UserHandler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	level, _ := strconv.Atoi(r.URL.Query().Get("level"))
	search := r.URL.Query().Get("search")

	// 获取用户列表
	ctx := r.Context()
	adminSvc := user.NewAdminService(h.store)
	users, total, err := adminSvc.ListUsers(ctx, (page-1)*limit, limit, int32(level), search)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/api/handler/user_handler.go
git commit -m "feat(api): 添加用户列表 API

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: 添加管理员 API

**Files:**
- Create: `internal/api/handler/admin_handler.go`

- [ ] **Step 1: 创建管理员 API 处理器**

创建 `internal/api/handler/admin_handler.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/daifei0527/agentwiki/internal/core/user"
	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// AdminHandler 管理员 API 处理器
type AdminHandler struct {
	adminSvc *user.AdminService
}

// NewAdminHandler 创建管理员处理器
func NewAdminHandler(store *storage.Store) *AdminHandler {
	return &AdminHandler{
		adminSvc: user.NewAdminService(store),
	}
}

// BanUserRequest 封禁用户请求
type BanUserRequest struct {
	Reason string `json:"reason"`
}

// SetLevelRequest 设置等级请求
type SetLevelRequest struct {
	Level  int32  `json:"level"`
	Reason string `json:"reason"`
}

// BanUserHandler 封禁用户
// POST /api/v1/admin/users/{public_key}/ban
func (h *AdminHandler) BanUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 从 URL 获取目标用户公钥
	publicKey := extractPathParam(r.URL.Path, "/api/v1/admin/users/", "/ban")
	if publicKey == "" {
		WriteError(w, http.StatusBadRequest, "missing public_key")
		return
	}

	// 解析请求体
	var req BanUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 从上下文获取管理员公钥
	adminPublicKey, _ := r.Context().Value("public_key").(string)

	// 执行封禁
	ctx := r.Context()
	if err := h.adminSvc.BanUser(ctx, publicKey, adminPublicKey, req.Reason); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// UnbanUserHandler 解封用户
// POST /api/v1/admin/users/{public_key}/unban
func (h *AdminHandler) UnbanUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	publicKey := extractPathParam(r.URL.Path, "/api/v1/admin/users/", "/unban")
	if publicKey == "" {
		WriteError(w, http.StatusBadRequest, "missing public_key")
		return
	}

	adminPublicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.adminSvc.UnbanUser(ctx, publicKey, adminPublicKey); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// SetUserLevelHandler 设置用户等级
// PUT /api/v1/admin/users/{public_key}/level
func (h *AdminHandler) SetUserLevelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	publicKey := extractPathParam(r.URL.Path, "/api/v1/admin/users/", "/level")
	if publicKey == "" {
		WriteError(w, http.StatusBadRequest, "missing public_key")
		return
	}

	var req SetLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	adminPublicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.adminSvc.SetUserLevel(ctx, publicKey, req.Level, adminPublicKey, req.Reason); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"new_level": req.Level,
	})
}

// GetUserStatsHandler 获取用户统计
// GET /api/v1/admin/stats/users
func (h *AdminHandler) GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	stats, err := h.adminSvc.GetUserStats(ctx)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}

// extractPathParam 从 URL 中提取路径参数
func extractPathParam(path, prefix, suffix string) string {
	if len(path) <= len(prefix)+len(suffix) {
		return ""
	}
	return path[len(prefix) : len(path)-len(suffix)]
}
```

- [ ] **Step 2: 提交管理员 API**

```bash
git add internal/api/handler/admin_handler.go
git commit -m "feat(api): 添加管理员 API 处理器

- 封禁/解封用户
- 设置用户等级
- 用户统计

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: 添加选举 API

**Files:**
- Create: `internal/api/handler/election_handler.go`

- [ ] **Step 1: 创建选举 API 处理器**

创建 `internal/api/handler/election_handler.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/daifei0527/agentwiki/internal/core/election"
	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ElectionHandler 选举 API 处理器
type ElectionHandler struct {
	electionSvc *election.ElectionService
}

// NewElectionHandler 创建选举处理器
func NewElectionHandler(kv kv.Store) *ElectionHandler {
	return &ElectionHandler{
		electionSvc: election.NewElectionService(
			kv.NewElectionStore(kv),
			kv.NewCandidateStore(kv),
			kv.NewVoteStore(kv),
			nil,
		),
	}
}

// CreateElectionRequest 创建选举请求
type CreateElectionRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	VoteThreshold int32  `json:"vote_threshold"`
	DurationDays  int    `json:"duration_days"`
}

// NominateRequest 提名请求
type NominateRequest struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
}

// VoteRequest 投票请求
type VoteRequest struct {
	CandidateID string `json:"candidate_id"`
}

// CreateElectionHandler 创建选举
// POST /api/v1/elections
func (h *ElectionHandler) CreateElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CreateElectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	publicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	election, err := h.electionSvc.CreateElection(ctx, req.Title, req.Description, publicKey, req.VoteThreshold, req.DurationDays)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"election_id": election.ID})
}

// ListElectionsHandler 列出选举
// GET /api/v1/elections?status=active
func (h *ElectionHandler) ListElectionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := model.ElectionStatus(r.URL.Query().Get("status"))

	ctx := r.Context()
	elections, err := h.electionSvc.ListElections(ctx, status)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"elections": elections})
}

// GetElectionHandler 获取选举详情
// GET /api/v1/elections/{id}
func (h *ElectionHandler) GetElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	electionID := extractLastPathParam(r.URL.Path)
	if electionID == "" {
		WriteError(w, http.StatusBadRequest, "missing election_id")
		return
	}

	ctx := r.Context()
	election, err := h.electionSvc.GetElection(ctx, electionID)
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	candidates, _ := h.electionSvc.ListCandidates(ctx, electionID)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"election":   election,
		"candidates": candidates,
	})
}

// NominateCandidateHandler 提名候选人
// POST /api/v1/elections/{id}/candidates
func (h *ElectionHandler) NominateCandidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/candidates")
	if electionID == "" {
		WriteError(w, http.StatusBadRequest, "missing election_id")
		return
	}

	var req NominateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	publicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.electionSvc.NominateCandidate(ctx, electionID, req.UserID, req.UserName, publicKey); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// VoteHandler 投票
// POST /api/v1/elections/{id}/vote
func (h *ElectionHandler) VoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/vote")
	if electionID == "" {
		WriteError(w, http.StatusBadRequest, "missing election_id")
		return
	}

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	publicKey, _ := r.Context().Value("public_key").(string)

	ctx := r.Context()
	if err := h.electionSvc.Vote(ctx, electionID, publicKey, req.CandidateID); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// CloseElectionHandler 关闭选举
// POST /api/v1/elections/{id}/close
func (h *ElectionHandler) CloseElectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	electionID := extractPathParam(r.URL.Path, "/api/v1/elections/", "/close")
	if electionID == "" {
		WriteError(w, http.StatusBadRequest, "missing election_id")
		return
	}

	ctx := r.Context()
	elected, err := h.electionSvc.CloseElection(ctx, electionID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{"elected": elected})
}

func extractLastPathParam(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
```

- [ ] **Step 2: 提交选举 API**

```bash
git add internal/api/handler/election_handler.go
git commit -m "feat(api): 添加选举 API 处理器

- 创建选举、列出选举、获取详情
- 提名候选人、投票、关闭选举

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: 更新路由和权限

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `internal/api/middleware/auth.go`

- [ ] **Step 1: 添加新路由到 router.go**

在 `internal/api/router/router.go` 中添加路由:

```go
// 在 SetupRouter 或 NewRouter 函数中添加:

// 用户列表 (Lv4+)
mux.Handle("/api/v1/admin/users", authMW.RequireLevel(4, http.HandlerFunc(uh.ListUsersHandler)))

// 管理员 API (Lv4+)
adminH := handler.NewAdminHandler(deps.EntryStore.(*kv.KVEntryStore).GetStore())
mux.Handle("/api/v1/admin/users/{public_key}/ban", authMW.RequireLevel(4, http.HandlerFunc(adminH.BanUserHandler)))
mux.Handle("/api/v1/admin/users/{public_key}/unban", authMW.RequireLevel(4, http.HandlerFunc(adminH.UnbanUserHandler)))
mux.Handle("/api/v1/admin/stats/users", authMW.RequireLevel(4, http.HandlerFunc(adminH.GetUserStatsHandler)))

// 超级管理员 API (Lv5 only)
mux.Handle("/api/v1/admin/users/{public_key}/level", authMW.RequireLevel(5, http.HandlerFunc(adminH.SetUserLevelHandler)))

// 选举 API
electionH := handler.NewElectionHandler(deps.EntryStore.(*kv.KVEntryStore).GetStore())
mux.HandleFunc("/api/v1/elections", electionH.ListElectionsHandler) // 公开
mux.HandleFunc("/api/v1/elections/{id}", electionH.GetElectionHandler) // 公开
mux.Handle("/api/v1/elections", authMW.RequireLevel(5, http.HandlerFunc(electionH.CreateElectionHandler))) // Lv5
mux.Handle("/api/v1/elections/{id}/candidates", authMW.Middleware(http.HandlerFunc(electionH.NominateCandidateHandler))) // Lv4
mux.Handle("/api/v1/elections/{id}/vote", authMW.RequireLevel(3, http.HandlerFunc(electionH.VoteHandler))) // Lv3+
mux.Handle("/api/v1/elections/{id}/close", authMW.RequireLevel(5, http.HandlerFunc(electionH.CloseElectionHandler))) // Lv5
```

- [ ] **Step 2: 添加等级检查中间件**

在 `internal/api/middleware/auth.go` 中添加:

```go
// RequireLevel 等级检查中间件
func (m *AuthMiddleware) RequireLevel(minLevel int32, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 先执行认证
		userLevel, ok := r.Context().Value("user_level").(int32)
		if !ok || userLevel < minLevel {
			WriteError(w, http.StatusForbidden, fmt.Sprintf("需要 Lv%d 或更高等级", minLevel))
			return
		}
		next.ServeHTTP(w, r)
	}
}
```

- [ ] **Step 3: 提交路由更新**

```bash
git add internal/api/router/router.go internal/api/middleware/auth.go
git commit -m "feat(router): 添加用户管理和选举路由

- 用户列表 API (Lv4+)
- 封禁/解封 API (Lv4+)
- 等级调整 API (Lv5)
- 选举 API (各级别权限)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 10: 运行完整测试套件

**Files:**
- 无新增文件

- [ ] **Step 1: 运行所有测试**

Run: `go test ./... -v -count=1`
Expected: 所有测试通过

- [ ] **Step 2: 运行测试覆盖率**

Run: `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1`
Expected: 覆盖率 > 55%

- [ ] **Step 3: 最终提交**

```bash
git add .
git commit -m "feat: Phase 6c 用户体系完善完成

功能:
- 投票选举系统 (Lv4→Lv5)
- 用户列表 API
- 封禁/解封用户
- 手动升级/降级
- 用户统计 API

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## 验收清单

- [ ] 选举数据模型定义
- [ ] 选举存储实现
- [ ] 投票存储实现
- [ ] 选举服务实现并通过测试
- [ ] 管理员服务实现并通过测试
- [ ] 用户列表 API 可用
- [ ] 封禁/解封 API 可用
- [ ] 等级调整 API 可用
- [ ] 选举 API 可用
- [ ] 路由和权限正确配置
- [ ] 所有测试通过
- [ ] 测试覆盖率 > 55%

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 选举作弊 | 破坏公平性 | 记录投票日志，限制投票频率 |
| 管理员滥用权限 | 用户投诉 | 记录所有管理操作，支持审计 |
| 数据一致性 | 统计不准 | 使用事务更新相关数据 |
