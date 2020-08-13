package api

const (
	MODULE_SSO_BASE = 1 << 6 << iota
	MODULE_FKS_BASE
	MODULE_GATHER_BASE
	MODULE_ARMORY_BASE
	MODULE_TRAILER_BASE
)

// for req context
const (
	AuthTokenKey = "authToken"
	RshDataKey   = "rshData"
	RshConnKey   = "rshConn"
	ReqEntityKey = "reqEntity"
)

const (
	_               = 8000 + 10*iota
	MODULE_SSO_PORT // 8010
)

const (
	SecurityDefinitionKey = "OAPI_SECURITY_DEFINITION"
	OauthSecurityName     = "OAuth"

	// scope
	OauthScopeNil           = "nil"
	OauthScopeRead          = "read"
	OauthScopeWrite         = "write"
	OauthScopeExec          = "exec"
	OauthScopeWork          = "work"
	OauthScopeRoot          = "root"
	OauthScopeUpload        = "upload"
	OauthScopeOverwrite     = "overwrite"
	OauthScopeEdit          = "edit"
	OauthScopeAdmin         = "admin"
	OauthScopeReadSecret    = "read:secret"
	OauthScopeWriteSecret   = "write:secret"
	OauthScopeWriteRegistry = "write:registry"
	OauthScopeReadSso       = "read:sso"
	OauthScopeWriteSso      = "write:sso"

	// action
	ActionInstall  = "install"
	ActionUpgrade  = "upgrade"
	ActionDelete   = "delete"
	ActionRollback = "rollback"
	ActionInit     = "init"
	ActionCreate   = "create"
	ActionUpdate   = "update"
	ActionCopy     = "copy"
	ActionMove     = "move"
	ActionStart    = "start"
	ActionStop     = "stop"
	ActionRsh      = "rsh"
	ActionLogin    = "login"
	ActionExec     = "exec"
	ActionRun      = "run"
	ActionApply    = "apply"
	ActionRestore  = "restore"
	ActionReboot   = "reboot"
)
