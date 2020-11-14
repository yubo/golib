package metrics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/m3db/prometheus_client_golang/prometheus/promhttp"
	"github.com/uber-go/tally"
	"k8s.io/klog"
)

const (
	defaultSamplingRate      = 1.0
	defaultReportingInterval = time.Second
)

type Metrics struct {
	*Config
	Scope  tally.Scope
	closer io.Closer
}

func NewMetrics(cf *Config) (*Metrics, error) {
	scope, closer, err := cf.NewRootScope()
	if err != nil {
		return nil, err
	}

	return &Metrics{
		Config: cf,
		Scope:  scope,
		closer: closer,
	}, nil
}

type httpMux interface {
	Handle(pattern string, handler http.Handler)
}

func (p *Metrics) Start(ctx context.Context, mux httpMux) error {
	if reporter := p.prometheusReporter; reporter != nil {
		c := p.PrometheusReporter
		configOpts := p.prometheusConfigurationOptions
		path := "/metrics"

		if handlerPath := strings.TrimSpace(c.HandlerPath); handlerPath != "" {
			path = handlerPath
		}

		handler := reporter.HTTPHandler()
		if configOpts.Registry != nil {
			gatherer := newMultiGatherer(configOpts.Registry, configOpts.ExternalRegistries)
			handler = promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
		}

		addr := strings.TrimSpace(c.ListenAddress)
		if addr == "" && configOpts.HandlerListener == nil {
			// If address not specified and server not specified, register
			// on default mux.
			mux.Handle(path, handler)
		} else {
			mux := http.NewServeMux()
			mux.Handle(path, handler)

			listener := configOpts.HandlerListener
			if listener == nil {
				// Address must be specified if server was nil.
				var err error
				listener, err = net.Listen("tcp", addr)
				if err != nil {
					return fmt.Errorf(
						"prometheus handler listen address error: %v", err)
				}
			}

			server := &http.Server{Handler: mux}
			go func() {
				if err := server.Serve(listener); err != nil {
					klog.Error(err)
				}
			}()

			go func() {
				<-ctx.Done()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
				defer cancel()
				err := server.Shutdown(ctx)
				klog.V(5).Infof("prometheus httpServer %s exit %v", addr, err)
			}()

		}

	}

	go func() {
		<-ctx.Done()

		if p.closer != nil {
			p.closer.Close()
		}
	}()

	return nil
}

// Reporter reports metrics about a component.
type Reporter interface {
	// Start starts the reporter.
	Start() error
	// Stop stops the reporter.
	Stop() error
}
