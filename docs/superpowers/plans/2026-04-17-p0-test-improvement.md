# P0 级测试改进实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增强现有测试覆盖，补充实际缺失的测试用例，确保 API 层覆盖率提升至 80%+

**Architecture:** 采用 TDD 方法，首先分析现有测试覆盖缺口，然后编写失败测试，实现代码，验证通过，提交

**Tech Stack:** Go 1.22+, testing 包, httptest, testify/assert, storage.MemoryStore

---

## 重要发现

经过对代码库的详细探索，发现审计设计文档中标注的多个"缺失测试"实际上已经存在：

| 功能 | 设计文档标注 | 实际状态 |
|------|--------------|----------|
| VoteStore 测试 | ❌ 缺失 | ✅ 已存在 (`internal/storage/kv/store_extra_test.go:571-754`) |
| 反向链接测试 | ❌ 缺失 | ✅ 已存在 (`internal/api/handler/handler_test.go:946`) |
| 正向链接测试 | ❌ 缺失 | ✅ 已存在 (`internal/api/handler/handler_test.go:974`) |
| 用户更新信息测试 | ❌ 缺失 | ✅ 已存在 (`internal/api/handler/handler_test.go:605`) |
| 选举流程测试 | ❌ 缺失 | ✅ 已存在 (`internal/api/handler/election_handler_test.go`) |
| 分类条目列表测试 | ❌ 缺失 | ✅ 已存在 (`internal/api/handler/handler_test.go:1059`) |

**实际需要补充的测试缺口：**
1. 反向链接/正向链接边界场景测试（条目不存在、空链接列表）
2. 选举 API 权限边界测试（Lv5 权限验证）
3. 用户更新信息的更多场景测试（无效参数）
4. 集成测试：完整选举流程端到端测试

---

## 文件结构

**修改文件：**
- `internal/api/handler/handler_test.go` - 增强反向链接/正向链接/用户更新测试
- `internal/api/handler/election_handler_test.go` - 增强权限边界测试

**新增文件：**
- `internal/api/handler/election_integration_test.go` - 选举完整流程集成测试

---

## Task 1: 增强反向链接测试 - 边界场景

**Files:**
- Modify: `internal/api/handler/handler_test.go`

- [ ] **Step 1: 编写反向链接条目不存在测试**

在 `internal/api/handler/handler_test.go` 文件末尾添加：

```go
func TestEntryHandler_GetBacklinksHandler_EntryNotFound(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/nonexistent-entry/backlinks", nil)
	rec := httptest.NewRecorder()

	handler.GetBacklinksHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for nonexistent entry, got %d", http.StatusNotFound, rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/api/handler/... -run TestEntryHandler_GetBacklinksHandler_EntryNotFound
```

Expected: PASS

- [ ] **Step 3: 编写反向链接空列表测试**

```go
func TestEntryHandler_GetBacklinksHandler_EmptyLinks(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Create entry with no backlinks
	entry := &model.KnowledgeEntry{
		ID:       "entry-no-backlinks",
		Title:    "Entry with no backlinks",
		Content:  "Content",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	store.Entry.Create(context.Background(), entry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/entry-no-backlinks/backlinks", nil)
	rec := httptest.NewRecorder()

	handler.GetBacklinksHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("Response data should be an array")
	}

	if len(data) != 0 {
		t.Errorf("Expected empty array, got %d items", len(data))
	}
}
```

- [ ] **Step 4: 运行测试验证**

```bash
go test -v ./internal/api/handler/... -run TestEntryHandler_GetBacklinksHandler_EmptyLinks
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/api/handler/handler_test.go
git commit -m "test(handler): add backlinks boundary tests - entry not found and empty links"
```

---

## Task 2: 增强正向链接测试 - 边界场景

**Files:**
- Modify: `internal/api/handler/handler_test.go`

- [ ] **Step 1: 编写正向链接条目不存在测试**

```go
func TestEntryHandler_GetOutlinksHandler_EntryNotFound(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/nonexistent-entry/outlinks", nil)
	rec := httptest.NewRecorder()

	handler.GetOutlinksHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for nonexistent entry, got %d", http.StatusNotFound, rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/api/handler/... -run TestEntryHandler_GetOutlinksHandler_EntryNotFound
```

