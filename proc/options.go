// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package proc

import (
	"fmt"
	"os"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/session"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

const (
	defaultSamplingRate      = 1.0
	defaultReportingInterval = time.Second
)

type options struct {
	name       string
	testConfig string

	zap             *zap.Logger
	scope           tally.Scope
	tracer          opentracing.Tracer
	samplingRate    float64
	timerOptions    TimerOptions
	reportInterval  time.Duration
	customBuildTags map[string]string

	auth    Auth
	mail    Mail
	db      *orm.Db
	grpc    GrpcServer
	http    HttpServer
	audit   Audit
	session *session.Session

	extra map[string]interface{}
}

// NewOptions creates new instrument options.
func NewOptions() Options {
	zapLogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(
		zap.NewProductionEncoderConfig()),
		os.Stdout, zap.InfoLevel))

	return &options{
		zap:             zapLogger,
		scope:           tally.NoopScope,
		samplingRate:    defaultSamplingRate,
		reportInterval:  defaultReportingInterval,
		customBuildTags: map[string]string{},
		extra:           map[string]interface{}{},
	}
}

func (o *options) SetName(name string) Options {
	opts := *o
	opts.name = name
	return &opts
}

func (o *options) Name() string {
	return o.name
}

func (o *options) SetLogger(value *zap.Logger) Options {
	opts := *o
	opts.zap = value
	return &opts
}

func (o *options) Logger() *zap.Logger {
	return o.zap
}

func (o *options) SetMetricsScope(value tally.Scope) Options {
	opts := *o
	opts.scope = value
	return &opts
}

func (o *options) MetricsScope() tally.Scope {
	return o.scope
}

func (o *options) Tracer() opentracing.Tracer {
	return o.tracer
}

func (o *options) SetTracer(tracer opentracing.Tracer) Options {
	opts := *o
	opts.tracer = tracer
	return &opts
}

func (o *options) SetTimerOptions(value TimerOptions) Options {
	opts := *o
	opts.timerOptions = value
	return &opts
}

func (o *options) TimerOptions() TimerOptions {
	return o.timerOptions
}

func (o *options) SetReportInterval(value time.Duration) Options {
	opts := *o
	opts.reportInterval = value
	return &opts
}

func (o *options) ReportInterval() time.Duration {
	return o.reportInterval
}

func (o *options) SetCustomBuildTags(tags map[string]string) Options {
	opts := *o
	opts.customBuildTags = tags
	return &opts
}

func (o *options) CustomBuildTags() map[string]string {
	return o.customBuildTags
}

func (o *options) Auth() Auth {
	return o.auth
}

func (o *options) SetAuth(auth Auth) Options {
	opts := *o
	opts.auth = auth
	return &opts
}

func (o *options) Mail() Mail {
	return o.mail
}

func (o *options) SetMail(mail Mail) Options {
	opts := *o
	opts.mail = mail
	return &opts
}

func (o *options) Db() *orm.Db {
	return o.db
}

func (o *options) SetDb(db *orm.Db) Options {
	opts := *o
	opts.db = db
	return &opts
}

func (o *options) Grpc() GrpcServer {
	return o.grpc
}

func (o *options) SetGrpc(grpc GrpcServer) Options {
	opts := *o
	opts.grpc = grpc
	return &opts
}

func (o *options) Http() HttpServer {
	return o.http
}

func (o *options) SetHttp(http HttpServer) Options {
	opts := *o
	opts.http = http
	return &opts
}

func (o *options) Audit() Audit {
	return o.audit
}

func (o *options) SetAudit(audit Audit) Options {
	opts := *o
	opts.audit = audit
	return &opts
}
func (o *options) Session() *session.Session {
	return o.session
}

func (o *options) SetSession(session *session.Session) Options {
	opts := *o
	opts.session = session
	return &opts
}

func (o *options) Set(name string, data interface{}) Options {
	opts := *o
	extra := make(map[string]interface{})
	for k, v := range opts.extra {
		extra[k] = v
	}
	if _, ok := extra[name]; ok {
		klog.WarningDepth(1, fmt.Sprintf("options.Set(%s) override", name))
	}
	extra[name] = data
	opts.extra = extra
	if klog.V(3).Enabled() {
		klog.WarningDepth(1, fmt.Sprintf("options.Set(%s)", name))
	}
	return &opts
}

func (o *options) Get(name string) interface{} {
	if klog.V(3).Enabled() {
		klog.WarningDepth(1, fmt.Sprintf("options.Get(%s)", name))
	}
	return o.extra[name]
}
