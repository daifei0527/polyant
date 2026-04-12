// Package kv_test 提供键值存储的单元测试
package kv_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/agentwiki/internal/storage/kv"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ==================== 内存存储实现（用于测试）====================

// MemoryStore 内存存储实现
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
	if _, ok := s.data[string(key)]; !ok {
		return kv.ErrKeyNotFound
	}
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

// 确保实现 Store 接口
var _ kv.Store = (*MemoryStore)(nil)

// ==================== Store 接口测试 ====================

// TestMemoryStoreBasic 测试内存存储基本操作
func TestMemoryStoreBasic(t *testing.T) {
	store := NewMemoryStore()

	// 测试 Put 和 Get
	key := []byte("test:key")
	value := []byte("test value")

	if err := store.Put(key, value); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Get 返回值错误: got %q, want %q", got, value)
	}
}

// TestMemoryStoreNotFound 测试获取不存在的键
func TestMemoryStoreNotFound(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Get([]byte("nonexistent"))
	if err != kv.ErrKeyNotFound {
		t.Errorf("期望 ErrKeyNotFound, got %v", err)
	}
}

// TestMemoryStoreDelete 测试删除操作
func TestMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()

	key := []byte("test:delete")
	value := []byte("to be deleted")

	// 先添加
	store.Put(key, value)

	// 删除
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	// 验证已删除
	_, err := store.Get(key)
	if err != kv.ErrKeyNotFound {
		t.Error("删除后键不应存在")
	}
}

// TestMemoryStoreDeleteNotFound 测试删除不存在的键
func TestMemoryStoreDeleteNotFound(t *testing.T) {
	store := NewMemoryStore()

	err := store.Delete([]byte("nonexistent"))
	if err != kv.ErrKeyNotFound {
		t.Errorf("删除不存在的键应返回 ErrKeyNotFound, got %v", err)
	}
}

// TestMemoryStoreScan 测试前缀扫描
func TestMemoryStoreScan(t *testing.T) {
	store := NewMemoryStore()

	// 添加多个键
	store.Put([]byte("entry:1"), []byte("entry1"))
	store.Put([]byte("entry:2"), []byte("entry2"))
	store.Put([]byte("user:1"), []byte("user1"))

	// 扫描 entry: 前缀
	result, err := store.Scan([]byte("entry:"))
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Scan 返回 %d 个结果, 期望 2", len(result))
	}

	// 扫描 user: 前缀
	result, err = store.Scan([]byte("user:"))
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Scan 返回 %d 个结果, 期望 1", len(result))
	}

	// 扫描不存在的 前缀
	result, err = store.Scan([]byte("nonexistent:"))
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Scan 返回 %d 个结果, 期望 0", len(result))
	}
}

// ==================== EntryStore 测试 ====================

// TestEntryStoreCreate 测试创建条目
func TestEntryStoreCreate(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	entry := &model.KnowledgeEntry{
		ID:       "test-entry-1",
		Title:    "测试条目",
		Content:  "这是测试内容",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}

	if err := entryStore.CreateEntry(entry); err != nil {
		t.Fatalf("CreateEntry 失败: %v", err)
	}

	// 验证已创建
	got, err := entryStore.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("GetEntry 失败: %v", err)
	}

	if got.Title != entry.Title {
		t.Errorf("Title 不匹配: got %q, want %q", got.Title, entry.Title)
	}

	if got.ContentHash == "" {
		t.Error("ContentHash 应该被自动计算")
	}
}

// TestEntryStoreCreateDuplicate 测试创建重复条目
func TestEntryStoreCreateDuplicate(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	entry := &model.KnowledgeEntry{
		ID:      "dup-entry",
		Title:   "重复测试",
		Content: "内容",
	}

	entryStore.CreateEntry(entry)

	// 尝试创建相同 ID 的条目
	err := entryStore.CreateEntry(&model.KnowledgeEntry{
		ID:      "dup-entry",
		Title:   "另一个",
		Content: "其他内容",
	})

	if err == nil {
		t.Error("创建重复条目应返回错误")
	}
}

// TestEntryStoreCreateEmptyID 测试创建空 ID 条目
func TestEntryStoreCreateEmptyID(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	entry := &model.KnowledgeEntry{
		Title:   "无 ID 条目",
		Content: "内容",
	}

	err := entryStore.CreateEntry(entry)
	if err == nil {
		t.Error("创建空 ID 条目应返回错误")
	}
}