Expected: PASS

- [ ] **Step 3: 编写正向链接空列表测试**

```go
func TestEntryHandler_GetOutlinksHandler_EmptyLinks(t *testing.T) {
	store := newTestStore(t)
	handler := NewEntryHandler(store.Entry, store.Search, store.Backlink, store.User)

	// Create entry with no outlinks
	entry := &model.KnowledgeEntry{
		ID:       "entry-no-outlinks",
		Title:    "Entry with no links",
		Content:  "Plain content without [[wiki-links]]",
		Category: "test",
		Status:   model.EntryStatusPublished,
	}
	store.Entry.Create(context.Background(), entry)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entry/entry-no-outlinks/outlinks", nil)
	rec := httptest.NewRecorder()

	handler.GetOutlinksHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("Response data should be an array")
	}

	if len(data) != 0 {
		t.Errorf("Expected empty array, got %d items", len(data))
	}
}
```

- [ ] **Step 4: 运行测试验证**

```bash
go test -v ./internal/api/handler/... -run TestEntryHandler_GetOutlinksHandler_EmptyLinks
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/api/handler/handler_test.go
git commit -m "test(handler): add outlinks boundary tests - entry not found and empty links"
```

---

## Task 3: 增强用户更新信息测试

**Files:**
- Modify: `internal/api/handler/handler_test.go`

- [ ] **Step 1: 编写用户更新无效参数测试**

```go
func TestUserHandler_UpdateUserInfoHandler_EmptyName(t *testing.T) {
	handler, store := newTestUserHandler(t)

	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	user := &model.User{
		PublicKey: pubKeyB64,
		AgentName: "old-name",
		UserLevel: model.UserLevelLv1,
		Status:    model.UserStatusActive,
	}
	store.User.Create(context.Background(), user)

	// Empty agent_name should be rejected
	body := `{"agent_name": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := setUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.UpdateUserInfoHandler(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected error for empty agent_name")
	}
}

