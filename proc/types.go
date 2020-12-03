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

// Package instrument implements functions to make instrumenting code,
// including metrics and logging, easier.
package proc

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/opentracing/opentracing-go"
	"github.com/uber-go/tally"
	"github.com/yubo/golib/mail"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/session"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Options represents the options for instrumentation.
type Options interface {
	// SetName sets the process name
	SetName(name string) Options

	// return process name
	Name() string

	// SetLogger sets the zap logger
	SetLogger(value *zap.Logger) Options

	// ZapLogger returns the zap logger
	Logger() *zap.Logger

	// SetMetricsScope sets the metrics scope.
	SetMetricsScope(value tally.Scope) Options

	// MetricsScope returns the metrics scope.
	MetricsScope() tally.Scope

	// Tracer returns the tracer.
	Tracer() opentracing.Tracer

	// SetTracer sets the tracer.
	SetTracer(tracer opentracing.Tracer) Options

	// SetTimerOptions sets the metrics timer options to used
	// when building timers from timer options.
	SetTimerOptions(value TimerOptions) Options

	// TimerOptions returns the metrics timer options to used
	// when building timers from timer options.
	TimerOptions() TimerOptions

	// SetReportInterval sets the time between reporting metrics within the system.
	SetReportInterval(time.Duration) Options

	// ReportInterval returns the time between reporting metrics within the system.
	ReportInterval() time.Duration

	// SetCustomBuildTags sets custom tags to be added to build report metrics in
	// addition to the defaults.
	SetCustomBuildTags(tags map[string]string) Options

	// CustomBuildTags returns the custom build tags.
	CustomBuildTags() map[string]string

	Auth() Auth
	SetAuth(Auth) Options

	Mail() Mail
	SetMail(Mail) Options

	Db() *orm.Db
	SetDb(*orm.Db) Options

	Grpc() *grpc.Server
	SetGrpc(*grpc.Server) Options

	Http() HttpServer
	SetHttp(HttpServer) Options

	Audit() Audit
	SetAudit(Audit) Options

	Session() *session.Session
	SetSession(*session.Session) Options

	Wg() sync.WaitGroup

	// extra
	Set(name string, data interface{}) Options
	Get(name string) interface{}
}

// Reporter reports metrics about a component.
type Reporter interface {
	// Start starts the reporter.
	Start() error
	// Stop stops the reporter.
	Stop() error
}

type Client interface {
	GetId() string
	GetSecret() string
	GetRedirectUri() string
}

type Auth interface {
	GetFilter(acl string) (restful.FilterFunction, string, error)
	IsAdmin(token openapi.Token) bool
	SsoClient() Client

	Access(req *restful.Request, resp *restful.Response, mustLogin bool, chain *restful.FilterChain, handles ...func(openapi.Token) error)
	WsAccess(req *restful.Request, resp *restful.Response, mustLogin bool, chain *restful.FilterChain, handles ...func(openapi.Token) error)
	GetAndVerifyTokenInfoByApiKey(code *string, peerAddr string) (openapi.Token, error)
	GetAndVerifyTokenInfoByBearer(code *string) (openapi.Token, error)
}

type Mail interface {
	NewMail(tpl Executer, data interface{}) (*mail.MailContext, error)
	SendMail(subject, to []string, tpl Executer, data interface{}) error
}

type HttpServer interface {
	// http
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))

	// restful.Container
	Add(service *restful.WebService) *restful.Container
	Filter(filter restful.FilterFunction)
}

type Audit interface {
	Log(UserName, Target, Action, PeerAddr, Extra, Err string, CreatedAt int64) error
}

type Executer interface {
	Execute(wr io.Writer, data interface{}) error
}

type HookFn func(ops *HookOps, cf *Configer) error

type HookOps struct {
	Hook     HookFn
	Owner    string
	HookNum  ProcessAction
	Priority ProcessPriority
	Data     interface{}
	module   *Module
}

type HookOpsBucket []*HookOps

func (p HookOpsBucket) Len() int {
	return len(p)
}

func (p HookOpsBucket) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p HookOpsBucket) Less(i, j int) bool {
	return p[i].Priority < p[j].Priority
}

func (p HookOps) SetOptions(opts Options) {
	p.module.options = opts
}

func (p HookOps) Options() Options {
	return p.module.options
}

type ProcessPriority uint32

const (
	_ ProcessPriority = iota
	PRI_PRE_SYS
	PRI_PRE_MODULE
	PRI_MODULE
	PRI_POST_MODULE
	PRI_SYS
	PRI_POST_SYS
)

type ProcessAction uint32

const (
	ACTION_START ProcessAction = iota
	ACTION_RELOAD
	ACTION_STOP
	ACTION_TEST
	ACTION_SIZE
)

type ProcessStatus uint32

const (
	STATUS_INIT ProcessStatus = iota
	STATUS_PENDING
	STATUS_RUNNING
	STATUS_RELOADING
	STATUS_EXIT
)

func (p *ProcessStatus) Set(v ProcessStatus) {
	atomic.StoreUint32((*uint32)(p), uint32(STATUS_RUNNING))
}
