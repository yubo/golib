package openapi

import (
	"net/http"

	"github.com/yubo/golib/status"
)

const (
	MODULE_SSO_BASE = 1 << 6 << iota
	MODULE_GATHER_BASE
)

// for req context
const (
	RshDataKey   = "rshData"
	RshConnKey   = "rshConn"
	ReqEntityKey = "reqEntity"
)

const (
	_               = 8000 + 10*iota
	MODULE_SSO_PORT // 8010
)

const (

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

type RespStatus struct {
	Code int    `json:"code" description:"status code"`
	Err  string `json:"err" description:"error msg"`
}

type RespTotal struct {
	Total int64 `json:"total" description:"total number"`
}

type RespID struct {
	ID int64 `json:"id" description:"id"`
}

func NewRespStatus(err error) RespStatus {
	ret := RespStatus{
		Code: 200,
	}
	if err != nil {
		ret.Err = err.Error()
		ret.Code = http.StatusBadRequest

		if s, ok := status.FromError(err); ok {
			ret.Code = status.HTTPStatusFromCode(s.Code())
		}
	}

	return ret
}

func NewRespID(id int64) RespID {
	return RespID{ID: id}
}

func NewRespTotal(total int64) RespTotal {
	return RespTotal{Total: total}
}
