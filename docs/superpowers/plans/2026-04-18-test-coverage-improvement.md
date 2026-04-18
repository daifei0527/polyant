# 测试覆盖率提升实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将所有 <75% 的模块提升至 75%+ 测试覆盖率

**Architecture:** 采用 TDD 方法，为每个模块补充缺失测试。优先补充 P0 模块（host, admin, storage），然后处理 P1 模块（protocol, sync, router, dht）。

**Tech Stack:** Go 1.22+, testing 包, httptest, testify/assert, storage.MemoryStore

---

## 文件结构

**修改文件：**
- `internal/network/host/host_test.go` - 添加 MockHost 测试
- `internal/api/admin/handler_test.go` - 添加 Handler 测试
- `internal/storage/badger_adapter_test.go` - 添加 BacklinkIndex 测试

---

## Task 1: MockHost 测试 - 基础方法

**Files:**
- Modify: `internal/network/host/host_test.go`

- [ ] **Step 1: 编写 MockHost 创建测试**

在 `internal/network/host/host_test.go` 文件末尾添加：

```go
// ==================== MockHost 测试 ====================

// TestNewMockP2PHost 测试创建 MockHost
func TestNewMockP2PHost(t *testing.T) {
	mock := NewMockP2PHost()
	if mock == nil {
		t.Fatal("NewMockP2PHost 不应返回 nil")
	}

	// 验证默认值
	if mock.ID() == "" {
		t.Error("ID 不应为空")
	}

	if mock.NodeID() == "" {
		t.Error("NodeID 不应为空")
	}

	peers := mock.GetConnectedPeers()
	if peers == nil {
		t.Error("GetConnectedPeers 不应返回 nil")
	}
	if len(peers) != 0 {
		t.Errorf("初始连接节点应为空，got %d", len(peers))
	}
}

// TestMockP2PHost_ID 测试 ID 方法
func TestMockP2PHost_ID(t *testing.T) {
	mock := NewMockP2PHost()

	id := mock.ID()
	if id != "mock-peer-id" {
		t.Errorf("ID = %q, want 'mock-peer-id'", id)
	}

	// 测试设置 ID
	mock.SetID("new-mock-id")
	if mock.ID() != "new-mock-id" {
		t.Errorf("SetID 后 ID = %q, want 'new-mock-id'", mock.ID())
	}
}

// TestMockP2PHost_NodeID 测试 NodeID 方法
func TestMockP2PHost_NodeID(t *testing.T) {
	mock := NewMockP2PHost()

	nodeID := mock.NodeID()
	if nodeID != "mock-node" {
		t.Errorf("NodeID = %q, want 'mock-node'", nodeID)
	}

	// 测试设置 NodeID
	mock.SetNodeID("new-node")
	if mock.NodeID() != "new-node" {
		t.Errorf("SetNodeID 后 NodeID = %q, want 'new-node'", mock.NodeID())
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/network/host/... -run TestMockP2PHost
```

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/network/host/host_test.go
git commit -m "test(host): add MockHost basic tests - NewMockP2PHost, ID, NodeID"
```

---

## Task 2: MockHost 测试 - 连接和流方法

**Files:**
- Modify: `internal/network/host/host_test.go`

- [ ] **Step 1: 编写 MockHost 连接和流测试**

在 `internal/network/host/host_test.go` 文件末尾添加：

```go
// TestMockP2PHost_GetConnectedPeers 测试获取连接节点
func TestMockP2PHost_GetConnectedPeers(t *testing.T) {
	mock := NewMockP2PHost()

	// 初始应为空
	peers := mock.GetConnectedPeers()
	if len(peers) != 0 {
		t.Errorf("初始连接节点应为空，got %d", len(peers))
	}

	// 设置连接节点
	mock.SetConnectedPeers([]peer.ID{"peer1", "peer2"})
	peers = mock.GetConnectedPeers()
	if len(peers) != 2 {
		t.Errorf("SetConnectedPeers 后应有 2 个节点，got %d", len(peers))
	}
}

