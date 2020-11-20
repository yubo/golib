package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/go-openapi/spec"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/proc/http/tracing"
	"github.com/yubo/golib/util"
	"github.com/yubo/goswagger"
	"k8s.io/klog/v2"
)

const (
	moduleName = "sys.http"
)

type Config struct {
	Addr         string           `json:"addr"`
	Profile      bool             `json:"profile"`
	HttpCross    bool             `json:"httpCross"`
	MaxLimitPage int              `json:"maxLimitPage"`
	DefLimitPage int              `json:"defLimitPage"`
	Apidocs      Apidocs          `json:"apidocs"`
	Swagger      goswagger.Config `json:"swagger"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

type Apidocs struct {
	Enabled bool `json:"enabled"`
	spec.InfoProps
}

type Module struct {
	*Config
	name string

	*restful.Container
	ctx             context.Context
	cancel          context.CancelFunc
	swaggerTags     []spec.Tag
	securitySchemes map[string]*spec.SecurityScheme
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
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
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)

func (p *Module) testHook(ops *proc.HookOps, configer *proc.Configer) error {
	cf := &Config{}
	if err := configer.Read(p.name, cf); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}
	if util.AddrIsDisable(cf.Addr) {
		return nil
	}

	return nil
}

func (p *Module) preStartHook(ops *proc.HookOps, configer *proc.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	popts := ops.Options()

	cf := &Config{}
	if err := configer.Read(p.name, cf); err != nil {
		return err
	}
	p.Config = cf

	p.Container = restful.NewContainer()
	http.DefaultServeMux = p.Container.ServeMux
	p.swaggerTags = []spec.Tag{}
	p.securitySchemes = map[string]*spec.SecurityScheme{}

	if p.HttpCross {
		httpCross(p.Container)
	}

	popts = popts.Set(proc.HttpServerName, p)
	ops.SetOptions(popts)

	return nil
}

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) error {

	// /debug
	if p.Profile {
		profile{}.Install(p)
	}

	// filter
	if klog.V(8).Enabled() {
		p.Filter(tracing.DbgFilter)
	}

	// /api
	p.installPing()
	p.installSwagger()
	p.installApidocs()

	if err := p.start(p.ctx); err != nil {
		return err
	}

	return nil
}

func (p *Module) stopHook(ops *proc.HookOps, cf *proc.Configer) error {
	p.cancel()
	return nil
}

func (p *Module) start(ctx context.Context) error {

	container := p.Container
	cf := p.Config

	if util.AddrIsDisable(cf.Addr) {
		klog.Warningf("httpServer is disabled sys.http.addr %s", cf.Addr)
		return nil
	}

	server := &http.Server{
		Addr:    cf.Addr,
		Handler: container.ServeMux,
	}

	klog.V(5).Infof("httpServer Listen addr %s", cf.Addr)

	addr := server.Addr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
	}()

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()
		err := server.Shutdown(ctx)
		klog.V(5).Infof("httpServer %s exit %v", cf.Addr, err)
	}()

	return nil
}

// same as http.Handle()
func (p *Module) Handle(pattern string, handler http.Handler) {
	p.Container.ServeMux.Handle(pattern, handler)
}

func (p *Module) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	p.Container.ServeMux.HandleFunc(pattern, handler)
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func httpCross(container *restful.Container) {
	// Add container filter to enable CORS
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type",
			"Accept", "x-api-key", "Authorization",
			"x-otp-code", "Referer", "User-Agent:",
			"X-Requested-With", "Origin", "host",
			"Connection", "Accept-Language", "Accept-Encoding",
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		CookiesAllowed: true,
		Container:      container,
	}
	container.Filter(cors.Filter)

	// Add container filter to respond to OPTIONS
	container.Filter(container.OPTIONSFilter)
}

func (p *Module) installPing() {
	p.SwaggerTagRegister("general", "general information")

	ws := new(restful.WebService)
	opt := &openapi.WsOption{
		Ws:   ws.Path("/ping").Produces("text/plain"),
		Tags: []string{"general"},
	}

	openapi.WsRouteBuild(opt, []openapi.WsRoute{{
		Desc:   "ping/pong",
		Method: "GET", SubPath: "/",
		Handle: func(req *restful.Request, resp *restful.Response) {
			resp.Write([]byte("OK"))
		},
	}})

	p.Add(ws)
}

func init() {
	proc.RegisterHooks(hookOps)
}
