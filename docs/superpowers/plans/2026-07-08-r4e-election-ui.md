# R4e Election Management UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin SPA election management (create/list/view/close) via new session-token endpoints, and fix the pre-existing bare-string pubkey bug that made the Ed25519 election API non-functional.

**Architecture:** 3 tasks — (1) bundled fix: `election_handler.go` 4 bare-string `Value("public_key")` → typed `mw.PublicKeyKey`; (2) new `AdminElectionHandler` (session-token, 4 endpoints) + admin.Handler wiring + router; (3) SPA `/elections` list + `/elections/:id` detail + create/close.

**Tech Stack:** Go 1.25.x / net/http / Vue 3 + Vite + element-plus.

## Global Constraints

- **Go 1.25.x**; module `github.com/daifei0527/polyant`.
- **The bug**: `internal/api/handler/election_handler.go` lines 70, 193, 249, 330 read `r.Context().Value("public_key")` (bare string). NO setter exists for bare-string `"public_key"` — only the typed `mw.PublicKeyKey` (`internal/api/middleware/auth.go:47`, set by both `auth.go:208` Ed25519 and `admin/middleware.go:49` session). So these reads ALWAYS return `""`. The fix: change all 4 to `r.Context().Value(mw.PublicKeyKey).(string)` + add the `mw` import.
- **`ElectionService`** (`internal/core/election/election.go`): `CreateElection(ctx, title, description, createdBy string, voteThreshold int32, durationDays int, autoElect bool) (*model.Election, error)`; `ListElections(ctx, status model.ElectionStatus) ([]*model.Election, error)`; `GetElection(ctx, id) (*model.Election, error)`; `ListCandidates(ctx, id) ([]*model.Candidate, error)`; `CloseElection(ctx, id) ([]*model.Candidate, error)`. Construct via `election.NewElectionService(kv.NewElectionStore(kvStore), kv.NewCandidateStore(kvStore), kv.NewVoteStore(kvStore))`.
- **`CreateElectionRequest`** (`election_handler.go:30`): `{Title, Description string; VoteThreshold int32; DurationDays int; AutoElect bool}` (JSON tags: `title,description,vote_threshold,duration_days,auto_elect` — verify exact tags).
- **`extractPathParam(path, prefix, suffix string) string`** (`election_handler.go:367`) — reusable in the `handler` package for the admin handler with prefix `"/api/v1/admin/elections/"`.
- **`admin.NewHandler(store, entryPusher, backupDir, engine)`** (`admin/handler.go:20`) — has `store *storage.Store`; `store.KVStore()` gives the `kv.Store` to build `ElectionService`. No signature change needed for R4e.
- **admin routes** registered in `registerAdminRoutes` (`router.go`) under `adminAuthMW`. Pattern (from R4b/R4d): exact path for list/create (`/api/v1/admin/elections`), trailing-slash suffix-routed for detail/close (`/api/v1/admin/elections/`).
- **SPA**: `request.js` unwraps `{code,data}`; new `/elections` + `/elections/:id` routes (permission 4); sidebar item.
- **Canonical verification block — run before every commit**:
  ```
  gofmt -l $(find . -name '*.go' -not -path './vendor/*')
  go build ./cmd/... ./internal/... ./pkg/...
  go vet ./...
  go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
  golangci-lint run ./...
  ```
- **Commit prefixes**: `fix(election)` for the bug fix, `feat(election-ui)` for endpoints/SPA. End every message with blank line + `Co-Authored-By: Claude <noreply@anthropic.com>`.
- **Line numbers** reference master `3490838`; they shift — locate by symbol.
- **Spec**: `docs/superpowers/specs/2026-07-08-polyant-r4e-election-ui-design.md`.

---

## Task 1: Fix bare-string pubkey bug in election_handler.go (bundled fix)

**Files:**
- Modify: `internal/api/handler/election_handler.go` (4 sites + import)
- Test: `internal/api/handler/election_handler_test.go` (add handler-level test)

**Interfaces:**
- Produces: the 4 Ed25519 election handlers (Create/Nominate/Vote/Confirm) now correctly read the typed `mw.PublicKeyKey` → non-empty createdBy/voterID/nominatedBy/userID.

- [ ] **Step 1: Write the failing test**

