// this is a sample echo rest api module
package echo

import (
	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/status"
	"google.golang.org/grpc/codes"
)

const (
	moduleName = "echo"
)

type Module struct {
	Name string
	http proc.HttpServer
	auth proc.Auth
}

var (
	_module = &Module{Name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.startHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}}
)

func (p *Module) startHook(ops *proc.HookOps, cf *proc.Configer) error {
	popts := ops.Options()
	p.http = popts.Http()
	p.auth = popts.Auth()
	p.installWs()
	return nil
}

func (p *Module) installWs() {
	openapi.SwaggerTagRegister("echo", "echo Api")

	ws := new(restful.WebService)

	openapi.WsRouteBuild(&openapi.WsOption{
		Ws:   ws.Path("/echo").Produces(openapi.MIME_JSON).Consumes("*/*"),
		Acl:  p.auth.GetFilter,
		Tags: []string{"echo"},
	}, []openapi.WsRoute{{
		Method: "GET", SubPath: "/msg",
		Desc: "msg", Acl: "echo:msg",
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

func init() {
	proc.RegisterHooks(hookOps)
	addAuthScope()
}

func addAuthScope() {
	openapi.ScopeRegister("echo:write", "echo msg")
}
