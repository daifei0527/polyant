package kv_test

import (
	"context"
	"testing"
	"time"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// ==================== AuditStore 测试 ====================

func TestAuditStore_Create(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	log := &model.AuditLog{
		ID:             "audit-1",
		Timestamp:      time.Now().UnixMilli(),
		OperatorPubkey: "operator-1",
		ActionType:     "entry.create",
		TargetID:       "entry-1",
		Success:        true,
	}

	if err := auditStore.Create(context.Background(), log); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestAuditStore_Get(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	log := &model.AuditLog{
		ID:             "audit-get-test",
		Timestamp:      time.Now().UnixMilli(),
		OperatorPubkey: "operator-1",
		ActionType:     "entry.create",
		Success:        true,
	}
	auditStore.Create(context.Background(), log)

	got, err := auditStore.Get(context.Background(), "audit-get-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.OperatorPubkey != "operator-1" {
		t.Errorf("Expected OperatorPubkey 'operator-1', got '%s'", got.OperatorPubkey)
	}
}

func TestAuditStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	_, err := auditStore.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent audit log")
	}
}

func TestAuditStore_List(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	// Create multiple audit logs
	for i := 0; i < 5; i++ {
		log := &model.AuditLog{
			ID:             string(rune('a' + i)),
			Timestamp:      time.Now().Add(-time.Duration(i) * time.Hour).UnixMilli(),
			OperatorPubkey: "operator-" + string(rune('a'+i%2)),
			ActionType:     "entry.create",
			Success:        true,
		}
		auditStore.Create(context.Background(), log)
	}

	logs, total, err := auditStore.List(context.Background(), model.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 5 {
		t.Errorf("Expected total 5, got %d", total)
	}

	if len(logs) != 5 {
		t.Errorf("Expected 5 logs, got %d", len(logs))
	}
}

func TestAuditStore_List_WithFilter(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	// Create audit logs with different operators
	auditStore.Create(context.Background(), &model.AuditLog{
		ID:             "1",
		Timestamp:      time.Now().UnixMilli(),
		OperatorPubkey: "operator-a",
		ActionType:     "entry.create",
		Success:        true,
	})
	auditStore.Create(context.Background(), &model.AuditLog{
		ID:             "2",
		Timestamp:      time.Now().UnixMilli(),
		OperatorPubkey: "operator-b",
		ActionType:     "entry.delete",
		Success:        false,
	})

	// Filter by operator
	logs, _, err := auditStore.List(context.Background(), model.AuditFilter{
		OperatorPubkey: "operator-a",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}

	// Filter by action type
	logs, _, err = auditStore.List(context.Background(), model.AuditFilter{
		ActionTypes: []string{"entry.delete"},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("List with action filter failed: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log with entry.delete, got %d", len(logs))
	}

	// Filter by success
	success := true
	logs, _, err = auditStore.List(context.Background(), model.AuditFilter{
		Success: &success,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("List with success filter failed: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 successful log, got %d", len(logs))
	}
}

func TestAuditStore_DeleteBefore(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	// Create logs with different timestamps
	now := time.Now()
	auditStore.Create(context.Background(), &model.AuditLog{
		ID:        "old",
		Timestamp: now.Add(-48 * time.Hour).UnixMilli(),
	})
	auditStore.Create(context.Background(), &model.AuditLog{
		ID:        "new",
		Timestamp: now.Add(-1 * time.Hour).UnixMilli(),
	})

	// Delete logs older than 24 hours
	deleted, err := auditStore.DeleteBefore(context.Background(), now.Add(-24*time.Hour).UnixMilli())
	if err != nil {
		t.Fatalf("DeleteBefore failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}
}

func TestAuditStore_GetStats(t *testing.T) {
	store := NewMemoryStore()
	auditStore := kv.NewAuditStore(store)

	// Create some logs
	auditStore.Create(context.Background(), &model.AuditLog{
		ID:         "1",
		Timestamp:  time.Now().UnixMilli(),
		ActionType: "entry.create",
		Success:    true,
	})
	auditStore.Create(context.Background(), &model.AuditLog{
		ID:         "2",
		Timestamp:  time.Now().UnixMilli(),
		ActionType: "entry.delete",
		Success:    false,
	})

	stats, err := auditStore.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalLogs != 2 {
		t.Errorf("Expected TotalLogs 2, got %d", stats.TotalLogs)
	}

	if stats.FailedCount != 1 {
		t.Errorf("Expected FailedCount 1, got %d", stats.FailedCount)
	}
}

// ==================== CategoryStore 测试 ====================

func TestCategoryStore_CreateCategory(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	cat := &model.Category{
		Path:  "test",
		Name:  "测试分类",
		Level: 0,
	}

	if err := categoryStore.CreateCategory(cat); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	// Verify created
	got, err := categoryStore.GetCategory("test")
	if err != nil {
		t.Fatalf("GetCategory failed: %v", err)
	}

	if got.Name != "测试分类" {
		t.Errorf("Expected Name '测试分类', got '%s'", got.Name)
	}
}

func TestCategoryStore_CreateCategory_Duplicate(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	cat := &model.Category{Path: "dup", Name: "重复"}
	categoryStore.CreateCategory(cat)

	err := categoryStore.CreateCategory(&model.Category{Path: "dup", Name: "另一个"})
	if err == nil {
		t.Error("Expected error for duplicate category")
	}
}

func TestCategoryStore_CreateCategory_EmptyPath(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	err := categoryStore.CreateCategory(&model.Category{Name: "无路径"})
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestCategoryStore_GetCategory_NotFound(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	_, err := categoryStore.GetCategory("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent category")
	}
}

func TestCategoryStore_ListCategories(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	// Create categories
	categoryStore.CreateCategory(&model.Category{Path: "cat1", Name: "分类1"})
	categoryStore.CreateCategory(&model.Category{Path: "cat2", Name: "分类2"})

	categories, err := categoryStore.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories failed: %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}
}

func TestCategoryStore_GetChildren(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	// Create parent and children
	categoryStore.CreateCategory(&model.Category{Path: "parent", Name: "父分类", Level: 0})
	categoryStore.CreateCategory(&model.Category{Path: "parent/child1", Name: "子分类1", ParentId: "parent", Level: 1})
	categoryStore.CreateCategory(&model.Category{Path: "parent/child2", Name: "子分类2", ParentId: "parent", Level: 1})
	categoryStore.CreateCategory(&model.Category{Path: "other", Name: "其他", Level: 0})

	children, err := categoryStore.GetChildren("parent")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}

	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

func TestCategoryStore_InitBuiltinCategories(t *testing.T) {
	store := NewMemoryStore()
	categoryStore := kv.NewCategoryStore(store)

	if err := categoryStore.InitBuiltinCategories(); err != nil {
		t.Fatalf("InitBuiltinCategories failed: %v", err)
	}

	categories, _ := categoryStore.ListCategories()
	if len(categories) == 0 {
		t.Error("Expected builtin categories to be created")
	}

	// Verify some expected categories exist
	if _, err := categoryStore.GetCategory("tech"); err != nil {
		t.Error("Expected 'tech' category to exist")
	}
	if _, err := categoryStore.GetCategory("tech/programming"); err != nil {
		t.Error("Expected 'tech/programming' category to exist")
	}
}

// ==================== ElectionStore 测试 ====================

func TestElectionStore_Create(t *testing.T) {
	store := NewMemoryStore()
	electionStore := kv.NewElectionStore(store)

	election := &model.Election{
		ID:            "election-1",
		Title:         "测试选举",
		Description:   "选举描述",
		Status:        model.ElectionStatusActive,
		VoteThreshold: 5,
		CreatedAt:     time.Now().UnixMilli(),
	}

	if err := electionStore.Create(context.Background(), election); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestElectionStore_Get(t *testing.T) {
	store := NewMemoryStore()
	electionStore := kv.NewElectionStore(store)

	election := &model.Election{
		ID:        "election-get",
		Title:     "测试选举",
		Status:    model.ElectionStatusActive,
		CreatedAt: time.Now().UnixMilli(),
	}
	electionStore.Create(context.Background(), election)

	got, err := electionStore.Get(context.Background(), "election-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Title != "测试选举" {
		t.Errorf("Expected Title '测试选举', got '%s'", got.Title)
	}
}

func TestElectionStore_Update(t *testing.T) {
	store := NewMemoryStore()
	electionStore := kv.NewElectionStore(store)

	election := &model.Election{
		ID:        "election-update",
		Title:     "原始标题",
		Status:    model.ElectionStatusActive,
		CreatedAt: time.Now().UnixMilli(),
	}
	electionStore.Create(context.Background(), election)

	// Update
	election.Title = "更新后标题"
	if err := electionStore.Update(context.Background(), election); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := electionStore.Get(context.Background(), "election-update")
	if got.Title != "更新后标题" {
		t.Errorf("Expected updated title, got '%s'", got.Title)
	}
}

func TestElectionStore_List(t *testing.T) {
	store := NewMemoryStore()
	electionStore := kv.NewElectionStore(store)

	// Create elections with different statuses
	electionStore.Create(context.Background(), &model.Election{
		ID:     "election-active",
		Title:  "活跃选举",
		Status: model.ElectionStatusActive,
	})
	electionStore.Create(context.Background(), &model.Election{
		ID:     "election-closed",
		Title:  "已关闭选举",
		Status: model.ElectionStatusClosed,
	})

	// List all
	all, err := electionStore.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("Expected 2 elections, got %d", len(all))
	}

	// List active only
	active, err := electionStore.List(context.Background(), model.ElectionStatusActive)
	if err != nil {
		t.Fatalf("List with status failed: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("Expected 1 active election, got %d", len(active))
	}
}

func TestElectionStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	electionStore := kv.NewElectionStore(store)

	election := &model.Election{
		ID:     "election-delete",
		Title:  "待删除",
		Status: model.ElectionStatusActive,
	}
	electionStore.Create(context.Background(), election)

	if err := electionStore.Delete(context.Background(), "election-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := electionStore.Get(context.Background(), "election-delete")
	if err == nil {
		t.Error("Expected error after delete")
	}
}

// ==================== CandidateStore 测试 ====================

func TestCandidateStore_Add(t *testing.T) {
	store := NewMemoryStore()
	candidateStore := kv.NewCandidateStore(store)

	candidate := &model.Candidate{
		ElectionID: "election-1",
		UserID:     "user-1",
		UserName:   "候选人",
		VoteCount:  0,
		Status:     model.CandidateStatusNominated,
	}

	if err := candidateStore.Add(context.Background(), candidate); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
}

func TestCandidateStore_Get(t *testing.T) {
	store := NewMemoryStore()
	candidateStore := kv.NewCandidateStore(store)

	candidate := &model.Candidate{
		ElectionID: "election-get",
		UserID:     "user-get",
		UserName:   "候选人",
	}
	candidateStore.Add(context.Background(), candidate)

	got, err := candidateStore.Get(context.Background(), "election-get", "user-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.UserName != "候选人" {
		t.Errorf("Expected UserName '候选人', got '%s'", got.UserName)
	}
}

func TestCandidateStore_ListByElection(t *testing.T) {
	store := NewMemoryStore()
	candidateStore := kv.NewCandidateStore(store)

	// Add candidates for same election
	candidateStore.Add(context.Background(), &model.Candidate{
		ElectionID: "election-list",
		UserID:     "user-1",
		UserName:   "候选人1",
	})
	candidateStore.Add(context.Background(), &model.Candidate{
		ElectionID: "election-list",
		UserID:     "user-2",
		UserName:   "候选人2",
	})
	candidateStore.Add(context.Background(), &model.Candidate{
		ElectionID: "other-election",
		UserID:     "user-3",
		UserName:   "其他候选人",
	})

	candidates, err := candidateStore.ListByElection(context.Background(), "election-list")
	if err != nil {
		t.Fatalf("ListByElection failed: %v", err)
	}

	if len(candidates) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(candidates))
	}
}

func TestCandidateStore_UpdateVoteCount(t *testing.T) {
	store := NewMemoryStore()
	candidateStore := kv.NewCandidateStore(store)

	candidate := &model.Candidate{
		ElectionID: "election-vote",
		UserID:     "user-vote",
		VoteCount:  0,
	}
	candidateStore.Add(context.Background(), candidate)

	if err := candidateStore.UpdateVoteCount(context.Background(), "election-vote", "user-vote", 5); err != nil {
		t.Fatalf("UpdateVoteCount failed: %v", err)
	}

	got, _ := candidateStore.Get(context.Background(), "election-vote", "user-vote")
	if got.VoteCount != 5 {
		t.Errorf("Expected VoteCount 5, got %d", got.VoteCount)
	}
}

func TestCandidateStore_UpdateStatus(t *testing.T) {
	store := NewMemoryStore()
	candidateStore := kv.NewCandidateStore(store)

	candidate := &model.Candidate{
		ElectionID: "election-status",
		UserID:     "user-status",
		Status:     model.CandidateStatusNominated,
	}
	candidateStore.Add(context.Background(), candidate)

	if err := candidateStore.UpdateStatus(context.Background(), "election-status", "user-status", model.CandidateStatusElected); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := candidateStore.Get(context.Background(), "election-status", "user-status")
	if got.Status != model.CandidateStatusElected {
		t.Errorf("Expected status elected, got %s", got.Status)
	}
}

// ==================== VoteStore 测试 ====================

func TestVoteStore_Create(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	vote := &model.Vote{
		ID:          "vote-1",
		ElectionID:  "election-1",
		VoterID:     "voter-1",
		CandidateID: "candidate-1",
		VotedAt:     time.Now().UnixMilli(),
	}

	if err := voteStore.Create(context.Background(), vote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestVoteStore_Get(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	vote := &model.Vote{
		ID:          "vote-get",
		ElectionID:  "election-1",
		VoterID:     "voter-1",
		CandidateID: "candidate-1",
	}
	voteStore.Create(context.Background(), vote)

	got, err := voteStore.Get(context.Background(), "vote-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.VoterID != "voter-1" {
		t.Errorf("Expected VoterID 'voter-1', got '%s'", got.VoterID)
	}
}

func TestVoteStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	_, err := voteStore.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent vote")
	}
}

func TestVoteStore_GetByVoterAndElection(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	vote := &model.Vote{
		ID:          "vote-voter",
		ElectionID:  "election-voter",
		VoterID:     "voter-1",
		CandidateID: "candidate-1",
	}
	voteStore.Create(context.Background(), vote)

	got, err := voteStore.GetByVoterAndElection(context.Background(), "voter-1", "election-voter")
	if err != nil {
		t.Fatalf("GetByVoterAndElection failed: %v", err)
	}

	if got.ID != "vote-voter" {
		t.Errorf("Expected ID 'vote-voter', got '%s'", got.ID)
	}
}

func TestVoteStore_GetByVoterAndElection_NotFound(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	_, err := voteStore.GetByVoterAndElection(context.Background(), "nonexistent-voter", "nonexistent-election")
	if err == nil {
		t.Error("Expected error for nonexistent vote")
	}
}

func TestVoteStore_ListByElection(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	// Create votes for different elections
	voteStore.Create(context.Background(), &model.Vote{
		ID:          "vote-1",
		ElectionID:  "election-list",
		VoterID:     "voter-1",
		CandidateID: "candidate-1",
	})
	voteStore.Create(context.Background(), &model.Vote{
		ID:          "vote-2",
		ElectionID:  "election-list",
		VoterID:     "voter-2",
		CandidateID: "candidate-1",
	})
	voteStore.Create(context.Background(), &model.Vote{
		ID:          "vote-3",
		ElectionID:  "other-election",
		VoterID:     "voter-3",
		CandidateID: "candidate-2",
	})

	votes, err := voteStore.ListByElection(context.Background(), "election-list")
	if err != nil {
		t.Fatalf("ListByElection failed: %v", err)
	}

	if len(votes) != 2 {
		t.Errorf("Expected 2 votes, got %d", len(votes))
	}
}

func TestVoteStore_HasVoted(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	vote := &model.Vote{
		ID:          "vote-has",
		ElectionID:  "election-has",
		VoterID:     "voter-has",
		CandidateID: "candidate-1",
	}
	voteStore.Create(context.Background(), vote)

	hasVoted, err := voteStore.HasVoted(context.Background(), "voter-has", "election-has")
	if err != nil {
		t.Fatalf("HasVoted failed: %v", err)
	}

	if !hasVoted {
		t.Error("Expected HasVoted to return true")
	}

	// Check for non-existent vote
	hasVoted, err = voteStore.HasVoted(context.Background(), "nonexistent-voter", "election-has")
	if err != nil {
		t.Fatalf("HasVoted failed: %v", err)
	}

	if hasVoted {
		t.Error("Expected HasVoted to return false for non-existent vote")
	}
}

func TestVoteStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	vote := &model.Vote{
		ID:          "vote-delete",
		ElectionID:  "election-delete",
		VoterID:     "voter-delete",
		CandidateID: "candidate-1",
	}
	voteStore.Create(context.Background(), vote)

	if err := voteStore.Delete(context.Background(), "vote-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := voteStore.Get(context.Background(), "vote-delete")
	if err == nil {
		t.Error("Expected error after delete")
	}

	// Verify index is also deleted
	hasVoted, _ := voteStore.HasVoted(context.Background(), "voter-delete", "election-delete")
	if hasVoted {
		t.Error("Expected HasVoted to return false after delete")
	}
}

func TestVoteStore_Delete_NotFound(t *testing.T) {
	store := NewMemoryStore()
	voteStore := kv.NewVoteStore(store)

	err := voteStore.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for deleting nonexistent vote")
	}
}