Add to `internal/api/handler/election_handler_test.go` (create if absent; `package handler`). This verifies CreateElectionHandler reads the typed key:
```go
func TestCreateElectionHandler_UsesTypedContextKey(t *testing.T) {
	store := newTestKVStore(t) // reuse existing helper; if absent, kv.NewMemoryStore()
	h := NewElectionHandler(store)

	body := strings.NewReader(`{"title":"T","description":"D","vote_threshold":1,"duration_days":1,"auto_elect":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/elections/create", body)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "pk-admin"))
	rec := httptest.NewRecorder()
	h.CreateElectionHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int    `json:"code"`
		Data struct{ ElectionID string `json:"election_id"` } `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data.ElectionID == "" { t.Fatal("no election_id") }

	// verify CreatedBy was set to the typed-key value (not empty)
	elections, _, _ := store... // list elections via the kv store or service; assert the election's CreatedBy == "pk-admin"
	// simplest: re-fetch via the service the handler used
}
```
**Note:** The cleanest assertion is to fetch the created election and check `CreatedBy == "pk-admin"`. If `NewElectionHandler` exposes its service or the store lets you list elections, use that. If wiring a fetch is awkward, assert indirectly: the handler returns 201 (before the fix, with empty pubkey, CreateElection still succeeds but CreatedBy is "" — so to make the test FAIL before the fix, you must assert CreatedBy is non-empty). Read the existing test file + `NewElectionHandler` to find the cleanest fetch path (e.g., the handler's `electionSvc` is unexported, but `store` can list `election:` keys, or add a `GetElection` via a fresh service). The test MUST fail before the fix (empty CreatedBy) and pass after.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestCreateElectionHandler_UsesTypedContextKey ./internal/api/handler/...`
Expected: FAIL — CreatedBy is "" (bare-string read returns empty).

- [ ] **Step 3: Apply the fix**

In `internal/api/handler/election_handler.go`:
(a) Add the import:
```go
	mw "github.com/daifei0527/polyant/internal/api/middleware"
```
(b) Change all 4 sites (lines 70, 193, 249, 330) from:
```go
	publicKey, _ := r.Context().Value("public_key").(string)
```
to:
```go
	publicKey, _ := r.Context().Value(mw.PublicKeyKey).(string)
```
(Use `replace_all` if the line is identical at all 4 sites — confirm via grep first.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -race -count=1 ./internal/api/handler/...`
Expected: PASS (CreatedBy == "pk-admin").

- [ ] **Step 5: Verify + commit**

Canonical verification block. Then:
```bash
git add internal/api/handler/election_handler.go internal/api/handler/election_handler_test.go
git commit -m "fix(election): use typed mw.PublicKeyKey instead of bare-string (Ed25519 election API was non-functional)

election_handler.go read Value(\"public_key\") (bare string) at 4 sites; no
setter exists -> always empty. Votes attributed to \"\", HasVoted(\"\") blocked
all subsequent votes; elections created with empty creator. Fix: read typed
mw.PublicKeyKey (set by both Ed25519 auth + admin session middleware).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Admin election endpoints (session-token) + wiring

**Files:**
- Create: `internal/api/handler/election_admin_handler.go`
- Create: `internal/api/handler/election_admin_handler_test.go`
- Modify: `internal/api/admin/handler.go` (add electionHandler field + delegation + construct service)
- Modify: `internal/api/router/router.go` (register 4 admin election routes under adminAuthMW)

**Interfaces:**
- Consumes: `election.ElectionService` (Task 1 unchanged), `mw.PublicKeyKey`, `extractPathParam`.
- Produces: `handler.AdminElectionHandler` with `NewAdminElectionHandler(svc *election.ElectionService)` + `CreateElectionHandler`/`ListElectionsHandler`/`GetElectionHandler`/`CloseElectionHandler`. 4 endpoints under `/api/v1/admin/elections*` (session-token).

- [ ] **Step 1: Write the failing handler test**

Create `internal/api/handler/election_admin_handler_test.go` (`package handler`):
```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/election"
	"github.com/daifei0527/polyant/internal/storage/kv"
)

func newAdminElectionHandler(t *testing.T) (*AdminElectionHandler, kv.Store) {
	t.Helper()
	store := kv.NewMemoryStore()
	svc := election.NewElectionService(kv.NewElectionStore(store), kv.NewCandidateStore(store), kv.NewVoteStore(store))
	return NewAdminElectionHandler(svc), store
}

func TestAdminCreateElection_UsesSessionPubkey(t *testing.T) {
	h, _ := newAdminElectionHandler(t)
	body := strings.NewReader(`{"title":"T","description":"D","vote_threshold":1,"duration_days":1,"auto_elect":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/elections", body)
	req = req.WithContext(context.WithValue(req.Context(), mw.PublicKeyKey, "pk-admin"))
	rec := httptest.NewRecorder()
	h.CreateElectionHandler(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminListElections(t *testing.T) {
	h, _ := newAdminElectionHandler(t)
	// create one, then list
	h.svc.CreateElection(context.Background(), "T", "D", "pk", 1, 1, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/elections", nil)
	rec := httptest.NewRecorder()
	h.ListElectionsHandler(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("status=%d", rec.Code) }
	var resp struct{ Data struct{ Elections []json.RawMessage `json:"elections"` } `json:"data"` }
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data.Elections) == 0 { t.Error("expected >=1 election") }
}
```
**Note:** confirm `h.svc` is accessible (the struct field name). If unexported, expose it or construct the election via a public method. The test accesses `h.svc.CreateElection` to seed — if `svc` is unexported, make it accessible or seed via the handler's Create endpoint instead.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestAdminCreateElection|TestAdminListElections' ./internal/api/handler/...`
Expected: FAIL — `AdminElectionHandler`/`NewAdminElectionHandler` undefined.

- [ ] **Step 3: Implement the admin election handler**

Create `internal/api/handler/election_admin_handler.go` (`package handler`). It wraps `*election.ElectionService`, reads `mw.PublicKeyKey` for create, uses `extractPathParam` with the admin prefix:
```go
package handler

import (
	"encoding/json"
	"net/http"

	mw "github.com/daifei0527/polyant/internal/api/middleware"
	"github.com/daifei0527/polyant/internal/core/election"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// AdminElectionHandler exposes election management to the admin SPA (session-token).
type AdminElectionHandler struct {
	svc *election.ElectionService
}

func NewAdminElectionHandler(svc *election.ElectionService) *AdminElectionHandler {
	return &AdminElectionHandler{svc: svc}
}

// CreateElectionHandler  POST /api/v1/admin/elections
func (h *AdminElectionHandler) CreateElectionHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateElectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: "invalid request body"})
		return
	}
	createdBy, _ := r.Context().Value(mw.PublicKeyKey).(string)
	el, err := h.svc.CreateElection(r.Context(), req.Title, req.Description, createdBy, req.VoteThreshold, req.DurationDays, req.AutoElect)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &APIResponse{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"election_id": el.ID, "auto_elect": el.AutoElect,
	}})
}

// ListElectionsHandler  GET /api/v1/admin/elections?status=
func (h *AdminElectionHandler) ListElectionsHandler(w http.ResponseWriter, r *http.Request) {
	status := model.ElectionStatus(r.URL.Query().Get("status")) // "" => list all (service accepts empty)
	elections, err := h.svc.ListElections(r.Context(), status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, &APIResponse{Code: 500, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{"elections": elections}})
}

// GetElectionHandler  GET /api/v1/admin/elections/{id}
func (h *AdminElectionHandler) GetElectionHandler(w http.ResponseWriter, r *http.Request) {
	electionID := extractPathParam(r.URL.Path, "/api/v1/admin/elections/", "")
	el, err := h.svc.GetElection(r.Context(), electionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, &APIResponse{Code: 404, Message: "election not found"})
		return
	}
	candidates, _ := h.svc.ListCandidates(r.Context(), electionID)
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"election": el, "candidates": candidates,
	}})
}

// CloseElectionHandler  POST /api/v1/admin/elections/{id}/close
func (h *AdminElectionHandler) CloseElectionHandler(w http.ResponseWriter, r *http.Request) {
	electionID := extractPathParam(r.URL.Path, "/api/v1/admin/elections/", "/close")
	if electionID == "" {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: "missing election_id"})
		return
	}
	elected, err := h.svc.CloseElection(r.Context(), electionID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, &APIResponse{Code: 400, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{"elected": elected}})
}
```
**Verify:** `CreateElectionRequest` is already defined in `election_handler.go:30` (same package, reusable). `extractPathParam(path, prefix, suffix)` — for GetElection (no suffix), pass `""`; confirm `extractPathParam` handles empty suffix (returns the segment after prefix up to end or next `/`). If it requires a non-empty suffix, inline the id extraction for Get. Read `extractPathParam` (`election_handler.go:367`) to confirm behavior with empty suffix.

- [ ] **Step 4: Run handler tests + verify + commit (handler)**

Canonical verification block. Commit:
```bash
git add internal/api/handler/election_admin_handler.go internal/api/handler/election_admin_handler_test.go
git commit -m "feat(election-ui): admin election handler (create/list/get/close, session-token)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 5: Wire into admin.Handler + router**

(a) `internal/api/admin/handler.go`: add `electionHandler *handler.AdminElectionHandler` field + 4 delegating methods + construct in `NewHandler`:
```go
	// inside NewHandler, after the other handlers (within the store != nil block or unconditionally):
	kvStore := store.KVStore()
	electionSvc := election.NewElectionService(kv.NewElectionStore(kvStore), kv.NewCandidateStore(kvStore), kv.NewVoteStore(kvStore))
	h.electionHandler = handler.NewAdminElectionHandler(electionSvc)
```
Add imports: `election`, `kv`. Add delegating methods: `CreateElectionHandler`/`ListElectionsHandler`/`GetElectionHandler`/`CloseElectionHandler` each calling `h.electionHandler.X(w,r)`.

(b) `internal/api/router/router.go` `registerAdminRoutes`: register under `adminAuthMW`:
```go
	// 选举管理（R4e：session-token，admin SPA 可达）
	mux.Handle("/api/v1/admin/elections",
		adminAuthMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				adminHandler.ListElectionsHandler(w, r)
			case http.MethodPost:
				adminHandler.CreateElectionHandler(w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		})))
	mux.Handle("/api/v1/admin/elections/",
		adminAuthMW.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/close") {
				adminHandler.CloseElectionHandler(w, r)
				return
			}
			adminHandler.GetElectionHandler(w, r) // GET /{id}
		})))
