# Polyant R1 安全加固设计

**日期**: 2026-07-03
**版本**: v2.3.0 → R1
**范围**: 4 轮"分阶段全扫荡"迭代的第一轮——安全加固（Tier 1）
**状态**: 设计已与用户确认，待 spec 评审

---

## 1. 背景

2026-07-03 对 Polyant v2.3.0 做了新一轮全面审计（4 个并行子代理 + controller 自验），共发现 ~50 个缺陷 + ~13 个功能候选。健康基线全绿：`build`/`vet`/`test -race ./...` 通过，0 TODO，31k 行代码 + 35k 行测试。

审计揭示两层"已实现但有严重漏洞"的问题：

- **可远程利用的安全洞**：管理员会话接管、P2P 写入零验签、存储型 XSS、SMTP 头注入、占位 API key、公开路由无 body 限制、限流形同虚设。
- **正确性/数据完整性 bug**（R2 范围）：数据竞争、镜像同步失效、版本号/时间戳错配、投票竞态等。

用户选择**分阶段全扫荡（4 轮）**推进：R1 安全 → R2 正确性 → R3 修坏功能 → R4 新功能。每轮独立走 design → plan → implement → verify。本 spec 只覆盖 **R1**。

### 1.1 R1 根因核实（controller 亲自读码确认）

- **管理员接管**：`internal/api/admin/session.go:33-86` `CreateSessionHandler` 仅查用户存在性（`:60`），**无等级门控、无私钥持有证明**，对任意 pubkey 签发 admin 会话；`isLocalRequest`（`:92-112`）主判 `RemoteAddr` loopback（正确），但**回退到客户端可控的 `Host` 头**（`:103-110`）。远程攻击者发 `Host: 127.0.0.1:18531` + 已知 admin 公钥（公钥本就是公开的）即获完整 admin。
- **SMTP 注入**：`internal/api/handler/user_handler.go:479-481` `isValidEmail` 仅 `strings.Contains(@)` + `(.)`；`internal/core/email/service.go:75-81` 用 `fmt.Sprintf("To: %s\r\n", …)` 原样拼头，无 CRLF 净化；`:99-107` 声明 `Content-Transfer-Encoding: base64` 却追加明文 `TextBody/HTMLBody`（同时是正确性 bug）。
- **User 模型**：`internal/storage/model/models.go` 有 `UserLevel`(Lv0-5)，**无 `PasswordHash`**（R1 新增）。

### 1.2 已确认的三个设计分叉

| 分叉 | 用户决策 |
|------|----------|
| ① 管理员远程认证模型 | **混合**：API/CLI 走 Ed25519 签名请求（现有 `authMW.RequirePermission`，已安全）；Web admin 给 Lv5 用户加 bcrypt 密码登录 |
| ② P2P 推送验签上线 | **软上线 + 开关**：新建条目强制签名；接收方"有签必验、错签必拒、无签暂收记审计"；config 开关默认 false |
| ③ 公开阅读前端去留 | **移除** `web/static/js/app.js`（失效 + XSS） |

---

## 2. R1 范围与目标

**目标**：闭合全部 Tier 1 可远程利用漏洞，使限流真正生效，且不破坏现有合法流量与历史数据。

**非目标（R2-R4 范围，本 spec 不做）**：数据竞争/镜像同步/版本号时间戳/投票竞态等正确性 bug（R2）；admin SPA 刷新/统计/条目审核等坏功能（R3）；backup-restore/MCP 扩展/选举 UI 等新功能（R4）。

---

## 3. 详细设计

### 3.1 R1.1 管理员认证（混合模型）

**修复 `isLocalRequest`**（`internal/api/admin/session.go`）：
- 删除 Host 头回退（`:102-110`）。仅保留 `net.SplitHostPort(r.RemoteAddr)` 的 loopback 判定（`127.0.0.1`/`::1`）。Host 头永不信任。

**新增密码登录端点**（取代被废弃的 `/admin/session/create` 公钥呈交路径）：
- `POST /api/v1/admin/session/login`，body `{ "identifier": "<public_key 或 email>", "password": "..." }`：
  1. 查 user（按 pubkey 或经 `user-email:` 索引按 email）。
  2. `bcrypt.CompareHashAndPassword(user.PasswordHash, password)`，失败→401。
  3. 等级门控：`user.UserLevel < Lv4` → 403。
  4. `sessionMgr.CreateSession(pubkey)` 签发 24h token。
  5. 严格限流（见 R1.7：per-IP 5 次/分钟 + 失败计数）。
