package model

import (
	"testing"
	"time"
)

func TestElectionStatusConstants(t *testing.T) {
	if ElectionStatusActive != "active" {
		t.Errorf("ElectionStatusActive should be 'active', got %s", ElectionStatusActive)
	}
	if ElectionStatusClosed != "closed" {
		t.Errorf("ElectionStatusClosed should be 'closed', got %s", ElectionStatusClosed)
	}
}

func TestCandidateStatusConstants(t *testing.T) {
	if CandidateStatusNominated != "nominated" {
		t.Errorf("CandidateStatusNominated should be 'nominated', got %s", CandidateStatusNominated)
	}
	if CandidateStatusElected != "elected" {
		t.Errorf("CandidateStatusElected should be 'elected', got %s", CandidateStatusElected)
	}
	if CandidateStatusRejected != "rejected" {
		t.Errorf("CandidateStatusRejected should be 'rejected', got %s", CandidateStatusRejected)
	}
}

func TestNewElection(t *testing.T) {
	title := "Test Election"
	description := "Test Description"
	createdBy := "creator-1"
	voteThreshold := int32(5)
	duration := 24 * time.Hour

	election := NewElection(title, description, createdBy, voteThreshold, duration)

	if election.ID == "" {
		t.Error("ID should not be empty")
	}
	if election.Title != title {
		t.Errorf("Expected title %q, got %q", title, election.Title)
	}
	if election.Description != description {
		t.Errorf("Expected description %q, got %q", description, election.Description)
	}
	if election.Status != ElectionStatusActive {
		t.Errorf("Expected status %q, got %q", ElectionStatusActive, election.Status)
	}
	if election.StartTime == 0 {
		t.Error("StartTime should be set")
	}
	if election.EndTime <= election.StartTime {
		t.Error("EndTime should be greater than StartTime")
	}
	if election.VoteThreshold != voteThreshold {
		t.Errorf("Expected voteThreshold %d, got %d", voteThreshold, election.VoteThreshold)
	}
	if election.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if election.CreatedBy != createdBy {
		t.Errorf("Expected createdBy %q, got %q", createdBy, election.CreatedBy)
	}
}

func TestElection_IsClosed(t *testing.T) {
	election := &Election{
		ID:     "test-id",
		Status: ElectionStatusActive,
	}

	if election.IsClosed() {
		t.Error("Election with active status should not be closed")
	}

	election.Status = ElectionStatusClosed
	if !election.IsClosed() {
		t.Error("Election with closed status should be closed")
	}
}

func TestElection_IsExpired(t *testing.T) {
	// Not expired - end time in the future
	election := &Election{
		ID:      "test-id",
		EndTime: time.Now().Add(24 * time.Hour).UnixMilli(),
	}

	if election.IsExpired() {
		t.Error("Election with future end time should not be expired")
	}

	// Expired - end time in the past
	election.EndTime = time.Now().Add(-1 * time.Hour).UnixMilli()
	if !election.IsExpired() {
		t.Error("Election with past end time should be expired")
	}
}

func TestElection_ToJSON_FromJSON(t *testing.T) {
	election := &Election{
		ID:            "election-1",
		Title:         "Test Election",
		Description:   "Test Description",
		Status:        ElectionStatusActive,
		StartTime:     time.Now().UnixMilli(),
		EndTime:       time.Now().Add(24 * time.Hour).UnixMilli(),
		VoteThreshold: 5,
		CreatedAt:     time.Now().UnixMilli(),
		CreatedBy:     "creator-1",
	}

	json, err := election.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(json) == 0 {
		t.Error("JSON should not be empty")
	}

	newElection := &Election{}
	err = newElection.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newElection.ID != election.ID {
		t.Errorf("ID mismatch: expected %s, got %s", election.ID, newElection.ID)
	}
	if newElection.Title != election.Title {
		t.Errorf("Title mismatch: expected %s, got %s", election.Title, newElection.Title)
	}
	if newElection.Description != election.Description {
		t.Errorf("Description mismatch: expected %s, got %s", election.Description, newElection.Description)
	}
	if newElection.Status != election.Status {
		t.Errorf("Status mismatch: expected %s, got %s", election.Status, newElection.Status)
	}
	if newElection.VoteThreshold != election.VoteThreshold {
		t.Errorf("VoteThreshold mismatch: expected %d, got %d", election.VoteThreshold, newElection.VoteThreshold)
	}
	if newElection.CreatedBy != election.CreatedBy {
		t.Errorf("CreatedBy mismatch: expected %s, got %s", election.CreatedBy, newElection.CreatedBy)
	}
}