```
(Confirm `strings` is imported in router.go — it is, used by other admin routes.)

- [ ] **Step 6: Verify build + full suite + commit (wiring)**

Canonical verification block. Commit:
```bash
git add internal/api/admin/handler.go internal/api/router/router.go
git commit -m "feat(election-ui): wire admin election endpoints (4 routes under adminAuthMW)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: SPA election list + detail + create + close

**Files:**
- Create: `web/admin/src/api/elections.js`
- Create: `web/admin/src/views/elections/List.vue`
- Create: `web/admin/src/views/elections/Detail.vue`
- Modify: `web/admin/src/router/index.js` (add `/elections` + `/elections/:id`)
- Modify: `web/admin/src/components/Sidebar.vue` (add menu item + icon)
- Build + sync: `internal/api/admin/dist/`

- [ ] **Step 1: Create the API client**

Create `web/admin/src/api/elections.js` (mirror `api/users.js`):
```js
import request from './request'

export function listElections(params) {
  return request.get('/admin/elections', { params })
}
export function getElection(id) {
  return request.get(`/admin/elections/${id}`)
}
export function createElection(data) {
  // data: { title, description, vote_threshold, duration_days, auto_elect }
  return request.post('/admin/elections', data)
}
export function closeElection(id) {
  return request.post(`/admin/elections/${id}/close`)
}
```