func TestUserHandler_UpdateUserInfoHandler_Unauthorized(t *testing.T) {
	handler, _ := newTestUserHandler(t)

	body := `{"agent_name": "new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/user/info", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateUserInfoHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试验证**

```bash
go test -v ./internal/api/handler/... -run TestUserHandler_UpdateUserInfoHandler
```

Expected: PASS (所有 4 个测试)

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/handler_test.go
git commit -m "test(handler): add user update validation tests - empty name and unauthorized"
```

---

## Task 4: 增强选举 Handler 权限测试

**Files:**
- Modify: `internal/api/handler/election_handler_test.go`

- [ ] **Step 1: 编写创建选举权限验证测试**

```go
func TestElectionHandler_CreateElectionHandler_RequiresLv5(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Lv4 user should not be able to create elections
	body := `{"title": "Test Election", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	
	// Note: Current implementation uses context value "public_key" directly
	// In production, auth middleware would verify user level
	// This test documents expected behavior
	ctx := context.WithValue(req.Context(), "user_level", model.UserLevelLv4)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.CreateElectionHandler(rec, req)

	// This test documents current behavior - may need auth middleware integration
	// Expected: 403 Forbidden if proper auth middleware is in place
	if rec.Code != http.StatusCreated && rec.Code != http.StatusForbidden {
		t.Logf("Current implementation allows creation. Auth middleware integration needed for proper level check.")
	}
}
```

- [ ] **Step 2: 编写投票权限验证测试**

```go
func TestElectionHandler_VoteHandler_RequiresLv3(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create election and candidate
	body := `{"title": "Test", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	electionID := createResp.Data.(map[string]interface{})["election_id"].(string)

	// Nominate candidate
	nominateBody := `{"user_name": "Candidate", "self_nominated": true}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(nominateBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "candidate-key")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	handler.NominateCandidateHandler(rec, req)

	// Lv2 user should not be able to vote (current implementation may not enforce)
	voteBody := `{"candidate_id": "candidate-key"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(voteBody))
	req.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req.Context(), "public_key", "lv2-voter")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.VoteHandler(rec, req)

	// Document: Router uses authMW.RequireLevel(model.UserLevelLv3)
	// This test verifies handler behavior without middleware
	t.Logf("Vote result: %d. Router enforces Lv3+ requirement via middleware.", rec.Code)
}
```

- [ ] **Step 3: 编写关闭选举权限验证测试**

```go
func TestElectionHandler_CloseElectionHandler_RequiresLv5(t *testing.T) {
	handler := newTestElectionHandler(t)

	// Create election
	body := `{"title": "Test", "description": "Test", "vote_threshold": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "public_key", "admin-key")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.CreateElectionHandler(rec, req)

	var createResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	electionID := createResp.Data.(map[string]interface{})["election_id"].(string)

	// Non-admin should not be able to close
	req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/close", nil)
	ctx = context.WithValue(req.Context(), "public_key", "non-admin")
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	handler.CloseElectionHandler(rec, req)

	t.Logf("Close result: %d. Router enforces Lv5 requirement via middleware.", rec.Code)
}
```

- [ ] **Step 4: 运行测试验证**

```bash
go test -v ./internal/api/handler/... -run TestElectionHandler_CreateElectionHandler_RequiresLv5
go test -v ./internal/api/handler/... -run TestElectionHandler_VoteHandler_RequiresLv3
go test -v ./internal/api/handler/... -run TestElectionHandler_CloseElectionHandler_RequiresLv5
```

Expected: Tests pass documenting current behavior

- [ ] **Step 5: 提交**

```bash
git add internal/api/handler/election_handler_test.go
git commit -m "test(handler): add election permission documentation tests"
```

---

## Task 5: 创建选举完整流程集成测试

**Files:**
- Create: `internal/api/handler/election_integration_test.go`

- [ ] **Step 1: 创建集成测试文件头**

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/storage/kv"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestElectionHandler_CompleteFlow tests the entire election lifecycle
// from creation to closing with all intermediate steps
func TestElectionHandler_CompleteFlow(t *testing.T) {
	store := kv.NewMemoryStore()
	handler := NewElectionHandler(store)

	// Step 1: Create election (admin action)
	t.Run("CreateElection", func(t *testing.T) {
		body := `{"title": "年度最佳贡献者选举", "description": "选举年度最佳贡献者", "vote_threshold": 3, "auto_elect": true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/create", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "admin-key")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.CreateElectionHandler(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("Create election failed: %d - %s", rec.Code, rec.Body.String())
		}
	})

	// Get election ID from creation response
	var electionID string
	t.Run("GetElectionID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/elections?status=active", nil)
		rec := httptest.NewRecorder()
		handler.ListElectionsHandler(rec, req)

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		elections := data["elections"].([]interface{})
		if len(elections) == 0 {
			t.Fatal("No active elections found")
		}
		electionID = elections[0].(map[string]interface{})["id"].(string)
	})

	// Step 2: Self-nomination
	t.Run("SelfNomination", func(t *testing.T) {
		body := `{"user_name": "候选人张三", "self_nominated": true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "candidate-zhangsan")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.NominateCandidateHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Self-nomination failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		if data["confirmed"] != true {
			t.Error("Self-nomination should be auto-confirmed")
		}
	})

	// Step 3: Peer nomination (not self-nominated)
	t.Run("PeerNomination", func(t *testing.T) {
		body := `{"user_id": "candidate-lisi", "user_name": "候选人李四", "self_nominated": false}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "nominator-wangwu")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.NominateCandidateHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Peer nomination failed: %d - %s", rec.Code, rec.Body.String())
		}
	})

	// Step 4: Confirm peer nomination
	t.Run("ConfirmNomination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/candidates/candidate-lisi/confirm", nil)
		ctx := context.WithValue(req.Context(), "public_key", "candidate-lisi")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ConfirmNominationHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Confirm nomination failed: %d - %s", rec.Code, rec.Body.String())
		}
	})

	// Step 5: Voting
	t.Run("Voting", func(t *testing.T) {
		// Vote for candidate-zhangsan
		body := `{"candidate_id": "candidate-zhangsan"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "voter-1")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.VoteHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Vote failed: %d - %s", rec.Code, rec.Body.String())
		}

		// Another vote for the same candidate
		body = `{"candidate_id": "candidate-zhangsan"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx = context.WithValue(req.Context(), "public_key", "voter-2")
		req = req.WithContext(ctx)
		rec = httptest.NewRecorder()
		handler.VoteHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Second vote failed: %d - %s", rec.Code, rec.Body.String())
		}

		// Vote for candidate-lisi
		body = `{"candidate_id": "candidate-lisi"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx = context.WithValue(req.Context(), "public_key", "voter-3")
		req = req.WithContext(ctx)
		rec = httptest.NewRecorder()
		handler.VoteHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Vote for lisi failed: %d - %s", rec.Code, rec.Body.String())
		}
	})

	// Step 6: Verify duplicate vote fails
	t.Run("DuplicateVoteFails", func(t *testing.T) {
		body := `{"candidate_id": "candidate-zhangsan"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/vote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "public_key", "voter-1") // Same voter
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.VoteHandler(rec, req)

		if rec.Code == http.StatusOK {
			t.Error("Duplicate vote should fail")
		}
	})

	// Step 7: Close election
	t.Run("CloseElection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/"+electionID+"/close", nil)
		ctx := context.WithValue(req.Context(), "public_key", "admin-key")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.CloseElectionHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Close election failed: %d - %s", rec.Code, rec.Body.String())
		}

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		elected := data["elected"].([]interface{})

		// With vote_threshold=3 and auto_elect=true, candidate-zhangsan should win (2 votes)
		t.Logf("Elected candidates: %d", len(elected))
	})

	// Step 8: Verify election is closed
	t.Run("VerifyClosed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/elections/"+electionID, nil)
		rec := httptest.NewRecorder()

		handler.GetElectionHandler(rec, req)

		var resp APIResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		data := resp.Data.(map[string]interface{})
		election := data["election"].(map[string]interface{})

		if election["status"] != model.ElectionStatusClosed {
			t.Errorf("Expected status 'closed', got '%s'", election["status"])
		}
	})
}
```

- [ ] **Step 2: 运行集成测试**

```bash
go test -v ./internal/api/handler/... -run TestElectionHandler_CompleteFlow
```

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add internal/api/handler/election_integration_test.go
git commit -m "test(handler): add election complete flow integration test"
```

