package proc

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/cobra"
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
	DefaultProcess = NewProcess()
)

type Process struct {
	*ProcessOptions

	debugConfig bool // print config after proc.init()
	debugFlags  bool // print flags after proc.init()
	dryrun      bool // will exit after proc.init()

	initDone      bool
	sigsCh        chan os.Signal
	hookOps       [ACTION_SIZE][]*HookOps
	namedFlagSets flag.NamedFlagSets
	status        ProcessStatus
	err           error
}

func newProcess() *Process {
	hookOps := [ACTION_SIZE][]*HookOps{}
	for i := ACTION_START; i < ACTION_SIZE; i++ {
		hookOps[i] = []*HookOps{}
	}

	return &Process{
		hookOps:        hookOps,
		sigsCh:         make(chan os.Signal, 2),
		ProcessOptions: newProcessOptions(),
	}

}

func NewProcess(opts ...ProcessOption) *Process {
	p := newProcess()

	for _, opt := range opts {
		opt(p.ProcessOptions)
	}

	return p
}

// for test
func Reset() {
	DefaultProcess = NewProcess()
}

func Start(cmd *cobra.Command) error {
	return DefaultProcess.Start(cmd)
}

func Init(cmd *cobra.Command) error {
	return DefaultProcess.init(cmd)
}

func Shutdown() error {
	DefaultProcess.sigsCh <- shutdownSignal
	return nil
}

func PrintConfig(w io.Writer) {
	DefaultProcess.PrintConfig(w)
}

func PrintFlags(fs *pflag.FlagSet, w io.Writer) {
	DefaultProcess.PrintFlags(fs, w)
}

func AddFlags(f *pflag.FlagSet) {
	DefaultProcess.AddFlags(f)
}

func Name() string {
	return DefaultProcess.Name()
}

func Description() string {
	return DefaultProcess.Description()
}

// RegisterHooks register hookOps as a module
func RegisterHooks(in []HookOps) error {
	for i := range in {
		v := &in[i]
		v.process = DefaultProcess
		v.priority = ProcessPriority(uint32(v.Priority)<<(16-3) + uint32(v.SubPriority))

		DefaultProcess.hookOps[v.HookNum] = append(DefaultProcess.hookOps[v.HookNum], v)
	}
	return nil
}

func NamedFlagSets() *flag.NamedFlagSets {
	return &DefaultProcess.namedFlagSets
}

func (p *Process) Start(cmd *cobra.Command) error {
	if err := p.init(cmd); err != nil {
		return err
	}

	if p.debugConfig {
		p.PrintConfig(os.Stdout)
	}
	if p.debugFlags {
		p.PrintFlags(cmd.Flags(), os.Stdout)
	}
	if p.dryrun {
		//return errors.New("dryrun")
		//pflag.Parse()
		return nil
	}

	if err := p.start(); err != nil {
		return err
	}

	if p.noloop {
		return p.stop()
	}

	return p.loop()
}

// init
// alloc configer
// parse configfile
// validate config each module
// sort hook options
func (p *Process) init(cmd *cobra.Command) error {
	if p.initDone {
		return nil
	}

	ctx := p.ctx
	go func() {
		c := ctx
		<-c.Done()
	}()

	if _, ok := AttrFrom(ctx); !ok {
		ctx = WithAttr(ctx, make(map[interface{}]interface{}))
		go func() {
			c := ctx
			<-c.Done()
		}()
	}

	// configer
	if _, ok := ConfigerFrom(ctx); !ok {
		configer, err := configer.NewConfiger(
			append(p.configerOptions,
				configer.WithFlagSet(cmd.Flags()),
			)...)
		if err != nil {
			return err
		}
		WithConfiger(ctx, configer)
	}

	if _, ok := WgFrom(ctx); !ok {
		WithWg(ctx, p.wg)
	}

	p.status = STATUS_PENDING

	for i := ACTION_START; i < ACTION_SIZE; i++ {
		x := p.hookOps[i]
		sort.SliceStable(x, func(i, j int) bool { return x[i].priority < x[j].priority })
	}

	p.ctx = ctx
	p.initDone = true

	return nil
}

