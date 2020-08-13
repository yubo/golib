package openapi

const (
	AuthTokenKey = "authToken"
	RshDataKey   = "rshData"
	RshConnKey   = "rshConn"
	ReqEntityKey = "reqEntity"

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