// TestEntryStoreUpdate 测试更新条目
func TestEntryStoreUpdate(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	// 创建条目
	entry := &model.KnowledgeEntry{
		ID:       "update-test",
		Title:    "原始标题",
		Content:  "原始内容",
		Category: "test",
		Version:  1, // 初始版本
	}
	entryStore.CreateEntry(entry)

	// 更新条目
	entry.Title = "更新后标题"
	entry.Content = "更新后内容"

	if err := entryStore.UpdateEntry(entry); err != nil {
		t.Fatalf("UpdateEntry 失败: %v", err)
	}

	// 验证更新
	got, err := entryStore.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("GetEntry 失败: %v", err)
	}

	if got.Title != "更新后标题" {
		t.Errorf("Title 未更新: got %q", got.Title)
	}

	// Version 应该增加
	if got.Version != 2 {
		t.Errorf("Version 应为 2, got %d", got.Version)
	}
}

// TestEntryStoreUpdateNotFound 测试更新不存在的条目
func TestEntryStoreUpdateNotFound(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	entry := &model.KnowledgeEntry{
		ID:      "nonexistent",
		Title:   "标题",
		Content: "内容",
	}

	err := entryStore.UpdateEntry(entry)
	if err == nil {
		t.Error("更新不存在的条目应返回错误")
	}
}

// TestEntryStoreDelete 测试删除条目
func TestEntryStoreDelete(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	entry := &model.KnowledgeEntry{
		ID:      "delete-test",
		Title:   "待删除",
		Content: "内容",
	}
	entryStore.CreateEntry(entry)

	if err := entryStore.DeleteEntry(entry.ID); err != nil {
		t.Fatalf("DeleteEntry 失败: %v", err)
	}

	// 验证已删除
	_, err := entryStore.GetEntry(entry.ID)
	if err == nil {
		t.Error("删除后条目不应存在")
	}
}