// TestMockP2PHost_Connect 测试连接方法
func TestMockP2PHost_Connect(t *testing.T) {
	mock := NewMockP2PHost()

	// 正常连接
	addr := peer.AddrInfo{ID: "test-peer"}
	err := mock.Connect(context.Background(), addr)
	if err != nil {
		t.Errorf("Connect 不应返回错误: %v", err)
	}

	peers := mock.GetConnectedPeers()
	if len(peers) != 1 {
		t.Errorf("连接后应有 1 个节点，got %d", len(peers))
	}

	// 测试错误情况
	mock.SetConnectError(errors.New("connection failed"))
	err = mock.Connect(context.Background(), addr)
	if err == nil {
		t.Error("设置错误后 Connect 应返回错误")
	}
}

// TestMockP2PHost_NewStream 测试创建流
func TestMockP2PHost_NewStream(t *testing.T) {
	mock := NewMockP2PHost()

	// 正常情况返回 (nil, nil)
	stream, err := mock.NewStream(context.Background(), "test-peer", "/test/protocol")
	if err != nil {
		t.Errorf("NewStream 不应返回错误: %v", err)
	}
	if stream != nil {
		t.Error("MockHost NewStream 应返回 nil stream")
	}

	// 测试错误情况
	mock.SetStreamError(errors.New("stream failed"))
	_, err = mock.NewStream(context.Background(), "test-peer", "/test/protocol")
	if err == nil {
		t.Error("设置错误后 NewStream 应返回错误")
	}
}

