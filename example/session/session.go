// this is a sample echo rest api module
package echo

import (
	"fmt"
	"strconv"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/session"
	"github.com/yubo/golib/status"
	"google.golang.org/grpc/codes"
)

const (
	moduleName = "demo.session"
)

type Module struct {
	Name    string
	http    proc.HttpServer
	session *session.Manager
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

func (p *Module) startHook(ops *proc.HookOps, cf *configer.Configer) error {
	popts := ops.Options()
	p.http = popts.Http()
	p.session = popts.Session()
	p.installWs()
	return nil
}

func (p *Module) installWs() {
	openapi.SwaggerTagRegister("session", "demo session")

	ws := new(restful.WebService)

	openapi.WsRouteBuild(&openapi.WsOption{
		Ws:     ws.Path("/session").Produces(openapi.MIME_JSON).Consumes("*/*"),
		Filter: p.sessionStart,
		Tags:   []string{"session"},
	}, []openapi.WsRoute{{
		Method: "GET", SubPath: "/",
		Desc:   "get session info",
		Handle: p.info,
	}, {
		Method: "GET", SubPath: "/set",
		Desc:   "set session info",
		Handle: p.set,
	}, {
		Method: "GET", SubPath: "/reset",
		Desc:   "reset session info",
		Handle: p.reset,
	}})

	p.http.Add(ws)
}

func (p *Module) sessionStart(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	session, err := p.session.Start(resp, req.Request)
	if err != nil {
		openapi.HttpWriteErr(resp, status.Errorf(codes.Internal,
			"session start err %s", err))
		return

	}
	req.SetAttribute("session", session)
	chain.ProcessFilter(req, resp)
	session.Update(resp)
}

// show session information
func (p *Module) info(req *restful.Request, resp *restful.Response) {
	session, ok := req.Attribute("session").(*session.SessionStore)
	if !ok {
		openapi.HttpWriteEntity(resp, "can't get session info", nil)
		return
	}

	userName := session.Get("userName")
	if userName == "" {
		openapi.HttpWriteEntity(resp, "can't get username from session", nil)
		return
	}

	cnt, err := strconv.Atoi(session.Get("info.cnt"))
	if err != nil {
		cnt = 0
	}

	cnt++
	session.Set("info.cnt", strconv.Itoa(cnt))
	openapi.HttpWriteEntity(resp, fmt.Sprintf("%s %d", userName, cnt), nil)
}

// set session
func (p *Module) set(req *restful.Request, resp *restful.Response) {
	session, ok := req.Attribute("session").(*session.SessionStore)
	if ok {
		session.Set("userName", "tom")
	}
	openapi.HttpWriteEntity(resp, "set username successfully", nil)
}

// reset session
func (p *Module) reset(req *restful.Request, resp *restful.Response) {
	session, ok := req.Attribute("session").(*session.SessionStore)
	if ok {
		session.Reset()
	}
	openapi.HttpWriteEntity(resp, "reset successfully", nil)
}

func init() {
	proc.RegisterHooks(hookOps)
}
