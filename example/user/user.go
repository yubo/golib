// this is a sample user rest api module
package user

import (
	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
)

const (
	moduleName = "user"
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

func (p *Module) startHook(ops *proc.HookOps, cf *configer.Configer) error {
	popts := ops.Options()
	p.http = popts.Http()
	p.auth = popts.Auth()
	p.installWs()
	return nil
}

func (p *Module) installWs() {
	openapi.SwaggerTagRegister("user", "user Api - for restful sample")

	ws := new(restful.WebService)

	openapi.WsRouteBuild(&openapi.WsOption{
		Ws:   ws.Path("/users").Produces(openapi.MIME_JSON).Consumes(openapi.MIME_JSON),
		Tags: []string{"user"},
	}, []openapi.WsRoute{{
		Method: "POST", SubPath: "/createWithList",
		Desc:   "create users",
		Handle: p.createUsers,
	}, {
		Method: "POST", SubPath: "/",
		Desc:   "create user",
		Handle: p.createUser,
	}, {
		Method: "GET", SubPath: "/",
		Desc:   "search/list users",
		Handle: p.getUsers,
	}, {
		Method: "GET", SubPath: "/{user-name}",
		Desc:   "get user",
		Handle: p.getUser,
	}, {
		Method: "PUT", SubPath: "/{user-name}",
		Desc:   "update user",
		Handle: p.updateUser,
	}, {
		Method: "DELETE", SubPath: "/{user-name}",
		Desc:   "delete user",
		Handle: p.deleteUser,
	}})

	p.http.Add(ws)
}

func (p *Module) createUser(req *restful.Request, resp *restful.Response, in *CreateUserInput) (*CreateUserOutput, error) {
	id, err := createUser(in)

	return &CreateUserOutput{
		RespStatus: openapi.NewRespStatus(err),
		Data:       openapi.NewRespID(int64(id)),
	}, nil

}

func (p *Module) createUsers(req *restful.Request, resp *restful.Response, in []CreateUserInput) (*CreateUsersOutput, error) {
	total, err := createUsers(in)
	return &CreateUsersOutput{
		RespStatus: openapi.NewRespStatus(err),
		Data:       openapi.NewRespTotal(int64(total)),
	}, nil
}

func (p *Module) getUsers(req *restful.Request, resp *restful.Response, in *GetUsersInput) (*GetUsersOutput, error) {
	total, list, err := getUsers(in)

	ret := &GetUsersOutput{RespStatus: openapi.NewRespStatus(err)}
	ret.Data.Total = total
	ret.Data.List = list

	return ret, nil
}

func (p *Module) getUser(req *restful.Request, resp *restful.Response, in *GetUserInput) (*GetUserOutput, error) {
	ret, err := getUser(in.Name)
	return &GetUserOutput{
		RespStatus: openapi.NewRespStatus(err),
		Data:       ret,
	}, nil
}

func (p *Module) updateUser(req *restful.Request, resp *restful.Response, in *UpdateUserInput) (*UpdateUserOutput, error) {
	ret, err := updateUser(in)
	return &UpdateUserOutput{
		RespStatus: openapi.NewRespStatus(err),
		Data:       ret,
	}, nil

}

func (p *Module) deleteUser(req *restful.Request, resp *restful.Response, in *DeleteUserInput) (*DeleteUserOutput, error) {
	ret, err := deleteUser(in.Name)
	return &DeleteUserOutput{
		RespStatus: openapi.NewRespStatus(err),
		Data:       ret,
	}, nil
}

func init() {
	proc.RegisterHooks(hookOps)
	addAuthScope()
}

func addAuthScope() {
	openapi.ScopeRegister("user:write", "user")
}
