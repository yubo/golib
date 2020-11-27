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
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.test,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_MODULE,
	}, {
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_MODULE,
	}}
)

func (p *Module) test(ops *proc.HookOps, configer *proc.Configer) error {
	cf := &metrics.Config{}
	if err := configer.ReadYaml(p.name, cf); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	klog.V(3).Infof("%s", cf)

	return nil
}

// TODO: should after http server register
func (p *Module) preStart(ops *proc.HookOps, configer *proc.Configer) (err error) {
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

func (p *Module) start(ops *proc.HookOps, configer *proc.Configer) error {
	popts := ops.Options()

	mux := popts.Http()

	p.metrics.Start(p.ctx, mux)
	return nil
}

func (p *Module) stop(ops *proc.HookOps, configer *proc.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
