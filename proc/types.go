package proc

import (
	"context"
	"sync/atomic"

	"github.com/yubo/golib/configer"
)

type HookFn func(ops *HookOps) error

type HookOps struct {
	Hook        HookFn
	Owner       string
	HookNum     ProcessAction
	Priority    uint16
	SubPriority uint16
	Data        interface{}

	priority ProcessPriority
	module   *Module
}

type HookOpsBucket []*HookOps

func (p HookOpsBucket) Len() int {
	return len(p)
}

func (p HookOpsBucket) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p HookOpsBucket) Less(i, j int) bool {
	return p[i].priority < p[j].priority
}

func (p HookOps) SetContext(ctx context.Context) {
	p.module.ctx = ctx
}

func (p HookOps) Context() context.Context {
	return p.module.ctx
}

func (p HookOps) Configer() *configer.Configer {
	return ConfigerFrom(p.module.ctx)
}

func (p HookOps) ContextAndConfiger() (context.Context, *configer.Configer) {
	return p.Context(), p.Configer()
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

type ProcessAction uint32

const (
	ACTION_START ProcessAction = iota
	ACTION_RELOAD
	ACTION_STOP
	ACTION_TEST
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