// only be called once
func (p *Process) start() error {
	p.wg.Add(1)
	defer p.wg.Done()

	for _, ops := range p.hookOps[ACTION_START] {
		ops.dlog()

		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			return fmt.Errorf("%s.%s() err: %s", ops.Owner, nameOfFunction(ops.Hook), err)
		}
	}
	p.status.Set(STATUS_RUNNING)
	return nil
}

func (p *Process) loop() error {
	signal.Notify(p.sigsCh, append(shutdownSignals, reloadSignals...)...)

	if _, err := daemon.SdNotify(true, "READY=1\n"); err != nil {
		klog.Errorf("Unable to send systemd daemon successful start message: %v\n", err)
	}

	shutdown := false
	for {
		select {
		case <-p.ctx.Done():
			return p.err
		case s := <-p.sigsCh:
			if sigContains(s, shutdownSignals) {
				klog.Infof("recv shutdown signal, exiting")
				if shutdown {
					klog.Infof("recv shutdown signal, force exiting")
					os.Exit(1)
				}
				shutdown = true
				go func() {
					p.stop()
				}()
			} else if sigContains(s, reloadSignals) {
				if err := p.reload(); err != nil {
					return err
				}
			}
		}
	}
}

func (p *Process) shutdown() error {
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return proc.Signal(shutdownSignal)
}

// reverse order
func (p *Process) stop() error {
	select {
	case <-p.ctx.Done():
		return nil
	default:
	}

	wgCh := make(chan struct{})

	go func() {
		p.wg.Wait()
		wgCh <- struct{}{}
	}()

	stopHooks := p.hookOps[ACTION_STOP]
	for i := len(stopHooks) - 1; i >= 0; i-- {
		stop := stopHooks[i]

		stop.dlog()
		if err := stop.Hook(WithHookOps(p.ctx, stop)); err != nil {
			p.err = fmt.Errorf("%s.%s() err: %s", stop.Owner, nameOfFunction(stop.Hook), err)

			return p.err
		}
	}
	p.status.Set(STATUS_EXIT)

	// Wait then close or hard close.
	closeTimeout := serverGracefulCloseTimeout
	select {
	case <-wgCh:
		klog.Info("See ya!")
	case <-time.After(closeTimeout):
		p.err = fmt.Errorf("%s closed after timeout %s", p.name, closeTimeout.String())

	}

	p.cancel()

	return p.err
}

func (p *Process) reload() (err error) {
	p.status.Set(STATUS_RELOADING)

	configer, err := configer.NewConfiger()
	if err != nil {
		p.err = err
		return err
	}

	WithConfiger(p.ctx, configer)

	for _, ops := range p.hookOps[ACTION_RELOAD] {
		ops.dlog()
		if err := ops.Hook(WithHookOps(p.ctx, ops)); err != nil {
			p.err = err
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)

	p.err = nil
	return nil
}

func (p *Process) PrintConfig(out io.Writer) {
	if c, _ := ConfigerFrom(p.ctx); c != nil {
		out.Write([]byte(c.String()))
	}
}

func (p *Process) PrintFlags(fs *pflag.FlagSet, w io.Writer) {
	flag.PrintFlags(fs, os.Stdout)
}

func (p *Process) AddFlags(f *pflag.FlagSet) {
	f.BoolVar(&p.debugConfig, "debug-config", p.debugConfig, "print config")
	f.BoolVar(&p.debugFlags, "debug-flags", p.debugFlags, "print flags")
	f.BoolVar(&p.dryrun, "dry-run", p.debugFlags, "exit before proc.Start()")
}

func (p *Process) Name() string {
	return p.name
}
func (p *Process) Description() string {
	return p.description
}
