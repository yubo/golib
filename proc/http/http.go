package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/go-openapi/spec"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
	"github.com/yubo/goswagger"
	"k8s.io/klog/v2"
)

const (
	moduleName                 = "sys.http"
	serverGracefulCloseTimeout = 10 * time.Second
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

func (p *Config) Validate() error {
	if p.Swagger.ClientId == "" {
		p.Swagger.ClientId = openapi.NativeClientID
		p.Swagger.ClientSecret = openapi.NativeClientSecret
	}
	if p.Swagger.Url == "" && len(p.Swagger.Urls) == 0 {
		p.Swagger.Name = "Embedded"
		p.Swagger.Url = "/apidocs.json"
	}
	for _, v := range p.Swagger.Schemes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("schemes %s", err)
		}
	}

	return nil
}

type Apidocs struct {
	Enabled bool `json:"enabled"`
	spec.InfoProps
}

type Module struct {
	*Config
	name string

	*restful.Container
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.testHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.start,
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

func (p *Module) preStart(ops *proc.HookOps, configer *proc.Configer) (err error) {
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

	if p.HttpCross {
		httpCross(p.Container)
	}

	popts = popts.SetHttp(p)
	ops.SetOptions(popts)

	return nil
}

func (p *Module) start(ops *proc.HookOps, configer *proc.Configer) error {
	popts := ops.Options()

	// /debug
	if p.Profile {
		profile{}.Install(p)
	}

	// filter
	if klog.V(8).Enabled() {
		p.Filter(DbgFilter)
	}

	// /api
	p.installPing()

	if p.Swagger.Enabled {
		goswagger.New(&p.Config.Swagger).Install(p)
	}

	if p.Apidocs.Enabled {
		openapi.InstallApiDocs(p, p.Apidocs.InfoProps)
	}

	if err := p.startServer(p.ctx, popts.Wg()); err != nil {
		return err
	}

	return nil
}

func (p *Module) stop(ops *proc.HookOps, configer *proc.Configer) error {
	p.cancel()
	return nil
}

func (p *Module) startServer(ctx context.Context, wg sync.WaitGroup) error {
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
		wg.Add(1)
		defer wg.Add(-1)

		server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
	}()

	go func() {
		<-ctx.Done()
		ctx, _ := context.WithTimeout(context.Background(), serverGracefulCloseTimeout)
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
	openapi.SwaggerTagRegister("general", "general information")

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
