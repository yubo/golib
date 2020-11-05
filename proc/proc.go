package proc

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"sync/atomic"

	"k8s.io/klog/v2"
)

// type {{{
type HookFn func(ops *HookOps, cf *Configer) error

type HookOps struct {
	Hook     HookFn
	Owner    string
	HookNum  int
	Priority int
	Data     interface{}
}

type HookOpsBucket []*HookOps

func (p HookOpsBucket) Len() int {
	return len(p)
}

func (p HookOpsBucket) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p HookOpsBucket) Less(i, j int) bool {
	return p[i].Priority < p[j].Priority
}

func (p HookOps) SetOptions(opts Options) {
	_module.options = opts
}

func (p HookOps) Options() Options {
	return _module.options
}

// }}}

// const {{{

const (
	ACTION_START = iota
	ACTION_RELOAD
	ACTION_STOP
	ACTION_TEST
	ACTION_SIZE
)
const (
	STATUS_INIT = iota
	STATUS_PENDING
	STATUS_RUNNING
	STATUS_RELOADING
	STATUS_EXIT
)

const (
	_ = iota
	PRI_PRE_SYS
	PRI_PRE_MODULE
	PRI_MODULE
	PRI_POST_MODULE
	PRI_SYS
	PRI_POST_SYS
)

// }}}

const (
	moduleName = "proc"
)

type Module struct {
	name     string
	status   uint32
	hookOps  [ACTION_SIZE]HookOpsBucket
	configer *Configer
	options  Options
	config   string
	test     bool
}

var (
	_module = &Module{
		name: moduleName,
	}
)

func init() {
	for i := 0; i < ACTION_SIZE; i++ {
		_module.hookOps[i] = HookOpsBucket([]*HookOps{})
	}
}

func RegisterHooks(in []HookOps) error {
	for i, _ := range in {
		v := &in[i]
		if v.HookNum < 0 || v.HookNum >= ACTION_SIZE {
			return fmt.Errorf("invalid HookNum %d", v.HookNum)
		}
		_module.hookOps[v.HookNum] = append(_module.hookOps[v.HookNum], v)
	}
	return nil
}

func RegisterHooksWithOptions(in []HookOps, opts Options) error {
	if _module.options != nil {
		return errAlreadySetted
	}
	for i, _ := range in {
		v := &in[i]
		if v.HookNum < 0 || v.HookNum >= ACTION_SIZE {
			return fmt.Errorf("invalid HookNum %d [0,%d]", v.HookNum, ACTION_SIZE)
		}
		_module.hookOps[v.HookNum] = append(_module.hookOps[v.HookNum], v)
	}
	_module.options = opts
	return nil
}

// procInit
// alloc configer
// parse configfile
// validate config each module
// sort hook options
func (p *Module) procInit(configFile string) (cf *Configer, err error) {
	if cf, err = NewConfiger(configFile); err != nil {
		return nil, err
	}

	if err = cf.Prepare(); err != nil {
		return nil, err
	}

	p.status = STATUS_PENDING

	for i := 0; i < ACTION_SIZE; i++ {
		sort.Sort(p.hookOps[i])
	}

	p.configer = cf
	return
}

func hookNumName(n int) string {
	switch n {
	case ACTION_START:
		return "start"
	case ACTION_RELOAD:
		return "reload"
	case ACTION_STOP:
		return "stop"
	case ACTION_TEST:
		return "test"
	default:
		return "unknown"
	}
}

func dbgOps(ops *HookOps) {
	klog.V(5).Infof("hook %s %s[%d] %s()",
		hookNumName(ops.HookNum),
		ops.Owner,
		ops.Priority,
		runtime.FuncForPC(reflect.ValueOf(ops.Hook).
			Pointer()).Name())
}

// only be called once
func (p *Module) procStart() error {
	for _, ops := range p.hookOps[ACTION_START] {
		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	atomic.StoreUint32(&p.status, STATUS_RUNNING)
	return nil
}

// reverse order
func (p *Module) procStop() error {
	ss := p.hookOps[ACTION_STOP]
	for i := len(ss) - 1; i >= 0; i-- {
		ops := ss[i]

		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	atomic.StoreUint32(&p.status, STATUS_EXIT)
	return nil
}

func (p *Module) procTest() error {
	for _, ops := range p.hookOps[ACTION_TEST] {
		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	return nil
}

func (p *Module) procReload() error {
	atomic.StoreUint32(&p.status, STATUS_RELOADING)

	if err := p.configer.Prepare(); err != nil {
		return err
	}

	for _, ops := range p.hookOps[ACTION_RELOAD] {
		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	atomic.StoreUint32(&p.status, STATUS_RUNNING)
	return nil
}

func envOr(name string, defs ...string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	for _, def := range defs {
		if def != "" {
			return def
		}
	}
	return ""
}

func getenvBool(str string) bool {
	b, _ := strconv.ParseBool(os.Getenv(str))
	return b
}