// TestMockP2PHost_Close 测试关闭方法
func TestMockP2PHost_Close(t *testing.T) {
	mock := NewMockP2PHost()

	err := mock.Close()
	if err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestMockP2PHost_Reset 测试重置方法
func TestMockP2PHost_Reset(t *testing.T) {
	mock := NewMockP2PHost()

	// 设置一些状态
	mock.SetConnectedPeers([]peer.ID{"peer1", "peer2"})
	mock.SetConnectError(errors.New("error"))
	mock.SetStreamError(errors.New("error"))

	// 重置
	mock.Reset()

	// 验证状态已清除
	peers := mock.GetConnectedPeers()
	if len(peers) != 0 {
		t.Errorf("Reset 后连接节点应为空，got %d", len(peers))
	}

	// 连接应成功
	err := mock.Connect(context.Background(), peer.AddrInfo{ID: "test"})
	if err != nil {
		t.Error("Reset 后 Connect 应成功")
	}
}
```

- [ ] **Step 2: 添加必要的 import**

确保文件顶部有：
```go
import (
	"context"
	"errors"
	"testing"
	// ... 其他 imports
)
```

- [ ] **Step 3: 运行测试验证**

```bash
go test -v ./internal/network/host/... -run TestMockP2PHost
```

Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/network/host/host_test.go
git commit -m "test(host): add MockHost connection and stream tests"
```

---

## Task 3: Admin Handler 测试 - 用户管理

**Files:**
- Modify: `internal/api/admin/handler_test.go`

- [ ] **Step 1: 编写 NewHandler 和 ListUsersHandler 测试**

在 `internal/api/admin/handler_test.go` 文件末尾添加：

```go
// ==================== Admin Handler 测试 ====================

// TestNewHandler 测试创建 Handler
func TestNewHandler(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	if handler == nil {
		t.Fatal("NewHandler 不应返回 nil")
	}
}

// TestListUsersHandler_Success 测试成功获取用户列表
func TestListUsersHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	// 创建测试用户
	for i := 0; i < 3; i++ {
		user := &model.User{
			PublicKey: "test-pk-" + string(rune('a'+i)),
			AgentName: "test-agent-" + string(rune('a'+i)),
			UserLevel: model.UserLevelLv1,
			Status:    model.UserStatusActive,
		}
		store.User.Create(context.Background(), user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ListUsersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	users, ok := data["users"].([]interface{})
	if !ok {
		t.Fatal("Response users is not an array")
	}

	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}
}

// TestListUsersHandler_Empty 测试空用户列表
func TestListUsersHandler_Empty(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ListUsersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	users := data["users"].([]interface{})

	if len(users) != 0 {
		t.Errorf("Expected 0 users, got %d", len(users))
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/api/admin/... -run TestListUsersHandler
```

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/admin/handler_test.go
git commit -m "test(admin): add NewHandler and ListUsersHandler tests"
```

---

## Task 4: Admin Handler 测试 - 用户操作

**Files:**
- Modify: `internal/api/admin/handler_test.go`

- [ ] **Step 1: 编写 Ban/Unban/SetLevel 测试**

在 `internal/api/admin/handler_test.go` 文件末尾添加：

```go
// TestBanUserHandler_Success 测试成功封禁用户
func TestBanUserHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	// 创建测试用户
	user := &model.User{
		PublicKey: "ban-test-pk",
		AgentName: "ban-test-agent",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), user)

	// 封禁用户
	body := `{"public_key": "ban-test-pk"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user/ban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BanUserHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// 验证用户状态
	updated, _ := store.User.Get(context.Background(), "ban-test-pk")
	if updated.Status != model.UserStatusBanned {
		t.Errorf("Expected status banned, got %s", updated.Status)
	}
}

// TestBanUserHandler_NotFound 测试封禁不存在的用户
func TestBanUserHandler_NotFound(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	body := `{"public_key": "non-existent-pk"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user/ban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BanUserHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// TestUnbanUserHandler_Success 测试成功解封用户
func TestUnbanUserHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	// 创建已封禁的用户
	user := &model.User{
		PublicKey: "unban-test-pk",
		AgentName: "unban-test-agent",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusBanned,
	}
	store.User.Create(context.Background(), user)

	// 解封用户
	body := `{"public_key": "unban-test-pk"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user/unban", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UnbanUserHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// 验证用户状态
	updated, _ := store.User.Get(context.Background(), "unban-test-pk")
	if updated.Status != model.UserStatusActive {
		t.Errorf("Expected status active, got %s", updated.Status)
	}
}

// TestSetUserLevelHandler_Success 测试成功设置用户等级
func TestSetUserLevelHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	// 创建测试用户
	user := &model.User{
		PublicKey: "level-test-pk",
		AgentName: "level-test-agent",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), user)

	// 设置用户等级
	body := `{"public_key": "level-test-pk", "level": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user/level", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.SetUserLevelHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// 验证用户等级
	updated, _ := store.User.Get(context.Background(), "level-test-pk")
	if updated.UserLevel != model.UserLevelLv3 {
		t.Errorf("Expected level 3, got %d", updated.UserLevel)
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/api/admin/... -run "TestBanUserHandler|TestUnbanUserHandler|TestSetUserLevelHandler"
```

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/admin/handler_test.go
git commit -m "test(admin): add BanUser, UnbanUser, SetUserLevel handler tests"
```

---

## Task 5: Admin Handler 测试 - 统计 API

**Files:**
- Modify: `internal/api/admin/handler_test.go`

- [ ] **Step 1: 编写统计 Handler 测试**

在 `internal/api/admin/handler_test.go` 文件末尾添加：

```go
// TestGetUserStatsHandler_Success 测试获取用户统计
func TestGetUserStatsHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	// 创建测试用户
	for i := 0; i < 5; i++ {
		user := &model.User{
			PublicKey: "stats-pk-" + string(rune('a'+i)),
			AgentName: "stats-agent-" + string(rune('a'+i)),
			UserLevel: model.UserLevelLv1,
			Status:    model.UserStatusActive,
		}
		store.User.Create(context.Background(), user)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/users", nil)
	w := httptest.NewRecorder()

	handler.GetUserStatsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// TestGetContributionStatsHandler_Success 测试获取贡献统计
func TestGetContributionStatsHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/contributions", nil)
	w := httptest.NewRecorder()

	handler.GetContributionStatsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// TestGetActivityTrendHandler_Success 测试获取活动趋势
func TestGetActivityTrendHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/activity", nil)
	w := httptest.NewRecorder()

	handler.GetActivityTrendHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// TestGetRegistrationTrendHandler_Success 测试获取注册趋势
func TestGetRegistrationTrendHandler_Success(t *testing.T) {
	store, _ := storage.NewMemoryStore()
	handler := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/registrations", nil)
	w := httptest.NewRecorder()

	handler.GetRegistrationTrendHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/api/admin/... -run "TestGetUserStatsHandler|TestGetContributionStatsHandler|TestGetActivityTrendHandler|TestGetRegistrationTrendHandler"
```

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/admin/handler_test.go
git commit -m "test(admin): add stats handler tests - user stats, contribution, activity, registration"
```

---

## Task 6: Storage BacklinkIndex 测试

**Files:**
- Modify: `internal/storage/badger_adapter_test.go`

- [ ] **Step 1: 编写 BacklinkIndex 测试**

在 `internal/storage/badger_adapter_test.go` 文件末尾添加：

```go
// ==================== BacklinkIndex 测试 ====================

// TestNewBadgerBacklinkIndex 测试创建反向链接索引
func TestNewBadgerBacklinkIndex(t *testing.T) {
	idx := NewBadgerBacklinkIndex()
	if idx == nil {
		t.Fatal("NewBadgerBacklinkIndex 不应返回 nil")
	}
}

// TestBacklinkIndex_UpdateIndex 测试更新索引
func TestBacklinkIndex_UpdateIndex(t *testing.T) {
	idx := NewBadgerBacklinkIndex()

	// 添加新条目的链接
	err := idx.UpdateIndex("entry-1", []string{"entry-2", "entry-3"})
	if err != nil {
		t.Fatalf("UpdateIndex 失败: %v", err)
	}

	// 验证 outlinks
	outlinks, _ := idx.GetOutlinks("entry-1")
	if len(outlinks) != 2 {
		t.Errorf("Expected 2 outlinks, got %d", len(outlinks))
	}

	// 验证 backlinks
	backlinks, _ := idx.GetBacklinks("entry-2")
	if len(backlinks) != 1 {
		t.Errorf("Expected 1 backlink for entry-2, got %d", len(backlinks))
	}
}

// TestBacklinkIndex_UpdateIndex_ExistingEntry 测试更新现有条目的索引
func TestBacklinkIndex_UpdateIndex_ExistingEntry(t *testing.T) {
	idx := NewBadgerBacklinkIndex()

	// 初始链接
	idx.UpdateIndex("entry-1", []string{"entry-2", "entry-3"})

	// 更新链接（移除 entry-2，添加 entry-4）
	idx.UpdateIndex("entry-1", []string{"entry-3", "entry-4"})

	// 验证 entry-2 的 backlink 已移除
	backlinks, _ := idx.GetBacklinks("entry-2")
	if len(backlinks) != 0 {
		t.Errorf("entry-2 should have 0 backlinks after update, got %d", len(backlinks))
	}

	// 验证 entry-4 的 backlink 已添加
	backlinks, _ = idx.GetBacklinks("entry-4")
	if len(backlinks) != 1 {
		t.Errorf("entry-4 should have 1 backlink, got %d", len(backlinks))
	}
}

// TestBacklinkIndex_DeleteIndex 测试删除索引
func TestBacklinkIndex_DeleteIndex(t *testing.T) {
	idx := NewBadgerBacklinkIndex()

	// 添加链接
	idx.UpdateIndex("entry-1", []string{"entry-2"})

	// 删除索引
	err := idx.DeleteIndex("entry-1")
	if err != nil {
		t.Fatalf("DeleteIndex 失败: %v", err)
	}

	// 验证 outlinks 已删除
	outlinks, _ := idx.GetOutlinks("entry-1")
	if len(outlinks) != 0 {
		t.Errorf("Expected 0 outlinks after delete, got %d", len(outlinks))
	}

	// 验证 backlinks 已删除
	backlinks, _ := idx.GetBacklinks("entry-2")
	if len(backlinks) != 0 {
		t.Errorf("Expected 0 backlinks after delete, got %d", len(backlinks))
	}
}

// TestBacklinkIndex_GetBacklinks_Empty 测试获取空反向链接
func TestBacklinkIndex_GetBacklinks_Empty(t *testing.T) {
	idx := NewBadgerBacklinkIndex()

	backlinks, err := idx.GetBacklinks("non-existent")
	if err != nil {
		t.Fatalf("GetBacklinks 失败: %v", err)
	}

	if len(backlinks) != 0 {
		t.Errorf("Expected empty backlinks for non-existent entry, got %d", len(backlinks))
	}
}

// TestBacklinkIndex_GetOutlinks_Empty 测试获取空正向链接
func TestBacklinkIndex_GetOutlinks_Empty(t *testing.T) {
	idx := NewBadgerBacklinkIndex()

	outlinks, err := idx.GetOutlinks("non-existent")
	if err != nil {
		t.Fatalf("GetOutlinks 失败: %v", err)
	}

	if len(outlinks) != 0 {
		t.Errorf("Expected empty outlinks for non-existent entry, got %d", len(outlinks))
	}
}

// TestBacklinkIndex_SelfLink 测试自链接被忽略
func TestBacklinkIndex_SelfLink(t *testing.T) {
	idx := NewBadgerBacklinkIndex()

	// 条目链接到自己（应被忽略）
	idx.UpdateIndex("entry-1", []string{"entry-1", "entry-2"})

	// 验证 outlinks 不包含自己
	outlinks, _ := idx.GetOutlinks("entry-1")
	if len(outlinks) != 1 {
		t.Errorf("Expected 1 outlink (self-link ignored), got %d", len(outlinks))
	}

	// 验证 entry-1 没有 backlink 指向自己
	backlinks, _ := idx.GetBacklinks("entry-1")
	if len(backlinks) != 0 {
		t.Errorf("Expected 0 backlinks for self, got %d", len(backlinks))
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/storage/... -run TestBacklinkIndex
```

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/storage/badger_adapter_test.go
git commit -m "test(storage): add BacklinkIndex tests - update, delete, get backlinks/outlinks"
```

---

## Task 7: 运行完整测试并验证覆盖率

**Files:**
- 无文件修改，仅运行测试

- [ ] **Step 1: 运行所有测试**

```bash
go test -v ./internal/network/host/...
go test -v ./internal/api/admin/...
go test -v ./internal/storage/...
```

Expected: All tests PASS

- [ ] **Step 2: 运行覆盖率分析**

```bash
go test -cover ./internal/network/host/...
go test -cover ./internal/api/admin/...
go test -cover ./internal/storage/...
```

Expected: 覆盖率提升

- [ ] **Step 3: 生成覆盖率报告**

```bash
go test -coverprofile=/tmp/coverage.out ./internal/network/host/... ./internal/api/admin/... ./internal/storage/...
go tool cover -func=/tmp/coverage.out | tail -5
```

- [ ] **Step 4: 最终提交**

```bash
git add docs/superpowers/plans/2026-04-18-test-coverage-improvement.md
git commit -m "docs: add test coverage improvement implementation plan"
```

---

## 总结

本计划补充了以下测试：

1. **host 模块**: MockHost 基础方法、连接/流方法测试
2. **admin 模块**: NewHandler、用户管理、统计 API 测试
3. **storage 模块**: BacklinkIndex 完整测试

**验收标准：**
- 所有新增测试通过
- host 模块覆盖率 ≥ 75%
- admin 模块覆盖率 ≥ 75%
- storage 模块覆盖率 ≥ 75%
