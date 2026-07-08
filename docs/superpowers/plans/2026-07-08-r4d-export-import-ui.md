# R4d Export/Import UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make data export/import reachable from the admin SPA by migrating the two endpoints to session-token auth, then adding a `/data` page (export checkboxes + ZIP download, import upload + conflict strategy).

**Architecture:** 2 tasks — (1) backend: migrate `/api/v1/admin/export|import` from Ed25519 `authMW` to session-token `adminAuthMW`, and re-source Import's `OperatorLevel` from the admin's publicKey via UserStore; (2) SPA: `api/data.js` (export via raw axios blob bypass + import via FormData) + `views/data/Index.vue` + router + sidebar.

**Tech Stack:** Go 1.25.x / net/http / Vue 3 + Vite + element-plus (el-upload new) + axios (blob download new).

## Global Constraints

- **Go 1.25.x**; module `github.com/daifei0527/polyant`.
- **Endpoints to migrate**: `/api/v1/admin/export` (GET, streams ZIP, `Content-Type: application/zip`) + `/api/v1/admin/import` (POST multipart `file` + `conflict`). Currently in `registerAuthRoutes` (`router.go:404-408`) under `authMW.Middleware(authMW.RequirePermission(rbac.PermManageUser, ...))`. Move to `registerAdminRoutes` under `adminAuthMW.Middleware(...)` (session-token). **No production caller** of the Ed25519 versions (only build-ignored `scripts/test_*.go`).
- **`ExportHandler`** (`internal/api/handler/export_handler.go:16`): struct holds `store *storage.Store, nodeID string`; `NewExportHandler(store, nodeID)`. `ImportHandler` currently gets operator via `getUserFromContext(r.Context())` (`:115`) → `user.UserLevel` (`:124`). Under session-token this returns nil — must re-source from `r.Context().Value(mw.PublicKeyKey)` → `store.User.Get`.
- **admin session context** injects ONLY `mw.PublicKeyKey` (`internal/api/admin/middleware.go:48`), NOT level/user. So Import looks up the admin user by publicKey to get `UserLevel`.
- **`Exporter.Export(ctx, opts)`** (`export/exporter.go:60`) returns `[]byte` ZIP; **`Importer.Import(zipData, opts)`** (`export/importer.go:75`) returns `*ImportResult`. `ImportOptions{ConflictStrategy, OperatorLevel int32}`. Reuse as-is.
- **SPA `request.js`**: response interceptor unwraps `{code,message,data}` → returns `data.data` (`request.js:24-29`). A binary ZIP response is a Blob → `data.code` is undefined → interceptor rejects. **Export MUST bypass** via a raw `axios` call with `responseType:'blob'` + manual `Authorization` header (do NOT edit `request.js`). Import returns the normal JSON envelope → use `request.post`.
- **`el-upload` + blob download are NEW** SPA patterns (not used elsewhere). element-plus `^2.4.4` (el-upload available globally via `main.js`); axios `^1.6.2` (blob supported).
- **admin.NewHandler** is `(store, entryPusher, backupDir, engine)` (`admin/handler.go:20`). To avoid 5-param bloat, register `exh` directly in `registerAdminRoutes` (construct `handler.NewExportHandler(deps.Store, deps.NodeID)` there) rather than routing through admin.Handler — it's still under `adminAuthMW`.
- **Canonical verification block — run before every commit** (gofmt included):
  ```
  gofmt -l $(find . -name '*.go' -not -path './vendor/*')
  go build ./cmd/... ./internal/... ./pkg/...
  go vet ./...
  go test -race -count=1 ./cmd/... ./internal/... ./pkg/...
  golangci-lint run ./...
  ```
- **Commit prefixes**: `feat(data-ops)` for features, `refactor(data-ops)!:` for the auth-migration (breaking for Ed25519 callers — none production). End every message with a blank line then `Co-Authored-By: Claude <noreply@anthropic.com>`.
- **Line numbers** reference master `a20904d`; they shift — locate by symbol/text.
- **Spec**: `docs/superpowers/specs/2026-07-08-polyant-r4d-export-import-ui-design.md`.

