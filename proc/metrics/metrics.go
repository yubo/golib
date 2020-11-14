package metrics

import (
	"context"
	"fmt"

	"github.com/yubo/golib/metrics"
	"github.com/yubo/golib/proc"
	"k8s.io/klog/v2"
)

const (
	moduleName = "sys.metrics"
)

type Module struct {
	*metrics.Config
	name    string
	metrics *metrics.Metrics
	ctx     context.Context
	cancel  context.CancelFunc
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.stopHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)

func (p *Module) testHook(ops *proc.HookOps, configer *proc.Configer) error {
	cf := &metrics.Config{}
	if err := configer.ReadYaml(p.name, cf); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	klog.V(3).Infof("%s", cf)

	return nil
}

// TODO: should after http server register
func (p *Module) preStartHook(ops *proc.HookOps, configer *proc.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	popts := ops.Options()

	cf := &metrics.Config{}
	if err := configer.ReadYaml(p.name, cf); err != nil {
		return err
	}
	p.Config = cf

	if p.metrics, err = metrics.NewMetrics(cf); err != nil {
		return err
	}

	popts = popts.SetMetricsScope(p.metrics.Scope)
	ops.SetOptions(popts)

	return nil
}

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) error {
	popts := ops.Options()

	mux := popts.Get(proc.HttpServerName).(proc.HttpServer)

	p.metrics.Start(p.ctx, mux)
	return nil
}

func (p *Module) stopHook(ops *proc.HookOps, cf *proc.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