// TestEntryStoreList 测试列出条目
func TestEntryStoreList(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	// 创建多个条目
	for i := 0; i < 5; i++ {
		entry := &model.KnowledgeEntry{
			ID:       string(rune('a' + i)),
			Title:    "条目",
			Content:  "内容",
			Category: "test",
			Status:   model.EntryStatusPublished,
		}
		entryStore.CreateEntry(entry)
	}

	// 列出所有条目
	entries, err := entryStore.ListEntries(0, 10)
	if err != nil {
		t.Fatalf("ListEntries 失败: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("期望 5 个条目, got %d", len(entries))
	}
}

// TestEntryStoreListPagination 测试分页
func TestEntryStoreListPagination(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	// 创建 10 个条目
	for i := 0; i < 10; i++ {
		entry := &model.KnowledgeEntry{
			ID:       string(rune('a' + i)),
			Title:    "条目",
			Content:  "内容",
			Category: "test",
			Status:   model.EntryStatusPublished,
		}
		entryStore.CreateEntry(entry)
	}

	// 第一页
	page1, err := entryStore.ListEntries(0, 5)
	if err != nil {
		t.Fatalf("ListEntries 失败: %v", err)
	}
	if len(page1) != 5 {
		t.Errorf("第一页应有 5 个条目, got %d", len(page1))
	}

	// 第二页
	page2, err := entryStore.ListEntries(5, 5)
	if err != nil {
		t.Fatalf("ListEntries 失败: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("第二页应有 5 个条目, got %d", len(page2))
	}

	// 超出范围的偏移
	page3, err := entryStore.ListEntries(20, 5)
	if err != nil {
		t.Fatalf("ListEntries 失败: %v", err)
	}
	if len(page3) != 0 {
		t.Errorf("超出范围应返回空, got %d", len(page3))
	}
}

// TestEntryStoreListByCategory 测试按分类列出
func TestEntryStoreListByCategory(t *testing.T) {
	store := NewMemoryStore()
	entryStore := kv.NewEntryStore(store)

	// 创建不同分类的条目
	entryStore.CreateEntry(&model.KnowledgeEntry{ID: "1", Title: "条目1", Category: "tech", Status: model.EntryStatusPublished})
	entryStore.CreateEntry(&model.KnowledgeEntry{ID: "2", Title: "条目2", Category: "tech", Status: model.EntryStatusPublished})
	entryStore.CreateEntry(&model.KnowledgeEntry{ID: "3", Title: "条目3", Category: "other", Status: model.EntryStatusPublished})

	// 按 tech 分类查询
	entries, err := entryStore.ListByCategory("tech")
	if err != nil {
		t.Fatalf("ListByCategory 失败: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("tech 分类应有 2 个条目, got %d", len(entries))
	}

	// 按 other 分类查询
	entries, err = entryStore.ListByCategory("other")
	if err != nil {
		t.Fatalf("ListByCategory 失败: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("other 分类应有 1 个条目, got %d", len(entries))
	}
}

// ==================== UserStore 测试 ====================

// TestUserStoreCreate 测试创建用户
func TestUserStoreCreate(t *testing.T) {
	store := NewMemoryStore()
	userStore := kv.NewUserStore(store)

	user := &model.User{
		PublicKey:    "test-pubkey-1",
		AgentName:    "测试用户",
		UserLevel:    model.UserLevelLv1,
		RegisteredAt: model.NowMillis(),
	}

	if err := userStore.CreateUser(user); err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}

	// 验证已创建
	got, err := userStore.GetUser(user.PublicKey)
	if err != nil {
		t.Fatalf("GetUser 失败: %v", err)
	}

	if got.AgentName != user.AgentName {
		t.Errorf("AgentName 不匹配: got %q, want %q", got.AgentName, user.AgentName)
	}
}

// TestUserStoreCreateDuplicate 测试创建重复用户
func TestUserStoreCreateDuplicate(t *testing.T) {
	store := NewMemoryStore()
	userStore := kv.NewUserStore(store)

	user := &model.User{
		PublicKey: "dup-pubkey",
		AgentName: "重复用户",
	}
	userStore.CreateUser(user)

	err := userStore.CreateUser(&model.User{
		PublicKey: "dup-pubkey",
		AgentName: "另一个用户",
	})

	if err == nil {
		t.Error("创建重复用户应返回错误")
	}
}

// TestUserStoreCreateEmptyPublicKey 测试创建空公钥用户
func TestUserStoreCreateEmptyPublicKey(t *testing.T) {
	store := NewMemoryStore()
	userStore := kv.NewUserStore(store)

	user := &model.User{
		AgentName: "无公钥用户",
	}

	err := userStore.CreateUser(user)
	if err == nil {
		t.Error("创建空公钥用户应返回错误")
	}
}

// TestUserStoreUpdate 测试更新用户
func TestUserStoreUpdate(t *testing.T) {
	store := NewMemoryStore()
	userStore := kv.NewUserStore(store)

	user := &model.User{
		PublicKey: "update-pubkey",
		AgentName: "原始名称",
	}
	userStore.CreateUser(user)

	// 更新用户
	user.AgentName = "更新后名称"
	user.Email = "test@example.com"

	if err := userStore.UpdateUser(user); err != nil {
		t.Fatalf("UpdateUser 失败: %v", err)
	}

	// 验证更新
	got, err := userStore.GetUser(user.PublicKey)
	if err != nil {
		t.Fatalf("GetUser 失败: %v", err)
	}

	if got.AgentName != "更新后名称" {
		t.Errorf("AgentName 未更新: got %q", got.AgentName)
	}

	if got.Email != "test@example.com" {
		t.Errorf("Email 未更新: got %q", got.Email)
	}
}

// TestUserStoreList 测试列出用户
func TestUserStoreList(t *testing.T) {
	store := NewMemoryStore()
	userStore := kv.NewUserStore(store)

	// 创建多个用户
	for i := 0; i < 5; i++ {
		user := &model.User{
			PublicKey:    string(rune('a' + i)),
			AgentName:    "用户",
			RegisteredAt: model.NowMillis(),
		}
		userStore.CreateUser(user)
	}

	// 列出用户
	users, err := userStore.ListUsers(0, 10)
	if err != nil {
		t.Fatalf("ListUsers 失败: %v", err)
	}

	if len(users) != 5 {
		t.Errorf("期望 5 个用户, got %d", len(users))
	}
}

// TestUserStoreGetByEmail 测试按邮箱查询用户
func TestUserStoreGetByEmail(t *testing.T) {
	store := NewMemoryStore()
	userStore := kv.NewUserStore(store)

	user := &model.User{
		PublicKey: "email-pubkey",
		AgentName: "邮箱用户",
		Email:     "test@example.com",
	}
	userStore.CreateUser(user)

	// 按邮箱查询
	got, err := userStore.GetByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail 失败: %v", err)
	}

	if got.PublicKey != user.PublicKey {
		t.Errorf("PublicKey 不匹配: got %q, want %q", got.PublicKey, user.PublicKey)
	}

	// 查询不存在的邮箱
	_, err = userStore.GetByEmail(context.Background(), "nonexistent@example.com")
	if err == nil {
		t.Error("查询不存在的邮箱应返回错误")
	}
}

// ==================== RatingStore 测试 ====================

// TestRatingStoreCreate 测试创建评分
func TestRatingStoreCreate(t *testing.T) {
	store := NewMemoryStore()
	ratingStore := kv.NewRatingStore(store)

	rating := &model.Rating{
		ID:          "rating-1",
		EntryId:     "entry-1",
		RaterPubkey: "rater-1",
		Score:       4.5,
		Weight:      1.0,
		Comment:     "很好的内容",
	}

	if err := ratingStore.CreateRating(rating); err != nil {
		t.Fatalf("CreateRating 失败: %v", err)
	}

	// 验证已创建
	got, err := ratingStore.GetRating(rating.EntryId, rating.RaterPubkey)
	if err != nil {
		t.Fatalf("GetRating 失败: %v", err)
	}

	if got.Score != rating.Score {
		t.Errorf("Score 不匹配: got %f, want %f", got.Score, rating.Score)
	}

	if got.WeightedScore != rating.Score*rating.Weight {
		t.Errorf("WeightedScore 应为 %f, got %f", rating.Score*rating.Weight, got.WeightedScore)
	}
}

// TestRatingStoreCreateDuplicate 测试重复评分
func TestRatingStoreCreateDuplicate(t *testing.T) {
	store := NewMemoryStore()
	ratingStore := kv.NewRatingStore(store)

	rating := &model.Rating{
		EntryId:     "entry-dup",
		RaterPubkey: "rater-dup",
		Score:       4.0,
		Weight:      1.0,
	}
	ratingStore.CreateRating(rating)

	// 尝试再次评分
	err := ratingStore.CreateRating(&model.Rating{
		EntryId:     "entry-dup",
		RaterPubkey: "rater-dup",
		Score:       5.0,
		Weight:      1.0,
	})

	if err == nil {
		t.Error("重复评分应返回错误")
	}
}

// TestRatingStoreCreateEmptyFields 测试创建空字段评分
func TestRatingStoreCreateEmptyFields(t *testing.T) {
	store := NewMemoryStore()
	ratingStore := kv.NewRatingStore(store)

	// 空 EntryId
	err := ratingStore.CreateRating(&model.Rating{
		RaterPubkey: "rater",
		Score:       4.0,
	})
	if err == nil {
		t.Error("空 EntryId 应返回错误")
	}

	// 空 RaterPubkey
	err = ratingStore.CreateRating(&model.Rating{
		EntryId: "entry",
		Score:   4.0,
	})
	if err == nil {
		t.Error("空 RaterPubkey 应返回错误")
	}
}

// TestRatingStoreGetByEntry 测试获取条目所有评分
func TestRatingStoreGetByEntry(t *testing.T) {
	store := NewMemoryStore()
	ratingStore := kv.NewRatingStore(store)

	// 为同一条目创建多个评分
	for i := 0; i < 3; i++ {
		rating := &model.Rating{
			EntryId:     "entry-multi",
			RaterPubkey: string(rune('a' + i)),
			Score:       float64(3 + i),
			Weight:      1.0,
		}
		ratingStore.CreateRating(rating)
	}

	// 获取条目的所有评分
	ratings, err := ratingStore.GetRatingsByEntry("entry-multi")
	if err != nil {
		t.Fatalf("GetRatingsByEntry 失败: %v", err)
	}

	if len(ratings) != 3 {
		t.Errorf("期望 3 个评分, got %d", len(ratings))
	}

	// 获取不存在条目的评分
	ratings, err = ratingStore.GetRatingsByEntry("nonexistent")
	if err != nil {
		t.Fatalf("GetRatingsByEntry 失败: %v", err)
	}

	if len(ratings) != 0 {
		t.Errorf("不存在条目应返回空, got %d", len(ratings))
	}
}

// TestRatingStoreUpdateEntryScore 测试计算加权平均分
func TestRatingStoreUpdateEntryScore(t *testing.T) {
	store := NewMemoryStore()
	ratingStore := kv.NewRatingStore(store)

	// 创建不同权重的评分
	ratingStore.CreateRating(&model.Rating{
		EntryId:     "entry-score",
		RaterPubkey: "rater1",
		Score:       4.0,
		Weight:      1.0, // Lv1
	})

	ratingStore.CreateRating(&model.Rating{
		EntryId:     "entry-score",
		RaterPubkey: "rater2",
		Score:       5.0,
		Weight:      2.0, // Lv4
	})

	// 计算加权平均
	// (4.0 * 1.0 + 5.0 * 2.0) / (1.0 + 2.0) = 14.0 / 3.0 ≈ 4.67
	avg, err := ratingStore.UpdateEntryScore("entry-score")
	if err != nil {
		t.Fatalf("UpdateEntryScore 失败: %v", err)
	}

	expected := (4.0*1.0 + 5.0*2.0) / 3.0
	if avg < expected-0.01 || avg > expected+0.01 {
		t.Errorf("加权平均分错误: got %f, want ~%f", avg, expected)
	}
}

// TestRatingStoreUpdateEntryScoreNoRatings 测试无评分时的计算
func TestRatingStoreUpdateEntryScoreNoRatings(t *testing.T) {
	store := NewMemoryStore()
	ratingStore := kv.NewRatingStore(store)

	avg, err := ratingStore.UpdateEntryScore("no-ratings")
	if err != nil {
		t.Fatalf("UpdateEntryScore 失败: %v", err)
	}

	if avg != 0 {
		t.Errorf("无评分时应返回 0, got %f", avg)
	}
}

// ==================== JSONFileStore 测试 ====================

// TestJSONFileStoreBasic 测试 JSON 文件存储基本操作
func TestJSONFileStoreBasic(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "kv-json-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.json")
	store, err := kv.NewJSONFileStore(filePath)
	if err != nil {
		t.Fatalf("NewJSONFileStore 失败: %v", err)
	}

	// 测试 Put
	if err := store.Put([]byte("key1"), []byte("value1")); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	// 测试 Get
	val, err := store.Get([]byte("key1"))
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("Get 返回错误值: got %q, want %q", val, "value1")
	}

	// 测试持久化
	store.Close()

	// 重新加载验证持久化
	store2, err := kv.NewJSONFileStore(filePath)
	if err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}
	defer store2.Close()

	val2, err := store2.Get([]byte("key1"))
	if err != nil {
		t.Fatalf("重新加载后 Get 失败: %v", err)
	}
	if string(val2) != "value1" {
		t.Errorf("持久化后值错误: got %q, want %q", val2, "value1")
	}
}

