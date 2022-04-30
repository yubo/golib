package proc

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yubo/golib/version"
	"k8s.io/klog/v2"
)

func (p *Process) reporterStart() error {
	return newBuildReporter(p.ctx, p.version).start()
}

func newBuildReporter(ctx context.Context, ver *version.Info) *buildReporter {
	return &buildReporter{ctx: ctx, version: ver}
}

type buildReporter struct {
	ctx       context.Context
	version   *version.Info
	buildTime time.Time
}

func (p *buildReporter) start() error {
	if p.version == nil {
		return InvalidVersionErr
	}

	buildTime, err := time.Parse(time.RFC3339, p.version.BuildDate)
	if err != nil {
		return err
	}
	p.buildTime = buildTime

	go p.report()
	return nil
}

func (p *buildReporter) report() {
	ver := p.version
	tags := map[string]string{
		"version":        ver.GitVersion,
		"commit":         ver.GitCommit,
		"build_date":     ver.BuildDate,
		"git_tree_state": ver.GitTreeState,
		"go_version":     ver.GoVersion,
	}

	buildInfoGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "build_information",
		ConstLabels: tags,
	})

	buildAgeGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "build_age",
		ConstLabels: tags,
	})

	buildInfoGauge.Set(1.0)
	buildAgeGauge.Set(float64(time.Since(p.buildTime)))

	ticker := time.NewTicker(time.Second * 10)

	for {
		select {
		case <-ticker.C:
			buildInfoGauge.Set(1.0)
			buildAgeGauge.Set(float64(time.Since(p.buildTime)))

		case <-p.ctx.Done():
			klog.V(1).InfoS("proc.reporter exit")
			return
		}
	}
}
