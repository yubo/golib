package proc

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/spf13/pflag"
	"github.com/yubo/golib/configer"
	"k8s.io/klog/v2"
)

type ConfigOps struct {
	fs     *pflag.FlagSet
	group  string
	path   string
	sample interface{}
	opts   []configer.ConfigFieldsOption
}

type HookFn func(context.Context) error

type HookOps struct {
	Hook        HookFn
	Owner       string
	HookNum     ProcessAction
	Priority    uint16
	SubPriority uint16
	Data        interface{}

	priority ProcessPriority
	process  *Process
}

func (p HookOps) SetContext(ctx context.Context) {
	p.process.ctx = ctx
}

func (p HookOps) Context() context.Context {
	return p.process.ctx
}

func (p HookOps) Configer() configer.ParsedConfiger {
	return p.process.parsedConfiger
}

func (p HookOps) ContextAndConfiger() (context.Context, configer.ParsedConfiger) {
	return p.Context(), p.Configer()
}

func (p HookOps) dlog() {
	if klog.V(5).Enabled() {
		klog.InfoSDepth(1, "dispatch hook",
			"hookName", p.HookNum.String(),
			"owner", p.Owner,
			"priority", p.priority.String(),
			"nameOfFunction", nameOfFunction(p.Hook))
	}
}

type ProcessPriority uint32

const (
	_                 uint16 = iota << 3
	PRI_SYS_INIT             // init & register each system.module
	PRI_SYS_PRESTART         // prepare each system.module's depend
	PRI_MODULE               // init each module
	PRI_SYS_START            // start each system.module
	PRI_SYS_POSTSTART        // no use
)

func (p ProcessPriority) String() string {
	return fmt.Sprintf("0x%08x", uint32(p))
}

type ProcessAction uint32

const (
	ACTION_START ProcessAction = iota
	ACTION_RELOAD
	ACTION_STOP
	ACTION_SIZE
)

type ProcessStatus uint32

const (
	STATUS_INIT ProcessStatus = iota
	STATUS_PENDING
	STATUS_RUNNING
	STATUS_RELOADING
	STATUS_EXIT
)

func (p *ProcessStatus) Set(v ProcessStatus) {
	atomic.StoreUint32((*uint32)(p), uint32(STATUS_RUNNING))
}

func (p ProcessAction) String() string {
	switch p {
	case ACTION_START:
		return "start"
	case ACTION_RELOAD:
		return "reload"
	case ACTION_STOP:
		return "stop"
	default:
		return "unknown"
	}

}