---

## Task 1: Migrate export/import endpoints to session-token + re-source OperatorLevel

**Files:**
- Modify: `internal/api/handler/export_handler.go` (ImportHandler: re-source OperatorLevel from session publicKey)
- Modify: `internal/api/router/router.go` (move 2 routes from registerAuthRoutes to registerAdminRoutes under adminAuthMW)
- Modify: `internal/api/handler/export_handler_test.go` (add session-token context test)
- Modify: build-ignored `scripts/test_*.go` — annotate stale (one-line comment); NOT compiled

**Interfaces:**
- Produces: `/api/v1/admin/export` (GET, session-token) streams ZIP; `/api/v1/admin/import` (POST multipart, session-token) → ImportResult. Import's OperatorLevel = the admin session user's level (looked up by publicKey).

- [ ] **Step 1: Write the failing handler test**

Add to `internal/api/handler/export_handler_test.go` (create if absent; `package handler`). This test verifies Import sources OperatorLevel from the session context (admin publicKey → UserStore → level) by round-tripping an export ZIP:
```go
func TestImportHandler_OperatorLevelFromSession(t *testing.T) {
	// setup: memory store with an admin user (Lv4) and one entry
	store := newTestStore(t) // reuse existing helper; if named differently, use storage.NewPersistentStore w/ memory search
	admin := &model.User{PublicKey: "pk-admin", UserLevel: model.UserLevelLv4, Status: model.UserStatusActive, AgentName: "admin"}
	store.User.Create(context.Background(), admin)
	store.Entry.Create(context.Background(), &model.KnowledgeEntry{ID: "e1", Title: "T", Content: "C", Category: "x", Status: model.EntryStatusPublished})

	// build an export ZIP (entries) via the Exporter
	exh := NewExportHandler(store, "test-node")
	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/export?include=entries", nil)
	exportReq = exportReq.WithContext(context.WithValue(exportReq.Context(), mw.PublicKeyKey, "pk-admin"))
	exportRec := httptest.NewRecorder()
	exh.ExportHandler(exportRec, exportReq)
	if exportRec.Code != 200 || exportRec.Body.Len() == 0 {
		t.Fatalf("export: code=%d len=%d", exportRec.Code, exportRec.Body.Len())
	}
	zipBytes := exportRec.Body.Bytes()

	// import the ZIP via the handler with a session context (admin publicKey)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, _ := writer.CreateFormFile("file", "export.zip")
	fw.Write(zipBytes)
	_ = writer.WriteField("conflict", "skip")
	writer.Close()

	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/import", body)
	importReq.Header.Set("Content-Type", writer.FormDataContentType())
	importReq = importReq.WithContext(context.WithValue(importReq.Context(), mw.PublicKeyKey, "pk-admin"))
	importRec := httptest.NewRecorder()
	exh.ImportHandler(importRec, importReq)

	if importRec.Code != 200 {
		t.Fatalf("import: code=%d body=%s", importRec.Code, importRec.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data *export.ImportResult `json:"data"`
	}
	if err := json.Unmarshal(importRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, importRec.Body.String())
	}
	if !resp.Data.Success {
		t.Errorf("import not successful: %+v", resp.Data.Errors)
	}
}
```
**Note:** confirm `newTestStore`, `mw` alias, and the `ExportHandler`/`ImportHandler` method names match the file. The `mw.PublicKeyKey` context value is what the session middleware sets; this test injects it directly to simulate session auth. Add imports (`bytes`, `mime/multipart`, `encoding/json`, `net/http/httptest`, `mw`, `export`, `model`) as needed.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestImportHandler_OperatorLevelFromSession ./internal/api/handler/...`
Expected: FAIL — `getUserFromContext` returns nil under the session context (no UserKey), so OperatorLevel=0 → import fails or wrong level.

- [ ] **Step 3: Re-source OperatorLevel in ImportHandler**

In `internal/api/handler/export_handler.go` `ImportHandler`, replace the `getUserFromContext(r.Context())` based level lookup (around line 115) with a session-based lookup. The handler has `h.store`:
```go
	// OperatorLevel from the admin session: look up the admin user by publicKey
	adminPubKey, _ := r.Context().Value(mw.PublicKeyKey).(string)
	operatorLevel := int32(model.UserLevelLv4) // fallback (admin sessions are Lv4+)
	if adminPubKey != "" {
		if adminUser, err := h.store.User.Get(r.Context(), adminPubKey); err == nil && adminUser != nil {
			operatorLevel = adminUser.UserLevel
		}
	}
