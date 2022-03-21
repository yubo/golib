package proc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/cli/globalflag"
	"github.com/yubo/golib/configer"
	"k8s.io/klog/v2"
)

const (
	serverGracefulCloseTimeout = 12 * time.Second
	moduleName                 = "proc"
)

var (
	DefaultProcess = NewProcess()
	ErrDryrun      = errors.New("dry run")
)

type Process struct {
	*ProcessOptions

	configer       configer.Configer
	parsedConfiger configer.ParsedConfiger
	//fs             *pflag.FlagSet
	configOps     []*ConfigOps // catalog of RegisterConfig
	namedFlagSets flag.NamedFlagSets

	debugConfig bool // print config after proc.init()
	debugFlags  bool // print flags after proc.init()
	dryrun      bool // will exit after proc.init()

	sigsCh  chan os.Signal
	hookOps [ACTION_SIZE][]*HookOps // catalog of RegisterHooks
	status  ProcessStatus
	err     error
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
		configer:       configer.NewConfiger(),
	}

}

func NewProcess(opts ...ProcessOption) *Process {
	p := newProcess()

	for _, opt := range opts {
		opt(p.ProcessOptions)
	}

	return p
}

func Context() context.Context {
	return DefaultProcess.Context()
}

func Configer() configer.ParsedConfiger {
	return DefaultProcess.parsedConfiger
}

func NewRootCmd(opts ...ProcessOption) *cobra.Command {
	return DefaultProcess.NewRootCmd(opts...)
}

func Start(fs *pflag.FlagSet) error {
	return DefaultProcess.Start(fs)
}

func Init(cmd *cobra.Command, opts ...ProcessOption) error {
	DefaultProcess.Init(cmd, opts...)
	return nil
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

func NamedFlagSets() *flag.NamedFlagSets {
	return &DefaultProcess.namedFlagSets
}

func RegisterHooks(in []HookOps) error {
	return DefaultProcess.RegisterHooks(in)
}

func ConfigVar(fs *pflag.FlagSet, path string, sample interface{}, opts ...configer.ConfigFieldsOption) error {
	return DefaultProcess.ConfigVar(fs, path, sample, opts...)
}

func Parse(fs *pflag.FlagSet, opts ...configer.ConfigerOption) (configer.ParsedConfiger, error) {
	return DefaultProcess.Parse(fs, opts...)
}

// must invoke before cmd.Execute()/pflag.Parse()
//func AddRegisteredFlags(fs *pflag.FlagSet) error {
//	return DefaultProcess.AddRegisteredFlags(fs)
//}

func RegisterFlags(configPath, groupName string, sample interface{}, opts ...configer.ConfigFieldsOption) error {
	return DefaultProcess.RegisterFlags(configPath, groupName, sample, opts...)
}

func (p *Process) RegisterFlags(configPath, groupName string, sample interface{}, opts ...configer.ConfigFieldsOption) error {
	p.configOps = append(p.configOps, &ConfigOps{
		fs:     p.namedFlagSets.FlagSet(groupName),
		group:  groupName,
		path:   configPath,
		sample: sample,
		opts:   opts,
	})
	return nil
}

func (p *Process) BindRegisteredFlags(fs *pflag.FlagSet) error {
	for _, v := range p.configOps {
		//klog.InfoS("add Var", "path", v.path, "group", v.group, "type", reflect.TypeOf(v.sample).String())
		if err := p.configer.Var(v.fs, v.path, v.sample, v.opts...); err != nil {
			return err
		}
	}

	for _, f := range p.namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	return nil
}

func (p *Process) ConfigVar(fs *pflag.FlagSet, path string, sample interface{}, opts ...configer.ConfigFieldsOption) error {
	return p.configer.Var(fs, path, sample, opts...)
}

// RegisterHooks register hookOps as a module
func (p *Process) RegisterHooks(in []HookOps) error {
	for i := range in {
		v := &in[i]
		v.process = p
		v.priority = ProcessPriority(uint32(v.Priority)<<(16-3) + uint32(v.SubPriority))

		p.hookOps[v.HookNum] = append(p.hookOps[v.HookNum], v)
	}
	return nil
}

// with proc.Start
func (p *Process) NewRootCmd(opts ...ProcessOption) *cobra.Command {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	cmd := &cobra.Command{
		Use:          p.name,
		Short:        p.description,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := p.Start(cmd.Flags())
			if err == ErrDryrun {
				return nil
			}
			return err
		},
	}

	p.Init(cmd, opts...)

	return cmd
}

