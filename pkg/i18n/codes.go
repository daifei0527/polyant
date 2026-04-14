// pkg/i18n/codes.go
package i18n

// 消息码命名规范: <模块>.<子模块>.<动作/状态>

// 通用消息码
const (
	CodeSuccess       = "common.success"
	CodeInvalidParams = "common.invalid_params"
	CodeInternalError = "common.internal_error"
	CodeNotFound      = "common.not_found"
)

// API 条目相关消息码
const (
	CodeEntryCreated    = "api.entry.created"
	CodeEntryUpdated    = "api.entry.updated"
	CodeEntryDeleted    = "api.entry.deleted"
	CodeEntryNotFound   = "api.entry.not_found"
	CodeEntryListLoaded = "api.entry.list_loaded"
)

// API 用户相关消息码
const (
	CodeUserRegistered  = "api.user.registered"
	CodeUserNotFound    = "api.user.not_found"
	CodeUserUpdated     = "api.user.updated"
	CodeUserInfoLoaded  = "api.user.info_loaded"
)

// API 认证相关消息码
const (
	CodeAuthMissing       = "api.auth.missing"
	CodeAuthInvalidSig    = "api.auth.invalid_signature"
	CodeAuthExpired       = "api.auth.expired"
	CodeAuthNoPermission  = "api.auth.permission_denied"
	CodeAuthUserSuspended = "api.auth.user_suspended"
)

// API 分类相关消息码
const (
	CodeCategoryCreated  = "api.category.created"
	CodeCategoryNotFound = "api.category.not_found"
	CodeCategoryList     = "api.category.list_loaded"
)

// API 搜索相关消息码
const (
	CodeSearchSuccess    = "api.search.success"
	CodeSearchKeywordShort = "api.search.keyword_too_short"
)

// CLI 消息码
const (
	CodeCLIEntryListTitle = "cli.entry.list_title"
	CodeCLIEntryNoResult  = "cli.entry.no_result"
	CodeCLIConfigSaved    = "cli.config.saved"
	CodeCLIKeyGenerated   = "cli.key.generated"
	CodeCLIUserListTitle  = "cli.user.list_title"
	CodeCLIServerStarted  = "cli.server.started"
	CodeCLIServerStopped  = "cli.server.stopped"
)

// 日志消息码
const (
	CodeLogServerStarted  = "log.server.started"
	CodeLogServerStopped  = "log.server.stopped"
	CodeLogDBConnected    = "log.db.connected"
	CodeLogDBError        = "log.db.error"
	CodeLogP2PConnected   = "log.p2p.connected"
	CodeLogP2PError       = "log.p2p.error"
	CodeLogSyncStarted    = "log.sync.started"
	CodeLogSyncCompleted  = "log.sync.completed"
	CodeLogSyncError      = "log.sync.error"
	CodeLogRequestIn      = "log.request.incoming"
	CodeLogRequestOut     = "log.request.outgoing"
)
