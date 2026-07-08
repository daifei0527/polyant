# R4d 导出/导入 UI 设计

**范围**：R4 第四个迷你轮——admin SPA 的数据导出/导入界面。R4a/R4b/R4c 已合入 master（`a20904d`）。导出/导入后端已完整（`Exporter`/`Importer` + 2 端点），但挂在 Ed25519 `authMW` 后，session-token admin SPA 够不到。本轮把两端点迁移到 session-token `adminAuthMW` 并加 SPA UI。

**轮次定位**：R4d 只做导出/导入的 admin 可达性 + UI，不动 Exporter/Importer 业务逻辑，不做选举 UI（R4e）。

## 目标

让 admin 在后台 SPA 完成数据导出/导入：

- **导出**：复选框选包含项（条目/分类/用户/评分）→ 下载 ZIP。
- **导入**：上传 ZIP + 选冲突策略（跳过/覆盖/合并）→ 显示 `ImportResult`（成功/汇总/错误）。
- 端点迁移到 session-token，SPA 可达。

## 非目标

- 选举 UI（R4e 单独迷你轮）。
- 备份 UI（R4c 已在 stats 页面板）。
- 导入进度条 / dry-run 预览（MVP 同步处理）。
- Exporter/Importer 业务改动（复用现成）。
- 增量导出 / 远程拉取（全量 ZIP）。

## 现状核实（代码 grounded）

| 能力 | 现状 | 位置 |
|---|---|---|
| `Exporter.Export(ctx, opts)` | 完整，返回 ZIP bytes，opts IncludeEntries/Categories/Users/Ratings | `export/exporter.go:60` |
| `Importer.Import(zipData, opts)` | 完整，返回 `*ImportResult`，opts ConflictStrategy + OperatorLevel | `export/importer.go:75` |
| 导出/导入 handler | 完整（流 ZIP / multipart） | `handler/export_handler.go:33,72` |
| 端点 `/api/v1/admin/export|import` | Ed25519 `authMW` + PermManageUser（Lv4+），SPA 够不到 | `router.go:405,408` |
| 生产调用方 | **无**（仅 build-ignored `scripts/test_*.go` 用 Ed25519 调） | scripts/ |
| Import 的 OperatorLevel | 现从 Ed25519 context 的 user 取 | `export_handler.go:115` |
| admin session context | 仅注入 `mw.PublicKeyKey`（无 level） | `admin/middleware.go:48` |
| SPA | 无导出/导入 API client、无 UI | `web/admin/src/` |

## 架构：迁移到 session-token

**方案：迁移现有端点**（推荐，详见决策）。`/api/v1/admin/export` + `/api/v1/admin/import` 从 `registerAuthRoutes`（Ed25519 authMW + PermManageUser）移到 `registerAdminRoutes`（session-token adminAuthMW）。无生产调用方（仅 build-ignored 脚本），迁移零业务风险。

被否方案：① 新增 `/api/v1/admin/data/*` 并行路径（两路径一功能，smell）；② 增强 admin session 携带 level（YAGNI，pubkey 查 UserStore 足够）。

**Import OperatorLevel 解法**：迁移后 handler 从 `r.Context().Value(mw.PublicKeyKey)` 拿 admin publicKey → `UserStore.Get` 查真实等级 → 作为 OperatorLevel。Lv4 admin 不能导入 Lv5 用户（Importer 现有安全检查 `importer.go:197` 继续生效）。

## 组件

### 1. 路由迁移（`internal/api/router/router.go`）

- 从 `registerAuthRoutes` 删除 `/api/v1/admin/export` + `/api/v1/admin/import` 的注册（及其 PermManageUser 包裹）。
- 在 `registerAdminRoutes` 新增（`adminAuthMW` session-token）：
  - `GET /api/v1/admin/export` → `adminHandler.ExportHandler`
  - `POST /api/v1/admin/import` → `adminHandler.ImportHandler`
- 复用现有 handler（挂 admin.Handler 委托，照搬 R4b review/R4c backup 模式）。

### 2. Handler 适配（`internal/api/handler/export_handler.go`）

- **Export handler**：不变（不需操作者身份）。
- **Import handler**：OperatorLevel 改为从 session 取——`adminPubKey, _ := r.Context().Value(mw.PublicKeyKey).(string)` → `h.userStore.Get(ctx, adminPubKey)` → `user.UserLevel`。若查不到 admin 用户，返回 401/403。其余 multipart 解析 + Importer 调用不变。
- handler 需访问 `UserStore`（确认构造时已注入；若无，加）。

