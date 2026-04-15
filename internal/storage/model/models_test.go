package model

import (
	"testing"
)

func TestUserLevelConstants(t *testing.T) {
	if UserLevelLv0 != 0 {
		t.Errorf("UserLevelLv0 should be 0, got %d", UserLevelLv0)
	}
	if UserLevelLv1 != 1 {
		t.Errorf("UserLevelLv1 should be 1, got %d", UserLevelLv1)
	}
	if UserLevelLv2 != 2 {
		t.Errorf("UserLevelLv2 should be 2, got %d", UserLevelLv2)
	}
	if UserLevelLv3 != 3 {
		t.Errorf("UserLevelLv3 should be 3, got %d", UserLevelLv3)
	}
	if UserLevelLv4 != 4 {
		t.Errorf("UserLevelLv4 should be 4, got %d", UserLevelLv4)
	}
	if UserLevelLv5 != 5 {
		t.Errorf("UserLevelLv5 should be 5, got %d", UserLevelLv5)
	}
}

func TestEntryStatusConstants(t *testing.T) {
	if EntryStatusDraft != "draft" {
		t.Errorf("EntryStatusDraft should be 'draft', got %s", EntryStatusDraft)
	}
	if EntryStatusPublished != "published" {
		t.Errorf("EntryStatusPublished should be 'published', got %s", EntryStatusPublished)
	}
	if EntryStatusArchived != "archived" {
		t.Errorf("EntryStatusArchived should be 'archived', got %s", EntryStatusArchived)
	}
	if EntryStatusDeleted != "deleted" {
		t.Errorf("EntryStatusDeleted should be 'deleted', got %s", EntryStatusDeleted)
	}
	if EntryStatusReview != "review" {
		t.Errorf("EntryStatusReview should be 'review', got %s", EntryStatusReview)
	}
}

func TestNodeTypeConstants(t *testing.T) {
	if NodeTypeFull != "full" {
		t.Errorf("NodeTypeFull should be 'full', got %s", NodeTypeFull)
	}
	if NodeTypeLight != "light" {
		t.Errorf("NodeTypeLight should be 'light', got %s", NodeTypeLight)
	}
	if NodeTypeArchive != "archive" {
		t.Errorf("NodeTypeArchive should be 'archive', got %s", NodeTypeArchive)
	}
	if NodeTypeEdge != "edge" {
		t.Errorf("NodeTypeEdge should be 'edge', got %s", NodeTypeEdge)
	}
}

func TestNewKnowledgeEntry(t *testing.T) {
	entry := NewKnowledgeEntry("Test Title", "Test Content", "test-category", "creator-key")

	if entry.ID == "" {
		t.Error("ID should not be empty")
	}
	if entry.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got %s", entry.Title)
	}
	if entry.Content != "Test Content" {
		t.Errorf("Expected content 'Test Content', got %s", entry.Content)
	}
	if entry.Category != "test-category" {
		t.Errorf("Expected category 'test-category', got %s", entry.Category)
	}
	if entry.CreatedBy != "creator-key" {
		t.Errorf("Expected creator 'creator-key', got %s", entry.CreatedBy)
	}
	if entry.Version != 1 {
		t.Errorf("Initial version should be 1, got %d", entry.Version)
	}
	if entry.Status != EntryStatusDraft {
		t.Errorf("Initial status should be draft, got %s", entry.Status)
	}
	if entry.ContentHash == "" {
		t.Error("ContentHash should be computed")
	}
}

func TestKnowledgeEntry_ComputeContentHash(t *testing.T) {
	entry1 := &KnowledgeEntry{
		Title:    "Title",
		Content:  "Content",
		Version:  1,
		JSONData: nil,
	}

	entry2 := &KnowledgeEntry{
		Title:    "Title",
		Content:  "Content",
		Version:  1,
		JSONData: nil,
	}

	hash1 := entry1.ComputeContentHash()
	hash2 := entry2.ComputeContentHash()

	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}

	// Different content should produce different hash
	entry3 := &KnowledgeEntry{
		Title:    "Different Title",
		Content:  "Content",
		Version:  1,
	}
	hash3 := entry3.ComputeContentHash()

	if hash1 == hash3 {
		t.Error("Different content should produce different hash")
	}
}

