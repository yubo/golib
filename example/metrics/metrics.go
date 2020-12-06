// this is a sample metric scope module
// https://github.com/uber-go/tally/blob/master/statsd/example/statsd_main.go
package metrics

import (
	"context"
	"math/rand"
	"time"

	"github.com/uber-go/tally"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/proc"
)

const (
	moduleName = "metrics"
)

type Module struct {
	Name   string
	scope  tally.Scope
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	_module = &Module{Name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_SYS,
	}}
)

func (p *Module) start(ops *proc.HookOps, cf *configer.Configer) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	popts := ops.Options()
	scope := popts.MetricsScope().SubScope("stats")

	counter := scope.Counter("test-counter")
	gauge := scope.Gauge("test-gauge")
	timer := scope.Timer("test-timer")
	histogram := scope.Histogram("test-histogram", tally.DefaultBuckets)

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}
			counter.Inc(1)
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}
			gauge.Update(rand.Float64() * 1000)
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}
			tsw := timer.Start()
			hsw := histogram.Start()
			time.Sleep(time.Duration(rand.Float64() * float64(time.Second)))
			tsw.Stop()
			hsw.Stop()
		}
	}()

	return nil
}

func (p *Module) stop(ops *proc.HookOps, cf *configer.Configer) error {
	p.cancel()
	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