func TestNewCandidate(t *testing.T) {
	electionID := "election-1"
	userID := "user-1"
	userName := "Test User"
	nominatedBy := "nominator-1"

	candidate := NewCandidate(electionID, userID, userName, nominatedBy)

	if candidate.ElectionID != electionID {
		t.Errorf("Expected electionID %q, got %q", electionID, candidate.ElectionID)
	}
	if candidate.UserID != userID {
		t.Errorf("Expected userID %q, got %q", userID, candidate.UserID)
	}
	if candidate.UserName != userName {
		t.Errorf("Expected userName %q, got %q", userName, candidate.UserName)
	}
	if candidate.NominatedBy != nominatedBy {
		t.Errorf("Expected nominatedBy %q, got %q", nominatedBy, candidate.NominatedBy)
	}
	if candidate.VoteCount != 0 {
		t.Errorf("Initial VoteCount should be 0, got %d", candidate.VoteCount)
	}
	if candidate.Status != CandidateStatusNominated {
		t.Errorf("Initial Status should be %q, got %q", CandidateStatusNominated, candidate.Status)
	}
	if candidate.NominatedAt == 0 {
		t.Error("NominatedAt should be set")
	}
}

func TestCandidate_ToJSON_FromJSON(t *testing.T) {
	candidate := &Candidate{
		ElectionID:  "election-1",
		UserID:      "user-1",
		UserName:    "Test User",
		NominatedBy: "nominator-1",
		VoteCount:   10,
		Status:      CandidateStatusElected,
		NominatedAt: time.Now().UnixMilli(),
	}

	json, err := candidate.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(json) == 0 {
		t.Error("JSON should not be empty")
	}

	newCandidate := &Candidate{}
	err = newCandidate.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newCandidate.ElectionID != candidate.ElectionID {
		t.Errorf("ElectionID mismatch: expected %s, got %s", candidate.ElectionID, newCandidate.ElectionID)
	}
	if newCandidate.UserID != candidate.UserID {
		t.Errorf("UserID mismatch: expected %s, got %s", candidate.UserID, newCandidate.UserID)
	}
	if newCandidate.UserName != candidate.UserName {
		t.Errorf("UserName mismatch: expected %s, got %s", candidate.UserName, newCandidate.UserName)
	}
	if newCandidate.VoteCount != candidate.VoteCount {
		t.Errorf("VoteCount mismatch: expected %d, got %d", candidate.VoteCount, newCandidate.VoteCount)
	}
	if newCandidate.Status != candidate.Status {
		t.Errorf("Status mismatch: expected %s, got %s", candidate.Status, newCandidate.Status)
	}
}

func TestNewVote(t *testing.T) {
	electionID := "election-1"
	voterID := "voter-1"
	candidateID := "candidate-1"

	vote := NewVote(electionID, voterID, candidateID)

	if vote.ID == "" {
		t.Error("ID should not be empty")
	}
	if vote.ElectionID != electionID {
		t.Errorf("Expected electionID %q, got %q", electionID, vote.ElectionID)
	}
	if vote.VoterID != voterID {
		t.Errorf("Expected voterID %q, got %q", voterID, vote.VoterID)
	}
	if vote.CandidateID != candidateID {
		t.Errorf("Expected candidateID %q, got %q", candidateID, vote.CandidateID)
	}
	if vote.VotedAt == 0 {
		t.Error("VotedAt should be set")
	}
}

func TestVote_ToJSON_FromJSON(t *testing.T) {
	vote := &Vote{
		ID:          "vote-1",
		ElectionID:  "election-1",
		VoterID:     "voter-1",
		CandidateID: "candidate-1",
		VotedAt:     time.Now().UnixMilli(),
	}

	json, err := vote.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(json) == 0 {
		t.Error("JSON should not be empty")
	}

	newVote := &Vote{}
	err = newVote.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newVote.ID != vote.ID {
		t.Errorf("ID mismatch: expected %s, got %s", vote.ID, newVote.ID)
	}
	if newVote.ElectionID != vote.ElectionID {
		t.Errorf("ElectionID mismatch: expected %s, got %s", vote.ElectionID, newVote.ElectionID)
	}
	if newVote.VoterID != vote.VoterID {
		t.Errorf("VoterID mismatch: expected %s, got %s", vote.VoterID, newVote.VoterID)
	}
	if newVote.CandidateID != vote.CandidateID {
		t.Errorf("CandidateID mismatch: expected %s, got %s", vote.CandidateID, newVote.CandidateID)
	}
	if newVote.VotedAt != vote.VotedAt {
		t.Errorf("VotedAt mismatch: expected %d, got %d", vote.VotedAt, newVote.VotedAt)
	}
}