func TestKnowledgeEntry_ToJSON(t *testing.T) {
	entry := &KnowledgeEntry{
		ID:       "test-id",
		Title:    "Test Title",
		Content:  "Test Content",
		Category: "test",
		Version:  1,
	}

	json, err := entry.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(json) == 0 {
		t.Error("JSON should not be empty")
	}
}

func TestKnowledgeEntry_FromJSON(t *testing.T) {
	entry := &KnowledgeEntry{
		ID:       "test-id",
		Title:    "Test Title",
		Content:  "Test Content",
		Category: "test",
		Version:  1,
	}

	json, _ := entry.ToJSON()

	newEntry := &KnowledgeEntry{}
	err := newEntry.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newEntry.ID != entry.ID {
		t.Errorf("ID mismatch: expected %s, got %s", entry.ID, newEntry.ID)
	}
	if newEntry.Title != entry.Title {
		t.Errorf("Title mismatch: expected %s, got %s", entry.Title, newEntry.Title)
	}
}

func TestKnowledgeEntry_FromJSON_Invalid(t *testing.T) {
	entry := &KnowledgeEntry{}
	err := entry.FromJSON([]byte("invalid json"))
	if err == nil {
		t.Error("FromJSON with invalid JSON should return error")
	}
}

func TestUser_ToJSON_FromJSON(t *testing.T) {
	user := &User{
		PublicKey:       "test-pubkey",
		AgentName:       "Test Agent",
		UserLevel:       UserLevelLv2,
		Email:           "test@example.com",
		EmailVerified:   true,
		ContributionCnt: 10,
		RatingCnt:       20,
		Status:          UserStatusActive,
	}

	json, err := user.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	newUser := &User{}
	err = newUser.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newUser.PublicKey != user.PublicKey {
		t.Errorf("PublicKey mismatch")
	}
	if newUser.AgentName != user.AgentName {
		t.Errorf("AgentName mismatch")
	}
	if newUser.UserLevel != user.UserLevel {
		t.Errorf("UserLevel mismatch")
	}
}

func TestRating_ToJSON_FromJSON(t *testing.T) {
	rating := &Rating{
		ID:            "rating-1",
		EntryId:       "entry-1",
		RaterPubkey:   "rater-1",
		Score:         4.5,
		Weight:        1.0,
		WeightedScore: 4.5,
		Comment:       "Great!",
	}

	json, err := rating.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	newRating := &Rating{}
	err = newRating.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newRating.ID != rating.ID {
		t.Errorf("ID mismatch")
	}
	if newRating.Score != rating.Score {
		t.Errorf("Score mismatch")
	}
}

func TestCategory_ToJSON_FromJSON(t *testing.T) {
	cat := &Category{
		ID:          "cat-1",
		Path:        "tech/programming",
		Name:        "Programming",
		ParentId:    "cat-tech",
		Level:       2,
		SortOrder:   1,
		IsBuiltin:   true,
		MaintainedBy: "admin",
	}

	json, err := cat.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	newCat := &Category{}
	err = newCat.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newCat.Path != cat.Path {
		t.Errorf("Path mismatch")
	}
}

func TestNodeInfo_ToJSON_FromJSON(t *testing.T) {
	node := &NodeInfo{
		NodeId:         "node-1",
		NodeType:       NodeTypeFull,
		PeerId:         "peer-123",
		PublicKey:      "pubkey",
		Addresses:      []string{"/ip4/1.2.3.4/tcp/8080"},
		Version:        "v1.0.0",
		EntryCount:     100,
		CategoryMirror: []string{"tech", "science"},
		LastSync:       1234567890,
		Uptime:         86400,
		AllowMirror:    true,
		BandwidthLimit: 1024000,
	}

	json, err := node.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	newNode := &NodeInfo{}
	err = newNode.FromJSON(json)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if newNode.NodeId != node.NodeId {
		t.Errorf("NodeId mismatch")
	}
	if len(newNode.Addresses) != len(node.Addresses) {
		t.Errorf("Addresses length mismatch")
	}
}

