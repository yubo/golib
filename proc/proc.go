package proc

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"time"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/spf13/pflag"
	"github.com/yubo/golib/configer"
	cliflag "github.com/yubo/golib/staging/cli/flag"
	"k8s.io/klog/v2"
)

const (
	serverGracefulCloseTimeout = 12 * time.Second
	moduleName                 = "proc"
)

var (
	_module *Module
)

type Module struct {
	name          string
	status        ProcessStatus
	hookOps       [ACTION_SIZE]HookOpsBucket
	namedFlagSets cliflag.NamedFlagSets
	configer      *configer.Configer
	wg            sync.WaitGroup
	ctx           context.Context
	//options       *Options
}

func RegisterHooks(in []HookOps) error {
	for i, _ := range in {
		v := &in[i]
		v.module = _module
		v.priority = ProcessPriority(uint32(v.Priority)<<(16-3) + uint32(v.SubPriority))

		_module.hookOps[v.HookNum] = append(_module.hookOps[v.HookNum], v)
	}
	return nil
}

type addFlags interface {
	AddFlags(fs *pflag.FlagSet)
}

//func RegisterFlags(name string, in addFlags) error {
//	in.AddFlags(_module.namedFlagSets.FlagSet(name))
//	return nil
//}

func NamedFlagSets() *cliflag.NamedFlagSets {
	return &_module.namedFlagSets
}

// init
// alloc configer
// parse configfile
// validate config each module
// sort hook options
func (p *Module) init() (err error) {
	ctx := p.ctx

	opts, _ := ConfigOptsFrom(ctx)
	if p.configer, err = configer.New(opts...); err != nil {
		return err
	}

	p.status = STATUS_PENDING

	for i := ACTION_START; i < ACTION_SIZE; i++ {
		sort.Sort(p.hookOps[i])
	}

	ctx = WithAttr(ctx, make(map[interface{}]interface{}))
	ctx = WithWg(ctx, &p.wg)
	ctx = WithConfiger(ctx, p.configer)
	p.ctx = ctx

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
	default:
		return "unknown"
	}
}

func logOps(ops *HookOps) {
	if klog.V(5).Enabled() {
		klog.InfoSDepth(1, "dispatch hook",
			"hookName", hookNumName(ops.HookNum),
			"owner", ops.Owner,
			"priority", fmt.Sprintf("0x%08x", ops.priority),
			"nameOfFunction", nameOfFunction(ops.Hook))
	}
}

// only be called once
func (p *Module) procStart() error {
	for _, ops := range p.hookOps[ACTION_START] {
		logOps(ops)

		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			return fmt.Errorf("%s.%s() err: %s", ops.Owner, nameOfFunction(ops.Hook), err)
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
}

// reverse order
func (p *Module) procStop() (err error) {
	wgCh := make(chan struct{})

	go func() {
		p.wg.Wait()
		wgCh <- struct{}{}
	}()

	ss := p.hookOps[ACTION_STOP]
	for i := len(ss) - 1; i >= 0; i-- {
		ops := ss[i]

		logOps(ops)
		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			return fmt.Errorf("%s.%s() err: %s", ops.Owner, nameOfFunction(ops.Hook), err)
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

func (p *Module) procReload() (err error) {
	p.status.Set(STATUS_RELOADING)

	opts, _ := ConfigOptsFrom(p.ctx)
	if p.configer, err = configer.New(opts...); err != nil {
		return err
	}

	for _, ops := range p.hookOps[ACTION_RELOAD] {
		logOps(ops)
		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
}

func (p *Module) start() error {
	if err := p.init(); err != nil {
		return err
	}

	if err := p.procStart(); err != nil {
		return err
	}

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, append(shutdownSignals, reloadSignals...)...)

	if _, err := systemd.SdNotify(true, "READY=1\n"); err != nil {
		klog.Errorf("Unable to send systemd daemon successful start message: %v\n", err)
	}

	shutdown := false
	shutdownResult := make(chan error, 1)

	for {
		select {
		case s := <-sigs:
			if sigContains(s, shutdownSignals) {
				if shutdown {
					os.Exit(1)
				}
				shutdown = true
				go func() {
					shutdownResult <- p.procStop()
				}()
			} else if sigContains(s, reloadSignals) {
				if err := p.procReload(); err != nil {
					return err
				}
			}
		case err := <-shutdownResult:
			return err
		}
	}
}

func init() {
	hookOps := [ACTION_SIZE]HookOpsBucket{}
	for i := ACTION_START; i < ACTION_SIZE; i++ {
		hookOps[i] = HookOpsBucket([]*HookOps{})
	}

	_module = &Module{
		hookOps: hookOps,
	}
}