// TestJSONFileStoreDelete 测试 JSON 文件存储删除
func TestJSONFileStoreDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kv-json-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.json")
	store, err := kv.NewJSONFileStore(filePath)
	if err != nil {
		t.Fatalf("NewJSONFileStore 失败: %v", err)
	}
	defer store.Close()

	store.Put([]byte("key1"), []byte("value1"))

	if err := store.Delete([]byte("key1")); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	_, err = store.Get([]byte("key1"))
	if err != kv.ErrKeyNotFound {
		t.Error("删除后键不应存在")
	}
}

// TestJSONFileStoreScan 测试 JSON 文件存储扫描
func TestJSONFileStoreScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kv-json-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.json")
	store, err := kv.NewJSONFileStore(filePath)
	if err != nil {
		t.Fatalf("NewJSONFileStore 失败: %v", err)
	}
	defer store.Close()

	store.Put([]byte("entry:1"), []byte("v1"))
	store.Put([]byte("entry:2"), []byte("v2"))
	store.Put([]byte("user:1"), []byte("u1"))

	result, err := store.Scan([]byte("entry:"))
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("期望 2 个结果, got %d", len(result))
	}
}

// ==================== 工厂函数测试 ====================

// TestNewStoreJSONFile 测试工厂函数创建 JSON 文件存储
func TestNewStoreJSONFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kv-factory-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.json")
	store, err := kv.NewStore(kv.StoreTypeJSONFile, filePath)
	if err != nil {
		t.Fatalf("NewStore 失败: %v", err)
	}
	defer store.Close()

	if err := store.Put([]byte("test"), []byte("value")); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
}

// TestNewStoreUnknownType 测试工厂函数未知类型
func TestNewStoreUnknownType(t *testing.T) {
	_, err := kv.NewStore("unknown", "/path")
	if err == nil {
		t.Error("未知存储类型应返回错误")
	}
}

// TestNewStorePebble 测试工厂函数创建 Pebble 存储
func TestNewStorePebble(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kv-pebble-factory-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := kv.NewStore(kv.StoreTypePebble, tmpDir)
	if err != nil {
		t.Fatalf("NewStore 失败: %v", err)
	}
	defer store.Close()

	// 测试基本操作
	key := []byte("factory-test-key")
	value := []byte("factory-test-value")

	if err := store.Put(key, value); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Get 返回错误值: got %q, want %q", got, value)
	}
}
