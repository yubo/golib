package proc

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/pflag"
	"github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/configer"
	"k8s.io/klog/v2"
)

const (
	serverGracefulCloseTimeout = 12 * time.Second
	moduleName                 = "proc"
)

var (
	proc *Process
)

type Process struct {
	name          string
	status        ProcessStatus
	hookOps       [ACTION_SIZE][]*HookOps
	namedFlagSets flag.NamedFlagSets
	//configer      *configer.Configer
	wg  sync.WaitGroup
	ctx context.Context
}

func RegisterHooks(in []HookOps) error {
	for i, _ := range in {
		v := &in[i]
		v.process = proc
		v.priority = ProcessPriority(uint32(v.Priority)<<(16-3) + uint32(v.SubPriority))

		proc.hookOps[v.HookNum] = append(proc.hookOps[v.HookNum], v)
	}
	return nil
}

type addFlags interface {
	AddFlags(fs *pflag.FlagSet)
}

func NamedFlagSets() *flag.NamedFlagSets {
	return &proc.namedFlagSets
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

func (p *Process) start() error {
	if err := p.init(); err != nil {
		return err
	}

	if err := p.procStart(); err != nil {
		return err
	}

	return p.loop()
}

// init
// alloc configer
// parse configfile
// validate config each module
// sort hook options
func (p *Process) init() error {
	ctx := p.ctx

	opts, _ := ConfigOptsFrom(ctx)
	configer, err := configer.New(opts...)
	if err != nil {
		return err
	}

	p.status = STATUS_PENDING

	for i := ACTION_START; i < ACTION_SIZE; i++ {
		x := p.hookOps[i]
		sort.Slice(x, func(i, j int) bool { return x[i].priority < x[j].priority })
	}

	ctx = WithAttr(ctx, make(map[interface{}]interface{}))
	WithWg(ctx, &p.wg)
	WithConfiger(ctx, configer)
	p.ctx = ctx

	return nil
}

// only be called once
func (p *Process) procStart() error {
	for _, ops := range p.hookOps[ACTION_START] {
		logOps(ops)

		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			return fmt.Errorf("%s.%s() err: %s", ops.Owner, nameOfFunction(ops.Hook), err)
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
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

func (p *Process) loop() error {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, append(shutdownSignals, reloadSignals...)...)

	if _, err := daemon.SdNotify(true, "READY=1\n"); err != nil {
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

// reverse order
func (p *Process) procStop() (err error) {
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

func (p *Process) procReload() (err error) {
	p.status.Set(STATUS_RELOADING)

	opts, _ := ConfigOptsFrom(p.ctx)
	configer, err := configer.New(opts...)
	if err != nil {
		return err
	}

	WithConfiger(p.ctx, configer)

	for _, ops := range p.hookOps[ACTION_RELOAD] {
		logOps(ops)
		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
}

func init() {
	hookOps := [ACTION_SIZE][]*HookOps{}
	for i := ACTION_START; i < ACTION_SIZE; i++ {
		hookOps[i] = []*HookOps{}
	}

	proc = &Process{hookOps: hookOps}
}