- [ ] **Step 2: Create the list view**

Create `web/admin/src/views/elections/List.vue` (mirror `views/users/List.vue` + the create-form pattern). Active/closed tab + create button (ElMessageBox or a simple dialog with form fields: title/description/voteThreshold/durationDays/autoElect) + table (title/status/threshold/time/actions: 查看/关闭):
```vue
<template>
  <div class="elections-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>选举管理</span>
          <div>
            <el-radio-group v-model="statusFilter" @change="fetchElections" style="margin-right: 12px;">
              <el-radio-button label="">全部</el-radio-button>
              <el-radio-button label="active">进行中</el-radio-button>
              <el-radio-button label="closed">已关闭</el-radio-button>
            </el-radio-group>
            <el-button type="primary" @click="showCreate = true">创建选举</el-button>
          </div>
        </div>
      </template>
      <el-table :data="elections" v-loading="loading">
        <el-table-column prop="title" label="标题" min-width="180" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }"><el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '进行中' : '已关闭' }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="vote_threshold" label="阈值" width="80" />
        <el-table-column prop="auto_elect" label="自动当选" width="100"><template #default="{ row }">{{ row.auto_elect ? '是' : '否' }}</template></el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button size="small" @click="router.push(`/elections/${row.id}`)">详情</el-button>
            <el-button v-if="row.status === 'active'" size="small" type="warning" @click="handleClose(row)">关闭</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showCreate" title="创建选举" width="500px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" /></el-form-item>
        <el-form-item label="当选阈值"><el-input-number v-model="form.vote_threshold" :min="1" /></el-form-item>
        <el-form-item label="持续天数"><el-input-number v-model="form.duration_days" :min="1" /></el-form-item>
        <el-form-item label="自动当选"><el-switch v-model="form.auto_elect" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listElections, createElection, closeElection } from '@/api/elections'

const router = useRouter()
const loading = ref(false)
const elections = ref([])
const statusFilter = ref('active')
const showCreate = ref(false)
const creating = ref(false)
const form = ref({ title: '', description: '', vote_threshold: 1, duration_days: 7, auto_elect: true })

const fetchElections = async () => {
  loading.value = true
  try {
    const res = await listElections({ status: statusFilter.value })
    elections.value = res.elections || []
  } catch (e) { console.error('fetch elections:', e) }
  finally { loading.value = false }
}
const handleCreate = async () => {
  creating.value = true
  try {
    await createElection(form.value)
    ElMessage.success('创建成功')
    showCreate.value = false
    fetchElections()
  } catch (e) { ElMessage.error('创建失败: ' + (e.message || e)) }
  finally { creating.value = false }
}
const handleClose = async (row) => {
  try {
    await ElMessageBox.confirm(`确认关闭选举「${row.title}」？`, '关闭选举', { type: 'warning' })
    await closeElection(row.id)
    ElMessage.success('已关闭')
    fetchElections()
  } catch (e) { if (e !== 'cancel') ElMessage.error('关闭失败: ' + (e.message || e)) }
}
onMounted(() => { fetchElections() })
</script>

<style scoped>.card-header { display: flex; justify-content: space-between; align-items: center; }</style>
```