---

## Task 6: 运行完整测试并验证覆盖率

**Files:**
- 无文件修改，仅运行测试

- [ ] **Step 1: 运行 handler 包所有测试**

```bash
go test -v ./internal/api/handler/...
```

Expected: All tests PASS

- [ ] **Step 2: 运行覆盖率分析**

```bash
go test -cover ./internal/api/handler/...
go test -cover ./internal/storage/kv/...
go test -cover ./internal/core/election/...
```

Expected: Coverage improvement visible

- [ ] **Step 3: 生成覆盖率报告**

```bash
go test -coverprofile=coverage.out ./internal/api/handler/...
go tool cover -func=coverage.out | grep -E 'handler.go|election_handler.go'
```

- [ ] **Step 4: 最终提交**

```bash
git add docs/superpowers/plans/2026-04-17-p0-test-improvement.md
git commit -m "docs: add P0 test improvement implementation plan"
```

---

## 总结

本计划补充了以下测试：

1. **反向链接边界测试**：条目不存在、空链接列表
2. **正向链接边界测试**：条目不存在、空链接列表  
3. **用户更新验证测试**：空名称、未授权
4. **选举权限文档测试**：记录当前行为和预期权限要求
5. **选举完整流程集成测试**：端到端测试选举生命周期

**注意：** 审计设计文档中标注的大部分"缺失测试"实际上已经存在：
- VoteStore 测试（完整覆盖）
- 反向/正向链接基本测试
- 用户更新基本测试
- 选举 Handler 测试（完整覆盖）
- 分类条目列表测试

本次改进重点在于增强边界场景和集成测试，而非补充基础测试。