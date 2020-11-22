// this is a sample custom auth module
package auth

import (
	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
	"k8s.io/klog"
)

const (
	moduleName = "auth"
)

type Config struct {
	Name string `json:"name"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

type Module struct {
	*Config
	Name string
	http proc.HttpServer
}

var (
	_module = &Module{Name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_MODULE,
	}, {
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_POST_MODULE,
	}}
)

func (p *Module) preStartHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	popts := ops.Options()

	c := &Config{}
	if err := cf.Read(p.Name, c); err != nil {
		return err
	}
	p.Config = c

	popts = popts.SetAuth(p)

	ops.SetOptions(popts)

	klog.V(10).Infof("auth config: %s", c)
	return
}

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) (err error) {
	popts := ops.Options()

	p.http = popts.Http()

	p.installWs()

	return nil
}

func init() {
	proc.RegisterHooks(hookOps)
}

func (p *Module) GetFilter(acl string) (restful.FilterFunction, string, error) {
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		klog.Infof("before %s filter", acl)
		chain.ProcessFilter(req, resp)
		klog.Infof("after %s filter", acl)
	}, acl, nil
}

func (p *Module) IsAdmin(token openapi.Token) bool {
	return false
}

type BaseClient struct{}

func (p BaseClient) GetId() string          { return "" }
func (p BaseClient) GetSecret() string      { return "" }
func (p BaseClient) GetRedirectUri() string { return "" }

func (p *Module) SsoClient() proc.Client {
	return &BaseClient{}
}

func (p *Module) Access(req *restful.Request, resp *restful.Response, mustLogin bool, chain *restful.FilterChain, handles ...func(openapi.Token) error) {
}

func (p *Module) WsAccess(req *restful.Request, resp *restful.Response, mustLogin bool, chain *restful.FilterChain, handles ...func(openapi.Token) error) {
}

func (p *Module) GetAndVerifyTokenInfoByApiKey(code *string, peerAddr string) (openapi.Token, error) {
	return &openapi.AnonymousToken{}, nil
}
func (p *Module) GetAndVerifyTokenInfoByBearer(code *string) (openapi.Token, error) {
	return &openapi.AnonymousToken{}, nil
}

func (p *Module) installWs() {
	p.http.SwaggerTagRegister("Auth", "Auth Api")
	ws := new(restful.WebService)

	openapi.WsRouteBuild(&openapi.WsOption{
		Ws:   ws.Path("/api/v1/auth").Produces(openapi.MIME_JSON).Consumes("*/*"),
		Acl:  p.GetFilter,
		Tags: []string{"auth"},
	}, []openapi.WsRoute{{
		Method: "GET", SubPath: "/echo",
		Desc: "ping", Acl: "echo",
		Handle: p.echo,
		Input:  echoInput{},
		Output: echoOutput{},
	}})

	p.http.Add(ws)
}

type echoInput struct {
	Msg *string `param:"query" name:"msg" description:"message"`
}

func (p *echoInput) Validate() error {
	if p.Msg == nil {
		return status.Errorf(codes.InvalidArgument, "msg must be set")
	}
	return nil
}

type echoOutput struct {
	Msg string `json:"msg" description:"message"`
}

func (p *Module) echo(req *restful.Request, resp *restful.Response) {
	in := &echoInput{}
	ret, err := func() (ret *echoOutput, err error) {
		if err = openapi.ReadEntity(req, in); err != nil {
			return
		}
		ret = &echoOutput{Msg: *in.Msg}
		return
	}()
	openapi.HttpWriteEntity(resp, ret, err)
}