- [ ] **Step 3: Create the detail view**

Create `web/admin/src/views/elections/Detail.vue` (election info + candidates table with VoteCount/Status/Confirmed):
```vue
<template>
  <div class="election-detail">
    <el-page-header @back="router.back()" :content="election?.title || '选举详情'" style="margin-bottom: 16px;" />
    <el-card v-if="election" style="margin-bottom: 16px;">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="标题">{{ election.title }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ election.status === 'active' ? '进行中' : '已关闭' }}</el-descriptions-item>
        <el-descriptions-item label="描述">{{ election.description }}</el-descriptions-item>
        <el-descriptions-item label="当选阈值">{{ election.vote_threshold }}</el-descriptions-item>
        <el-descriptions-item label="自动当选">{{ election.auto_elect ? '是' : '否' }}</el-descriptions-item>
        <el-descriptions-item label="创建者">{{ (election.created_by || '').slice(0, 16) }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
    <el-card>
      <template #header><span>候选人</span></template>
      <el-table :data="candidates" v-loading="loading">
        <el-table-column prop="user_name" label="名称" />
        <el-table-column prop="vote_count" label="票数" width="100" />
        <el-table-column prop="status" label="状态" width="120"><template #default="{ row }"><el-tag :type="row.status === 'elected' ? 'success' : row.status === 'rejected' ? 'danger' : ''">{{ row.status }}</el-tag></template></el-table-column>
        <el-table-column prop="confirmed" label="已确认" width="100"><template #default="{ row }">{{ row.confirmed ? '是' : '否' }}</template></el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getElection } from '@/api/elections'

const route = useRoute()
const router = useRouter()
const election = ref(null)
const candidates = ref([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    const res = await getElection(route.params.id)
    election.value = res.election
    candidates.value = res.candidates || []
  } catch (e) { console.error('fetch election:', e) }
  finally { loading.value = false }
})
</script>
```

- [ ] **Step 4: Register routes + sidebar**

`web/admin/src/router/index.js` — add 2 children:
```js
        { path: 'elections', name: 'Elections', component: () => import('@/views/elections/List.vue'), meta: { permission: 4, title: '选举管理' } },
        { path: 'elections/:id', name: 'ElectionDetail', component: () => import('@/views/elections/Detail.vue'), meta: { permission: 4, title: '选举详情' } },
```
`web/admin/src/components/Sidebar.vue` — add menu item (icon `Ticket` or `Trophy`) + import:
```vue
    <el-menu-item index="/elections" v-if="hasPermission(4)">
      <el-icon><Ticket /></el-icon>
      <span>选举管理</span>
    </el-menu-item>
```

- [ ] **Step 5: Build + sync dist + verify + commit**

```bash
cd web/admin && npm run build && cd -
rm -rf internal/api/admin/dist && cp -r web/admin/dist internal/api/admin/dist
```
Canonical Go verification. Commit:
```bash
git add web/admin/src/ internal/api/admin/dist/
git commit -m "feat(election-ui): admin SPA elections page (list + detail + create + close)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Final verification (after all 3 tasks)

- [ ] `gofmt -l` repo-wide empty; `go build ./cmd/... ./internal/... ./pkg/...` OK; `go vet ./...` OK.
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` all PASS.
- [ ] `golangci-lint run ./...` exit 0.
- [ ] `cd web/admin && npm run build` succeeds; `internal/api/admin/dist/index.html` exists.
- [ ] Manual smoke (optional): admin SPA `/elections` → create → appears in list → detail shows candidates → close.
- [ ] Branch `r4e-election-ui` has 4 task commits + spec/plan, ready for review/merge.
