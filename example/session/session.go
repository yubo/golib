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
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_MODULE,
	}}
)

func (p *Module) start(ops *proc.HookOps, cf *configer.Configer) error {
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
		Filter: session.Filter(p.session),
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

// show session information
func (p *Module) info(req *restful.Request, resp *restful.Response) {
	sess, ok := session.FromReq(req)
	if !ok {
		openapi.HttpWriteEntity(resp, "can't get session info", nil)
		return
	}

	userName := sess.Get("userName")
	if userName == "" {
		openapi.HttpWriteEntity(resp, "can't get username from session", nil)
		return
	}

	cnt, err := strconv.Atoi(sess.Get("info.cnt"))
	if err != nil {
		cnt = 0
	}

	cnt++
	sess.Set("info.cnt", strconv.Itoa(cnt))
	openapi.HttpWriteEntity(resp, fmt.Sprintf("%s %d", userName, cnt), nil)
}

// set session
func (p *Module) set(req *restful.Request, resp *restful.Response) {
	sess, ok := session.FromReq(req)
	if ok {
		sess.Set("userName", "tom")
	}
	openapi.HttpWriteEntity(resp, "set username successfully", nil)
}

// reset session
func (p *Module) reset(req *restful.Request, resp *restful.Response) {
	sess, ok := session.FromReq(req)
	if ok {
		sess.Reset()
	}
	openapi.HttpWriteEntity(resp, "reset successfully", nil)
}

func init() {
	proc.RegisterHooks(hookOps)
}