### 3. SPA `api/data.js`（新）

```js
import axios from 'axios'
import request from './request'

const token = () => sessionStorage.getItem('admin_token')

// 导出：二进制 ZIP，绕过 request.js 的 JSON envelope 拦截器
export function exportData(include) {
  return axios.get('/api/v1/admin/export', {
    params: { include: include.join(',') },
    responseType: 'blob',
    headers: { Authorization: `Bearer ${token()}` }
  })
}

// 导入：multipart
export function importData(file, conflict) {
  const form = new FormData()
  form.append('file', file)
  form.append('conflict', conflict)
  return request.post('/admin/import', form)
}
```

### 4. SPA `views/data/Index.vue`（新）

- **导出面板**（el-card）：4 复选框（条目/分类/用户/评分，默认条目+分类）+ "导出 ZIP" 按钮。点击 → `exportData(include)` → blob → `window.URL.createObjectURL` + `<a download>` 触发下载。
- **导入面板**（el-card）：`el-upload`（拖拽/选择 ZIP，单文件）+ 冲突策略 `el-radio-group`（跳过/覆盖/合并）+ "导入" 按钮。点击 → `importData(file, conflict)` → 显示 `ImportResult`（success tag + summary 表 + errors 列表）。
- 照搬现有 el-card/el-table/ElMessage 模式。

### 5. 路由 + 侧栏

- `web/admin/src/router/index.js`：加 `{ path: 'data', name: 'Data', component: () => import('@/views/data/Index.vue'), meta: { permission: 4, title: '导入导出' } }`。
- `web/admin/src/components/Sidebar.vue`：加 `<el-menu-item index="/data" v-if="hasPermission(4)">` + 图标 import。

### 6. 脚本（build-ignored）

`scripts/test_admin.go` / `test_full.go` / `test_import.go` 的 Ed25519 export/import 调用迁移后失效。标注 stale（注释说明端点已迁 session-token）——不删（保留作历史参考），不影响 build/test（`//go:build ignore`）。

## 数据流

- **导出**：SPA 复选框 → `GET /api/v1/admin/export?include=...`（Bearer）→ `Exporter.Export` → ZIP 流（`Content-Type: application/zip`）→ SPA blob 下载。
- **导入**：SPA 上传 + 策略 → `POST /api/v1/admin/import`（multipart, Bearer）→ handler 查 admin 等级 → `Importer.Import(zipData, {Conflict, OperatorLevel})` → `ImportResult` → SPA 渲染。

## 测试

- **后端**（`export_handler_test.go` 扩展）：
  - Export handler 返回 ZIP（Content-Type `application/zip`，body 非空）。
  - Import handler 用 session-token context（设 `mw.PublicKeyKey`）+ mock/内存 UserStore 返回 Lv4 用户 → OperatorLevel=4；`ImportResult.Success` 正确；导入 Lv5 用户被拒（OperatorLevel 门控）。
  - 401 无 token。
- **SPA**：导出按钮触发下载（mock axios blob）；导入上传调端点（mock）；结果渲染。
- **路由**：`/api/v1/admin/export|import` 在 adminAuthMW 下（401 无 token），不再在 authMW 下。

## 接口变化

- 端点认证模型变更：`/api/v1/admin/export|import` Ed25519 → session-token（**破坏性**对 Ed25519 调用方，但无生产调用方）。
- 新增 SPA `/data` 路由 + 菜单。
- 无 Exporter/Importer/模型改动。

## 风险与回退

- **blob 下载绕过拦截器**：export 是二进制，request.js 响应拦截器会按 JSON envelope 解析失败。`api/data.js` 用独立 `axios` 调（不复用 `request`）+ `responseType:'blob'` + 手动 Bearer。已纳入设计。
- **认证迁移破坏性**：仅 build-ignored 脚本受影响（非生产）。若误判有隐藏调用方，回退 = 把端点移回 registerAuthRoutes（单 commit revert）。
- **大文件导入同步阻塞**：100MB 上限（现有 `export_handler.go` body 限制）。MVP 同步；大库可能慢，不阻塞（admin 操作）。
- 每个 task 独立 commit + 验证。

## 出范围跟踪

- 选举管理 UI（R4e）。
- 导入进度条 / dry-run。
- 增量 / 远程导出。