- 保留 `POST /api/v1/admin/session/create` **仅作 localhost CLI 快路径**：`isLocalRequest`（修后）+ 仍需等级门控 Lv4+。移除"任意 pubkey 即签发"逻辑。

**新增 token 自检端点**（R3 SPA 刷新也复用）：
- `GET /api/v1/admin/session`：Bearer token 校验通过→返回 `{public_key, agent_name, user_level}`；否则 401。

**User 模型扩展**（`internal/storage/model/models.go`）：
- 加 `PasswordHash string`（bcrypt）。零值 = 未设密码。所有 `UserStore` 实现（Badger/Memory/JSONFile/Pebble）自动兼容（字段序列化）。
- 注册/迁移：现有用户 `PasswordHash` 为空，只能用 Ed25519 CLI；设密码后可用 Web 登录。

**密码设置流程**（CLI，Ed25519 签名授权）：
- `pactl admin set-password --pubkey <pubkey> --password <pwd>`：Ed25519 签名请求，服务端 `RequirePermission(PermManageUser)`，bcrypt 哈希后写入 `User.PasswordHash`。
- `pactl admin change-password`：同上 + 可选要求旧密码。
- 首次 bootstrap：seed 节点首次启动若**无任何 Lv5 用户**，日志打印一次性 bootstrap 提示（引导用 CLI 设密码），不自动创建弱默认密码。

**Web admin UI**（`web/admin/src/views/Login.vue` + `api/session.js`）：
- 登录表单从"呈交 pubkey"改为"标识 + 密码"。调用 `/admin/session/login`。token 存 sessionStorage（与现状一致）。

**审计**：登录成功/失败、set-password 均记审计日志（`admin.login`/`admin.login_failed`/`admin.password_set`）。

### 3.2 R1.2 P2P 推送内容验签（软上线）

**签名原语**（`pkg/crypto`，复用 `ComputeContentHash`）：
- `SignContent(privKey ed25519.PrivateKey, title, content, category string) ([]byte, error)` = `ed25519.Sign(priv, ComputeContentHash(title, content, category))`。
- `VerifyContent(pubKey ed25519.PublicKey, sig []byte, title, content, category string) bool`。
- Rating 同理：签名内容 = `SHA256(entryID + "\n" + rater + "\n" + score)`（具体以 `RatingCalculator` 现有约定为准，实现时核实）。

**条目模型**：`KnowledgeEntry.Signature`/`SignatureAlgorithm` 字段已存在（CLAUDE.md 已述），无需改模型。

**生成端（客户端）**：
- `pkg/polysdk`：`CreateEntry` / `UpdateEntry` 用客户端私钥计算 `SignContent`，随请求体提交。
- `cmd/pactl`：同上。
- 服务端 `CreateEntryHandler` / `UpdateEntryHandler`（`internal/api/handler/entry_handler.go`）：
  - **新建/更新强制签名**：校验 `VerifyContent(entry.CreatedBy 公钥, entry.Signature, …)`；未签名或错签→401/403（记审计 `security.forged_entry`）。
  - 只有原作者（或 moderator）可更新——更新时重签并校验签名者 == `CreatedBy` 或有权者。

**接收端（P2P，`internal/network/sync/sync.go` `HandlePushEntry`/`HandleRatingPush`）**：
- 若 `entry.Signature` 非空 → `VerifyContent`，失败→**拒绝**并记审计 `security.forged_entry`。
- 若 `entry.Signature` 为空（历史未签名条目）→ **接受**，记审计 `security.unsigned_entry`（仅当 `config.P2P.RequireEntrySignatures == false`）。
- 若 `RequireEntrySignatures == true` → 无签一律拒绝。

**config**：`Network`/`P2P` 段加 `RequireEntrySignatures bool`（默认 false）。

**兼容性**：历史未签名条目在默认配置下继续同步；开关允许全新部署直接硬要求。

### 3.3 R1.3 + R1.9 存储型 XSS / 公开前端移除

- 删除 `web/static/js/app.js` 及加载它的模板/静态路由（核实 `web/templates/index.html`、`internal/api/router` 静态处理器、`web/static/css` 中仅为该 app 服务的部分）。
- 保留 `web/landing/`（营销页，独立）。
- Admin SPA（Vue）模板默认转义；实现时 grep `v-html`，若存在则改用 `textContent` 或 DOMPurify（预期 admin 无 v-html，但需确认）。
- 移除后该路径 404 或重定向至 landing。

