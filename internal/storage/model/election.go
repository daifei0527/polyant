// Package model 定义了Polyant系统的核心数据模型
package model

import (
	"encoding/json"
	"time"
)

// ==================== 选举状态常量 ====================

// ElectionStatus 选举状态
type ElectionStatus string

const (
	ElectionStatusActive ElectionStatus = "active" // 进行中
	ElectionStatusClosed ElectionStatus = "closed" // 已关闭
)

// CandidateStatus 候选人状态
type CandidateStatus string

const (
	CandidateStatusNominated CandidateStatus = "nominated" // 已提名
	CandidateStatusElected   CandidateStatus = "elected"   // 已当选
	CandidateStatusRejected  CandidateStatus = "rejected"  // 已落选
)

// ==================== 选举 ====================

// Election 表示一次选举
type Election struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Status        ElectionStatus `json:"status"`
	StartTime     int64          `json:"startTime"`     // 开始时间(Unix毫秒)
	EndTime       int64          `json:"endTime"`       // 结束时间(Unix毫秒)
	VoteThreshold int32          `json:"voteThreshold"` // 当选所需票数
	AutoElect     bool           `json:"autoElect"`     // 是否自动当选
	CreatedAt     int64          `json:"createdAt"`
	CreatedBy     string         `json:"createdBy"` // 创建者用户ID
}

// NewElection 创建新选举
func NewElection(title, description, createdBy string, voteThreshold int32, duration time.Duration, autoElect bool) *Election {
	now := time.Now().UnixMilli()
	return &Election{
		ID:            generateID(),
		Title:         title,
		Description:   description,
		Status:        ElectionStatusActive,
		StartTime:     now,
		EndTime:       now + duration.Milliseconds(),
		VoteThreshold: voteThreshold,
		AutoElect:     autoElect,
		CreatedAt:     now,
		CreatedBy:     createdBy,
	}
}

// ToJSON 将选举序列化为JSON字节数组
func (e *Election) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON 从JSON字节数组反序列化为选举
func (e *Election) FromJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

// IsClosed 判断选举是否已关闭
func (e *Election) IsClosed() bool {
	return e.Status == ElectionStatusClosed
}

// IsExpired 判断选举是否已过期
func (e *Election) IsExpired() bool {
	return time.Now().UnixMilli() > e.EndTime
}

// ShouldAutoElect 判断是否应该自动当选
func (e *Election) ShouldAutoElect() bool {
	return e.AutoElect && e.Status == ElectionStatusActive
}

// ==================== 候选人 ====================

// Candidate 表示选举候选人
type Candidate struct {
	ElectionID    string          `json:"electionId"`
	UserID        string          `json:"userId"`
	UserName      string          `json:"userName"`
	NominatedBy   string          `json:"nominatedBy"`   // 提名人ID
	SelfNominated bool            `json:"selfNominated"` // 是否自荐
	Confirmed     bool            `json:"confirmed"`     // 是否确认接受提名
	ConfirmedAt   int64           `json:"confirmedAt,omitempty"`
	VoteCount     int32           `json:"voteCount"`
	Status        CandidateStatus `json:"status"`
	NominatedAt   int64           `json:"nominatedAt"`
}

// NewCandidate 创建新候选人
func NewCandidate(electionID, userID, userName, nominatedBy string, selfNominated bool) *Candidate {
	now := time.Now().UnixMilli()
	c := &Candidate{
		ElectionID:    electionID,
		UserID:        userID,
		UserName:      userName,
		NominatedBy:   nominatedBy,
		SelfNominated: selfNominated,
		VoteCount:     0,
		Status:        CandidateStatusNominated,
		NominatedAt:   now,
	}
	// 自荐自动确认
	if selfNominated {
		c.Confirmed = true
		c.ConfirmedAt = now
	}
	return c
}

// ToJSON 将候选人序列化为JSON字节数组
func (c *Candidate) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON 从JSON字节数组反序列化为候选人
func (c *Candidate) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// IsReady 判断候选人是否准备好（已确认接受提名）
func (c *Candidate) IsReady() bool {
	return c.Confirmed
}

// Confirm 确认接受提名
func (c *Candidate) Confirm() {
	c.Confirmed = true
	c.ConfirmedAt = time.Now().UnixMilli()
}

// ==================== 投票记录 ====================

// Vote 表示一次投票记录
type Vote struct {
	ID          string `json:"id"`
	ElectionID  string `json:"electionId"`
	VoterID     string `json:"voterId"`
	CandidateID string `json:"candidateId"` // 候选人用户ID
	VotedAt     int64  `json:"votedAt"`
}

// NewVote 创建新投票
func NewVote(electionID, voterID, candidateID string) *Vote {
	return &Vote{
		ID:          generateID(),
		ElectionID:  electionID,
		VoterID:     voterID,
		CandidateID: candidateID,
		VotedAt:     time.Now().UnixMilli(),
	}
}

// ToJSON 将投票序列化为JSON字节数组
func (v *Vote) ToJSON() ([]byte, error) {
	return json.Marshal(v)
}

// FromJSON 从JSON字节数组反序列化为投票
func (v *Vote) FromJSON(data []byte) error {
	return json.Unmarshal(data, v)
}
