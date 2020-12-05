package tracing

import (
	"context"
	"fmt"

	xopentracing "github.com/m3db/m3/src/x/opentracing"
	"github.com/opentracing/opentracing-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
	"go.uber.org/zap"
)

const (
	moduleName = "sys.tracing"
)

type Config struct {
	xopentracing.TracingConfiguration `yaml:",inline"`
	HttpBody                          bool `yaml:"httpBody"`
	HttpHeader                        bool `yaml:"httpHeader"`
	RespTraceId                       bool `yaml:"respTraceId"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

func (p *Config) Validate() error {
	return nil
}

type Module struct {
	*Config
	name   string
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.test,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_MODULE,
	}}
)

func (p *Module) test(ops *proc.HookOps, configer *configer.Configer) error {
	c := &Config{}
	if err := configer.ReadYaml(p.name, c); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}

	return nil
}

func (p *Module) start(ops *proc.HookOps, configer *configer.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	popts := ops.Options()

	c := &Config{}
	if jc, err := jaegercfg.FromEnv(); err != nil {
		c.TracingConfiguration.Jaeger = *jc
	}
	if err := configer.ReadYaml(p.name, c); err != nil {
		return err
	}
	p.Config = c
	// klog.Infof("config %s", c)

	if c.TracingConfiguration.Backend == "" {
		return
	}

	serviceName := popts.Name()
	scope := popts.MetricsScope().SubScope("tracing")
	logger := popts.Logger()
	tracer, traceCloser, err := p.Config.TracingConfiguration.NewTracer(serviceName, scope, logger)
	if err != nil {
		tracer = opentracing.NoopTracer{}
		logger.Warn("could not initialize tracing; using no-op tracer instead",
			zap.String("service", serviceName), zap.Error(err))
	}
	go func() {
		<-p.ctx.Done()
		traceCloser.Close()
	}()

	// add tracer filter
	http := popts.Http()
	http.Filter(p.filter)

	popts = popts.SetTracer(tracer)
	logger.Info("tracing enabled", zap.String("service", serviceName))
	opentracing.SetGlobalTracer(tracer)

	ops.SetOptions(popts)
	return nil
}

func (p *Module) stop(ops *proc.HookOps, configer *configer.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
