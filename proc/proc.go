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
	"github.com/yubo/golib/proc/config"
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
	options       *Options
	status        ProcessStatus
	hookOps       [ACTION_SIZE]HookOpsBucket
	namedFlagSets cliflag.NamedFlagSets
	configer      *config.Configer
	wg            sync.WaitGroup
	ctx           context.Context
}

func RegisterHooks(in []HookOps) error {
	for i, _ := range in {
		v := &in[i]
		v.module = _module
		v.priority = ProcessPriority(uint32(v.Priority)<<16 + uint32(v.SubPriority))

		_module.hookOps[v.HookNum] = append(_module.hookOps[v.HookNum], v)
	}
	return nil
}

type addFlags interface {
	AddFlags(fs *pflag.FlagSet)
}

func RegisterFlags(name string, in addFlags) error {
	in.AddFlags(_module.namedFlagSets.FlagSet(name))
	return nil
}

func NamedFlagSets() *cliflag.NamedFlagSets {
	return &_module.namedFlagSets
}

// procInit
// alloc configer
// parse configfile
// validate config each module
// sort hook options
func (p *Module) procInit() (err error) {
	ctx := p.ctx
	opts, _ := ConfigOptsFrom(ctx)

	if p.configer, err = config.NewConfiger(p.options.configFile, opts...); err != nil {
		return err
	}

	if err = p.configer.Prepare(); err != nil {
		return err
	}

	p.status = STATUS_PENDING

	for i := ACTION_START; i < ACTION_SIZE; i++ {
		sort.Sort(p.hookOps[i])
	}

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
	case ACTION_TEST:
		return "test"
	default:
		return "unknown"
	}
}

func logOps(ops *HookOps) {
	klog.V(5).Infof("hook %s %s[%d.%d] %s",
		hookNumName(ops.HookNum),
		ops.Owner,
		ops.Priority,
		ops.SubPriority,
		nameOfFunction(ops.Hook))
}

// only be called once
func (p *Module) procStart() error {
	for _, ops := range p.hookOps[ACTION_START] {
		logOps(ops)
		if err := ops.Hook(ops); err != nil {
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
		if err := ops.Hook(ops); err != nil {
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

func (p *Module) procTest() error {
	for _, ops := range p.hookOps[ACTION_TEST] {
		logOps(ops)
		if err := ops.Hook(ops); err != nil {
			return fmt.Errorf("%s.%s() err: %s", ops.Owner, nameOfFunction(ops.Hook), err)
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
		logOps(ops)
		if err := ops.Hook(ops); err != nil {
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
}

// for general startCmd
func (p *Module) testConfig() error {
	if err := p.procInit(); err != nil {
		klog.Error(err)
		return err
	}

	klog.V(3).Infof("#### %s\n", p.options.configFile)
	klog.V(3).Infof("%s\n", p.configer)

	if err := p.procTest(); err != nil {
		return err
	}

	fmt.Printf("%s: configuration file %s test is successful\n",
		os.Args[0], p.options.configFile)
	return nil
}

func (p *Module) start() error {
	if err := p.procInit(); err != nil {
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