```
Then pass `OperatorLevel: operatorLevel` in `ImportOptions` (replacing the previous `user.UserLevel`). Remove the now-unused `getUserFromContext` call + the `_ = user` if it was only used for level. Add the `mw` import (`mw "github.com/daifei0527/polyant/internal/api/middleware"`) and `model` import if not present.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -race -count=1 -run TestImportHandler ./internal/api/handler/...`
Expected: PASS.

- [ ] **Step 5: Migrate the routes**

In `internal/api/router/router.go`:
(a) In `registerAuthRoutes`, REMOVE the two export/import registrations (~lines 404-408):
```go
	// 数据导出 GET /api/v1/admin/export - Lv4+ (Admin)
	mux.Handle("/api/v1/admin/export", authMW.Middleware(authMW.RequirePermission(rbac.PermManageUser, http.HandlerFunc(exh.ExportHandler))))

	// 数据导入 POST /api/v1/admin/import - Lv4+ (Admin)
	mux.Handle("/api/v1/admin/import", authMW.Middleware(authMW.RequirePermission(rbac.PermManageUser, http.HandlerFunc(exh.ImportHandler))))
```
If `exh` becomes unused in `registerAuthRoutes` after removal, remove the `exh *handler.ExportHandler` parameter from the `registerAuthRoutes` signature AND its call site in `NewRouterWithDeps` (grep `registerAuthRoutes(` to find the call).

