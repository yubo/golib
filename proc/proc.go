package proc

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yubo/golib/configer"
	"k8s.io/klog/v2"
)

const (
	serverGracefulCloseTimeout = 12 * time.Second
)

const (
	moduleName = "proc"
)

type Module struct {
	name     string
	status   ProcessStatus
	hookOps  [ACTION_SIZE]HookOpsBucket
	configer *configer.Configer
	options  Options
	config   string
	test     bool
}

var (
	_module = &Module{name: moduleName}
)

func RegisterHooks(in []HookOps) error {
	for i, _ := range in {
		v := &in[i]
		v.module = _module

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
		v.module = _module
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
func (p *Module) procInit(configFile string) (cf *configer.Configer, err error) {
	if cf, err = configer.NewConfiger(configFile); err != nil {
		return nil, err
	}

	if err = cf.Prepare(); err != nil {
		return nil, err
	}

	p.status = STATUS_PENDING

	for i := ACTION_START; i < ACTION_SIZE; i++ {
		sort.Sort(p.hookOps[i])
	}

	p.configer = cf
	return
}

func hookNumName(n ProcessAction) string {
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

func nameOfFunction(f interface{}) string {
	fun := runtime.FuncForPC(reflect.ValueOf(f).Pointer())
	tokenized := strings.Split(fun.Name(), ".")
	last := tokenized[len(tokenized)-1]
	last = strings.TrimSuffix(last, ")·fm") // < Go 1.5
	last = strings.TrimSuffix(last, ")-fm") // Go 1.5
	last = strings.TrimSuffix(last, "·fm")  // < Go 1.5
	last = strings.TrimSuffix(last, "-fm")  // Go 1.5
	return last

}

func dbgOps(ops *HookOps) {
	klog.V(5).Infof("hook %s %s[%d] %s",
		hookNumName(ops.HookNum),
		ops.Owner,
		ops.Priority,
		nameOfFunction(ops.Hook))
}

// only be called once
func (p *Module) procStart() error {
	for _, ops := range p.hookOps[ACTION_START] {
		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
}

// reverse order
func (p *Module) procStop() (err error) {
	wgCh := make(chan struct{})
	wg := p.options.Wg()
	go func() {
		wg.Wait()
		wgCh <- struct{}{}
	}()

	ss := p.hookOps[ACTION_STOP]
	for i := len(ss) - 1; i >= 0; i-- {
		ops := ss[i]

		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	p.status.Set(STATUS_EXIT)

	// Wait then close or hard close.
	closeTimeout := serverGracefulCloseTimeout
	select {
	case <-wgCh:
		klog.Info("server closed")
	case <-time.After(closeTimeout):
		err = fmt.Errorf("server closed after timeout %ds", closeTimeout/time.Second)

	}
	return err
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
	p.status.Set(STATUS_RELOADING)

	if err := p.configer.Prepare(); err != nil {
		return err
	}

	for _, ops := range p.hookOps[ACTION_RELOAD] {
		dbgOps(ops)
		if err := ops.Hook(ops, p.configer); err != nil {
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)
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

func init() {
	for i := ACTION_START; i < ACTION_SIZE; i++ {
		_module.hookOps[i] = HookOpsBucket([]*HookOps{})
	}
}