func TestIsCJK(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'中', true},  // CJK统一汉字
		{'文', true},  // CJK统一汉字
		{'漢', true},  // CJK统一汉字
		{'。', true},  // CJK标点符号
		{'、', true},  // CJK标点符号
		{'ａ', true},  // 全角字符
		{'ｚ', true},  // 全角字符
		{'０', true},  // 全角数字
		{'a', false},  // ASCII
		{'Z', false},  // ASCII
		{' ', false},  // Space
		{'あ', false}, // Hiragana (不在 CJK 范围内)
	}

	for _, tt := range tests {
		result := IsCJK(tt.char)
		if result != tt.expected {
			t.Errorf("IsCJK(%c) = %v, expected %v", tt.char, result, tt.expected)
		}
	}
}

func TestContainsCJK(t *testing.T) {
	tests := []struct {
		str      string
		expected bool
	}{
		{"Hello World", false},
		{"你好世界", true},
		{"Hello 世界", true},
		{"日本語", true},
		{"", false},
		{"123", false},
		{"测试Test", true},
	}

	for _, tt := range tests {
		result := ContainsCJK(tt.str)
		if result != tt.expected {
			t.Errorf("ContainsCJK(%q) = %v, expected %v", tt.str, result, tt.expected)
		}
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test", "test"},
		{"TEST", "test"},
		{"  test  ", "test"},
		{"  Test Key  ", "test key"},
		{"", ""},
	}

	for _, tt := range tests {
		result := NormalizeKey(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeKey(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

func TestUserBanType(t *testing.T) {
	user := &User{
		PublicKey: "test-key",
		Status:    UserStatusBanned,
		BanType:   BanTypeReadonly,
		BanReason: "违规操作",
	}

	if user.Status != UserStatusBanned {
		t.Errorf("Expected status %s, got %s", UserStatusBanned, user.Status)
	}
	if user.BanType != BanTypeReadonly {
		t.Errorf("Expected BanType %s, got %s", BanTypeReadonly, user.BanType)
	}
	if user.BanReason != "违规操作" {
		t.Errorf("Expected BanReason '违规操作', got %s", user.BanReason)
	}
}

func TestUserBanTypeDefaults(t *testing.T) {
	user := &User{
		PublicKey: "test-key",
		Status:    UserStatusBanned,
	}
	// 默认封禁类型应该是 full（空字符串时 IsFullBanned 返回 true）
	if !user.IsFullBanned() {
		t.Error("Empty BanType should be treated as full ban")
	}
}

func TestUserBanHelperMethods(t *testing.T) {
	// Test IsFullBanned
	fullBannedUser := &User{
		PublicKey: "test-key",
		Status:    UserStatusBanned,
		BanType:   BanTypeFull,
	}
	if !fullBannedUser.IsFullBanned() {
		t.Error("User with BanTypeFull should be fully banned")
	}
	if fullBannedUser.IsReadOnly() {
		t.Error("Fully banned user should not be read-only")
	}

	// Test IsReadOnly
	readOnlyUser := &User{
		PublicKey: "test-key",
		Status:    UserStatusBanned,
		BanType:   BanTypeReadonly,
	}
	if !readOnlyUser.IsReadOnly() {
		t.Error("User with BanTypeReadonly should be read-only")
	}
	if readOnlyUser.IsFullBanned() {
		t.Error("Read-only user should not be fully banned")
	}

	// Test active user
	activeUser := &User{
		PublicKey: "test-key",
		Status:    UserStatusActive,
	}
	if activeUser.IsBanned() {
		t.Error("Active user should not be banned")
	}
	if activeUser.IsFullBanned() {
		t.Error("Active user should not be fully banned")
	}
	if activeUser.IsReadOnly() {
		t.Error("Active user should not be read-only")
	}
}