func (p *Process) Context() context.Context {
	return p.ctx
}

func (p *Process) Start(fs *pflag.FlagSet) error {
	if _, err := p.Parse(fs); err != nil {
		return err
	}

	if err := p.start(); err != nil {
		return err
	}

	if p.noloop {
		return p.stop()
	}

	return p.loop()
}

func (p *Process) Parse(fs *pflag.FlagSet, opts ...configer.ConfigerOption) (configer.ParsedConfiger, error) {
	klog.V(10).Infof("entering Parse")
	defer klog.V(10).Infof("leaving Parse")
	// parse configpositive
	if p.parsedConfiger == nil {
		opts = append(p.configerOptions, opts...)
		c, err := p.configer.Parse(opts...)
		if err != nil {
			return nil, err
		}
		p.parsedConfiger = c
	}

	if p.debugConfig {
		p.PrintConfig(os.Stdout)
	}
	if p.debugFlags {
		p.PrintFlags(fs, os.Stdout)
	}
	if p.dryrun {
		// ugly hack
		// Do not initialize any stateful objects before this
		return nil, ErrDryrun
	}

	return p.parsedConfiger, nil
}

// Init
// set configer options
// alloc p.ctx
// validate config each module
// sort hook options
func (p *Process) Init(cmd *cobra.Command, opts ...ProcessOption) error {
	for _, opt := range opts {
		opt(p.ProcessOptions)
	}

	if err := p.RegisterHooks(p.hooks); err != nil {
		return err
	}
	if c, ok := configer.ConfigerFrom(p.ctx); ok {
		p.parsedConfiger = c
	}
	if _, ok := AttrFrom(p.ctx); !ok {
		p.ctx = WithAttr(p.ctx, make(map[interface{}]interface{}))
	}
	if _, ok := WgFrom(p.ctx); !ok {
		WithWg(p.ctx, p.wg)
	}

	p.AddGlobalFlags()

	fs := cmd.PersistentFlags()
	fs.ParseErrorsWhitelist.UnknownFlags = true

	p.BindRegisteredFlags(fs)

	if p.group {
		setGroupCommandFunc(cmd, p.namedFlagSets)
	}

	return nil
}

// only be called once
func (p *Process) start() error {
	p.wg.Add(1)
	defer p.wg.Done()

	for i := ACTION_START; i < ACTION_SIZE; i++ {
		x := p.hookOps[i]
		sort.SliceStable(x, func(i, j int) bool { return x[i].priority < x[j].priority })
	}

	ctx := configer.WithConfiger(p.ctx, p.parsedConfiger)
	for _, ops := range p.hookOps[ACTION_START] {
		ops.dlog()
		if err := ops.Hook(WithHookOps(ctx, ops)); err != nil {
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
	ctx := configer.WithConfiger(p.ctx, p.parsedConfiger)
	for i := len(stopHooks) - 1; i >= 0; i-- {
		stop := stopHooks[i]

		stop.dlog()
		if err := stop.Hook(WithHookOps(ctx, stop)); err != nil {
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

	cf, err := p.configer.Parse(p.configerOptions...)
	if err != nil {
		p.err = err
		return err
	}

	p.parsedConfiger = cf

	ctx := configer.WithConfiger(p.ctx, p.parsedConfiger)
	for _, ops := range p.hookOps[ACTION_RELOAD] {
		ops.dlog()
		if err := ops.Hook(WithHookOps(ctx, ops)); err != nil {
			p.err = err
			return err
		}
	}
	p.status.Set(STATUS_RUNNING)

	p.err = nil
	return nil
}

func (p *Process) PrintConfig(out io.Writer) {
	out.Write([]byte(p.parsedConfiger.String()))
}

func (p *Process) PrintFlags(fs *pflag.FlagSet, w io.Writer) {
	flag.PrintFlags(fs, os.Stdout)
}

func (p *Process) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&p.debugConfig, "debug-config", p.debugConfig, "print config")
	fs.BoolVar(&p.debugFlags, "debug-flags", p.debugFlags, "print flags")
	fs.BoolVar(&p.dryrun, "dry-run", p.debugFlags, "exit before proc.Start()")
}

func (p *Process) AddGlobalFlags() {
	gfs := p.namedFlagSets.FlagSet("global")

	// add klog, logs, help flags
	globalflag.AddGlobalFlags(gfs)

	// add process flags
	p.AddFlags(gfs)

	p.configer.AddFlags(gfs)
}

func (p *Process) Name() string {
	return p.name
}

func (p *Process) Description() string {
	return p.description
}