(b) In `registerAdminRoutes`, ADD (the exportHandler must be available — construct it here from deps, or receive it as a param). Construct it inline (deps has Store + NodeID):
```go
	// 数据导出/导入（R4d：迁移到 session-token，admin SPA 可达）
	dataExh := handler.NewExportHandler(deps.Store, deps.NodeID)
	mux.Handle("/api/v1/admin/export",
		adminAuthMW.Middleware(http.HandlerFunc(dataExh.ExportHandler)))
	mux.Handle("/api/v1/admin/import",
		adminAuthMW.Middleware(http.HandlerFunc(dataExh.ImportHandler)))
```
If `registerAdminRoutes` already receives an `exh` (unlikely) or you prefer threading the existing one, either works — the goal is the two routes under `adminAuthMW`. Confirm `deps.NodeID` exists on the `Dependencies` struct (grep `NodeID` in router.go); if not, pass it through from the node main (it's already used at router.go:169 `handler.NewExportHandler(deps.Store, deps.NodeID)`, so `deps.NodeID` exists).

- [ ] **Step 6: Verify build + lint + full suite**

Run the canonical verification block. Then:
```bash
git add internal/api/handler/export_handler.go internal/api/handler/export_handler_test.go internal/api/router/router.go
git commit -m "refactor(data-ops)!: migrate export/import to session-token adminAuthMW

Endpoints /api/v1/admin/export|import move from Ed25519 authMW to session-token
adminAuthMW (admin SPA reachable; no production Ed25519 caller). Import OperatorLevel
now sourced from admin publicKey via UserStore. Breaking for Ed25519 callers (none prod).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

- [ ] **Step 7: Annotate stale scratch scripts**

In each of `scripts/test_admin.go`, `scripts/test_full.go`, `scripts/test_import.go`, add a one-line comment near the export/import calls noting they're stale (the endpoints migrated to session-token). Do NOT modify the script logic (they're `//go:build ignore`, not compiled). Example comment:
```go
// NOTE: /api/v1/admin/export migrated to session-token (R4d); this Ed25519 call is stale.
```
Then:
```bash
git add scripts/test_admin.go scripts/test_full.go scripts/test_import.go
git commit -m "chore(data-ops): mark stale Ed25519 export/import test scripts (R4d migration)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: SPA export/import page (api/data.js + views/data/Index.vue + router + sidebar)

**Files:**
- Create: `web/admin/src/api/data.js`
- Create: `web/admin/src/views/data/Index.vue`
- Modify: `web/admin/src/router/index.js` (add `/data` route)
- Modify: `web/admin/src/components/Sidebar.vue` (add menu item + icon import)
- Build + sync: `internal/api/admin/dist/`

**Interfaces:**
- Consumes: the 2 migrated endpoints (`GET /api/v1/admin/export?include=...` blob; `POST /api/v1/admin/import` multipart).

- [ ] **Step 1: Create the API client**

Create `web/admin/src/api/data.js`:
```js
import axios from 'axios'
import request from './request'

const token = () => sessionStorage.getItem('admin_token')

// 导出：二进制 ZIP，绕过 request.js 的 JSON envelope 拦截器（raw axios + blob）
export function exportData(include) {
  return axios.get('/api/v1/admin/export', {
    params: { include: include.join(',') },
    responseType: 'blob',
    headers: { Authorization: `Bearer ${token()}` }
  })
}

// 导入：multipart 上传，返回 JSON envelope（走 request 拦截器，自动解包 data.data）
export function importData(file, conflict) {
  const form = new FormData()
  form.append('file', file)
  form.append('conflict', conflict)
  return request.post('/admin/import', form)
}
```
**Key:** `exportData` uses raw `axios` (not `request`) so the blob response isn't mis-parsed by the `{code,data}` interceptor. `importData` uses `request` (normal envelope).

- [ ] **Step 2: Create the data view**

Create `web/admin/src/views/data/Index.vue` (mirror the el-card layout from `views/reviews/List.vue` + the loading-button handler from `views/stats/Index.vue:141-152`):
```vue
<template>
  <div class="data-page">
    <!-- 导出 -->
    <el-card style="margin-bottom: 16px;">
      <template #header>
        <div class="card-header">
          <span>数据导出</span>
          <el-button type="primary" :loading="exportLoading" @click="handleExport">导出 ZIP</el-button>
        </div>
      </template>
      <el-checkbox-group v-model="exportInclude">
        <el-checkbox label="entries">条目</el-checkbox>
        <el-checkbox label="categories">分类</el-checkbox>
        <el-checkbox label="users">用户</el-checkbox>
        <el-checkbox label="ratings">评分</el-checkbox>
      </el-checkbox-group>
    </el-card>

    <!-- 导入 -->
    <el-card>
      <template #header>
        <div class="card-header">
          <span>数据导入</span>
          <el-button type="primary" :loading="importLoading" :disabled="!importFile" @click="handleImport">导入</el-button>
        </div>
      </template>
      <el-upload
        :auto-upload="false"
        :limit="1"
        :on-change="handleFileChange"
        :on-exceed="() => ElMessage.warning('仅支持单个 ZIP 文件')"
        accept=".zip"
      >
        <el-button>选择 ZIP 文件</el-button>
      </el-upload>
      <div style="margin-top: 12px;">
        <span>冲突策略：</span>
        <el-radio-group v-model="conflict">
          <el-radio label="skip">跳过</el-radio>
          <el-radio label="overwrite">覆盖</el-radio>
          <el-radio label="merge">合并</el-radio>
        </el-radio-group>
      </div>
      <div v-if="importResult" style="margin-top: 16px;">
        <el-alert :title="importResult.success ? '导入成功' : '导入完成（有错误/跳过）'" :type="importResult.success ? 'success' : 'warning'" :closable="false" />
        <el-descriptions :column="2" border style="margin-top: 8px;">
          <el-descriptions-item label="条目导入">{{ importResult.summary?.entries_imported || 0 }}</el-descriptions-item>
          <el-descriptions-item label="条目跳过">{{ importResult.summary?.entries_skipped || 0 }}</el-descriptions-item>
          <el-descriptions-item label="用户导入">{{ importResult.summary?.users_imported || 0 }}</el-descriptions-item>
          <el-descriptions-item label="分类导入">{{ importResult.summary?.categories_imported || 0 }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="importResult.errors?.length" style="margin-top: 8px; max-height: 200px; overflow:auto;">
          <div v-for="(e, i) in importResult.errors" :key="i" style="color: #e6a23c; font-size: 12px;">
            [{{ e.type }}/{{ e.id }}] {{ e.message }}
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { exportData, importData } from '@/api/data'

const exportInclude = ref(['entries', 'categories'])
const exportLoading = ref(false)
const importFile = ref(null)
const conflict = ref('skip')
const importLoading = ref(false)
const importResult = ref(null)

const handleExport = async () => {
  exportLoading.value = true
  try {
    const res = await exportData(exportInclude.value)
    const blob = new Blob([res.data], { type: 'application/zip' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `polyant-export-${new Date().toISOString().slice(0, 10)}.zip`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (error) {
    if (error.response?.status === 401) {
      sessionStorage.removeItem('admin_token')
      window.location.href = '/admin/login'
    }
    ElMessage.error('导出失败: ' + (error.message || error))
  } finally {
    exportLoading.value = false
  }
}

const handleFileChange = (file) => {
  importFile.value = file.raw
}

const handleImport = async () => {
  if (!importFile.value) return
  importLoading.value = true
  importResult.value = null
  try {
    const result = await importData(importFile.value, conflict.value)
    importResult.value = result
    ElMessage.success('导入完成')
  } catch (error) {
    ElMessage.error('导入失败: ' + (error.message || error))
  } finally {
    importLoading.value = false
  }
}
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
</style>
```
**Note:** `el-upload`'s `on-change` gives `{ raw }` (the File). `importData` sends it as FormData. The `importResult` shape matches `ImportResult{Success, Summary{entries_imported,...}, Errors[]}` (camelCase JSON from the model). Confirm the JSON field names match `ImportSummary`'s tags (`internal/core/export/importer.go:31-37` — `json:"entries_imported"` etc.); if they're snake_case, the template's `importResult.summary?.entries_imported` already matches.

- [ ] **Step 3: Register the route**

In `web/admin/src/router/index.js`, add a child (after `reviews`):
```js
        {
          path: 'data',
          name: 'Data',
          component: () => import('@/views/data/Index.vue'),
          meta: { permission: 4, title: '导入导出' }
        },
```

- [ ] **Step 4: Add the sidebar item**

In `web/admin/src/components/Sidebar.vue`, add after the reviews item:
```vue
    <el-menu-item index="/data" v-if="hasPermission(4)">
      <el-icon><Download /></el-icon>
      <span>导入导出</span>
    </el-menu-item>
```
And update the icon import (~line 27): add `Download` to the `@element-plus/icons-vue` import.

- [ ] **Step 5: Build + sync dist + verify**

```bash
cd web/admin && npm run build && cd -
rm -rf internal/api/admin/dist && cp -r web/admin/dist internal/api/admin/dist
```
Then the canonical Go verification block (SPA embedded). Commit:
```bash
git add web/admin/src/ internal/api/admin/dist/
git commit -m "feat(data-ops): admin SPA export/import page (/data)

Export (checkboxes + ZIP blob download via raw axios) + import (el-upload +
conflict strategy + ImportResult display). New /data route + sidebar item.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Final verification (after both tasks)

- [ ] `gofmt -l` repo-wide empty; `go build ./cmd/... ./internal/... ./pkg/...` OK; `go vet ./...` OK.
- [ ] `go test -race -count=1 ./cmd/... ./internal/... ./pkg/...` all PASS.
- [ ] `golangci-lint run ./...` exit 0.
- [ ] `cd web/admin && npm run build` succeeds; `internal/api/admin/dist/index.html` exists.
- [ ] Manual smoke (optional): admin SPA `/data` → export (checkboxes → ZIP downloads) → import (upload the ZIP → ImportResult shown).
- [ ] Branch `r4d-export-import-ui` has 3 task commits + spec/plan, ready for review/merge.