### 3.4 R1.4 SMTP 头注入 + base64 修复

- `isValidEmail`（`user_handler.go:479`）改用 `net/mail.ParseAddress(email)`；并显式拒绝含 `\r` 或 `\n` 的输入。所有进入邮件流程的 email 字段（注册、验证）统一过该校验。
- `email/service.go`：
  - `From`/`To`/`Subject` 头值做 CRLF 净化（剥离 `\r`/`\n`）；`FromName` 用 `mime.QEncoding.Encode("utf-8", name)`。
  - 修 base64：声明 `base64` 时实际 `base64.StdEncoding.EncodeString(body)`（同时闭合 storage 审计 M7）。
- `email.To` 列表每项均校验。

### 3.5 R1.5 API key 加固

- `pkg/config`：`Network.ApiKey` 绑定 env `POLYANT_NETWORK_API_KEY`（与现有 `POLYANT_` 前缀一致）。
- `cmd/seed`/`cmd/user main.go`：配置加载后，若 `ApiKey == "sk_live_YOUR_API_KEY_HERE"`（已知占位符）→ **拒启**并提示设 env。空值允许（=禁用 ApiKeyMiddleware，现状行为）。
- `internal/api/middleware/apikey.go:25`：`subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) != 1`。

### 3.6 R1.6 公开路由 body 大小限制

- 新增 `internal/api/middleware/bodylimit.go`：`BodyLimitMiddleware(maxBytes int64)`，用 `http.MaxBytesReader` 包 `r.Body`。
- 在全局 mux 最外层装配（所有路由，公开+认证），默认 1MB，config `BodyLimit` 可调。
- 现有 `AuthMiddleware` 内的 1MB 限制保留（纵深防御），但全局层先兜底。

### 3.7 R1.7 限流重做

`internal/api/middleware/ratelimit.go` + `router.go`：

1. **可信代理 XFF**：`RateLimitConfig` 加 `TrustedProxies []string`（CIDR/IP）。`getLimitKey`：
   - 若 `RemoteAddr` 命中 TrustedProxies → 取 `X-Forwarded-For` **最左一跳**；
   - 否则 → 用 `net.SplitHostPort(RemoteAddr)` 的 host，**永不信任裸 XFF**。
2. **per-user 限流真生效**：在 `AuthMiddleware` 内（认证后）追加一道限流，key = `user.PublicKey[:16]`。当前 `getLimitKey` 的 user 分支因限流在 auth 之前而是死代码——通过这第二道解决。
3. **修数学错误**：明确 `Rate` 语义 = **每秒补充 token 数**（tokens/sec）。据此校正默认值达成文档意图：**读 60/min（DefaultRate=1/s，burst=10）、写 20/min（WriteRate≈0.33/s 或独立配置 WriteBurst）**。`Allow` 公式保持 `newTokens = float64(l.rate) * elapsed`（rate 已是/秒，语义自洽）。字段名/注释写清"tokens per second"，删除"60 请求/分钟 = 1 请求/秒"这类误导注释。验收标准：连续打读接口 >60 次/分钟即被限。
4. **write 分类按 method**：`isWritePath` 改为按 `r.Method ∈ {POST,PUT,PATCH,DELETE}` 判定（替代易漏的路径白名单）。
5. **OPTIONS 豁免 + CORS 前置**：CORS 中间件移到限流之前；或限流中间件首行 `if r.Method == OPTIONS { next; return }`。预检不计入配额。

### 3.8 R1.8 验证码防暴破

- `internal/core/email/verification.go`：`VerificationManager` 加 per-email 失败计数 + 锁定（默认 5 次失败→锁定 15min）。复用现有 `codeStore`（`vcode:` 前缀 KV）持久化计数。
- `/api/v1/user/verify-email` 端点纳入限流（write 类）。
- 锁定期间 `/send-verification` 对该 email 也拒绝（防止刷码）。

### 3.9 R1.x CI 加固（附带快赢）

- `.github/workflows/ci.yml`：`actions/setup-go` 的 `go-version` 从 `1.22` 升到 `1.25.x`（对齐 `go.mod 1.25.7`）。
- 加 `govulncheck ./...` 步骤（go1.25 自带 `go tool govulncheck` 或独立 action）。
- golangci-lint 已在跑，补一个 `.golangci.yml` 显式启用 `gosec`/`staticcheck`/`govet`/`unused`。

---

## 4. 数据与配置变更汇总

**模型**（`internal/storage/model/models.go`）：
- `User.PasswordHash string`（新增，零值兼容）。

**config**（`pkg/config`，对应 JSON/YAML + env）：
- `Network.ApiKey` + env `POLYANT_NETWORK_API_KEY`
- `P2P.RequireEntrySignatures bool`（默认 false）
- `RateLimit.TrustedProxies []string`
- `RateLimit.*` 数值校正（读/写速率）
- `BodyLimit`（默认 1048576）

**新端点**：
- `POST /api/v1/admin/session/login`（密码登录）
- `GET /api/v1/admin/session`（token 自检）
- `POST /api/v1/admin/session/create` 保留为 localhost-only + Lv4 门控（语义收窄）

**移除**：
- `web/static/js/app.js` 及其模板/路由。

---

## 5. 错误处理约定

- 安全失败一律返回**通用** 401/403，不泄露内部原因（不回显"用户不存在"vs"密码错"，统一"凭证无效"）。
- 伪造签名拒绝：对调用方返回通用错误，详细记审计。
- 占位 API key / 配置错误：启动期 fail-fast，日志给明确修复指引。

---

## 6. 测试策略

每修一验（systematic-debugging 纪律），`go test -race ./...` 保持全绿。

| 项 | 关键测试 |
|----|----------|
| R1.1 | Host 头伪造→拒；错密码→拒；非 Lv4→拒；暴破触发限流；`pactl set-password` 往返；`GET /admin/session` token 自检 |
| R1.2 | 伪造签名→拒；合法签名→收；无签→收+审计；`RequireEntrySignatures=true`→拒无签；扩展 `mocknet_e2e_test` 携带/伪造签名 |
| R1.4 | 含 `\r\n` email→拒；非法 email→拒；合法→通过；base64 正文可被邮件客户端正确解码 |
| R1.5 | constant-time 比较；占位符→拒启；env 覆盖生效 |
| R1.6 | 超 1MB body→413；正常 body→通过 |
| R1.7 | 不受信 XFF→用 RemoteAddr；可信代理→取首跳；per-user 限流 post-auth 触发；POST→write 分类；OPTIONS→豁免；速率数值符合文档 |
| R1.8 | N 次失败→锁定；锁定期内拒绝；过期恢复 |

回归：全量 `go test -race ./cmd/... ./internal/... ./pkg/...` + 既有 mocknet/export/level_checker 测试不回归。

---

## 7. 兼容性与回滚

- 全部新行为**开关化**：`RequireEntrySignatures`(默认false)、密码可选、限流可配、TrustedProxies 默认空（=纯 RemoteAddr，最安全）。
- 历史未签名条目在默认配置下继续同步，零数据迁移。
- 未设密码的 admin 暂仍可用 Ed25519 CLI（`/admin/session/create` localhost 快路径 + `authMW` 签名路由均保留）。
- 回滚：关闭相关 config 开关即恢复旧行为（签名软上线可关、密码登录可不用）。

---

## 8. R1 工作量预估与拆分（供 writing-plans 细化）

按依赖与风险分组（每组一个可独立提交的 plan 单元）：

- **R1-A（认证）**：3.1 admin 混合认证 + 3.5 API key 加固（同属访问控制，一起改、一起测）。
- **R1-B（验签）**：3.2 P2P 推送验签（独立，改动跨 polysdk/handler/sync/crypto）。
- **R1-C（输入卫生）**：3.4 SMTP 注入 + 3.6 body 限制 + 3.8 验证码防暴破（输入侧加固）。
- **R1-D（限流）**：3.7 限流重做（独立中间件改造）。
- **R1-E（前端/CI）**：3.3+3.9 移除公开前端 + CI 加固（低风险清理）。

---

## 9. 后续轮次（仅备忘，各自再开 cycle）

- **R2 正确性**：versionVec 加锁、镜像同步接收端、Update 双重 Version++、时间戳单位统一、bleve 重启核对、投票原子化、候选人 UpdateStatus 加锁、DHT seed nodes dial、goroutine 生命周期、Scan 错误传播。
- **R3 修坏功能**：admin SPA 刷新、条目审核实现、统计端点、UI 对齐、panic code:0、CORS、审计明文 body。
- **R4 新功能**：KV backup/restore + GC 调度、admin 选举/导出导入 UI、MCP 工具扩展，候选池含 gossipsub/circuit relay/peer 信誉/密钥轮换/审计轮转/keyset 分页。
