package sys

import (
	"encoding/json"

	restful "github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"

	. "github.com/yubo/golib/sys/api"
	. "github.com/yubo/golib/sys/model"
)

func (p *Module) InstallLogWs(aclHandle func(aclName string) (restful.FilterFunction, string, error)) {
	p.SwaggerTagRegister("log", "get log")

	ws := new(restful.WebService)
	opt := &openapi.WsOption{
		Ws:          ws.Path("/api/v1/logs").Produces(restful.MIME_JSON).Consumes("*/*"),
		Kind:        "log",
		Tags:        []string{"log"},
		Acl:         aclHandle,
		Obj:         Log{},
		ResourceKey: "id",
	}

	openapi.WsRouteBuild(opt, []openapi.WsRoute{{
		Action: "list", Acl: "read",
		Handle: p.getLogs,
		Input:  GetLogsInput{},
	}, {
		Action: "get", Acl: "read",
		Handle: p.getLog,
		Input:  GetLogInput{},
	}})

	p.RestAdd(ws)
}

// ###########################
func (p *Module) getLogs(req *restful.Request, resp *restful.Response) {
	in := &GetLogsInput{}

	ret, err := func() ([]Log, error) {
		if err := openapi.ReadEntity(req, in); err != nil {
			return nil, err
		}

		sqlExtra := in.SqlExtra("id desc")
		return GetLogs(p.db, in.UserName, in.Action, in.Target, in.Start, in.End, sqlExtra)
	}()
	openapi.HttpWriteEntity(resp, ret, err)
}

func (p *Module) getLog(req *restful.Request, resp *restful.Response) {
	in := &GetLogInput{}
	ret, err := func() (*Log, error) {
		if err := openapi.ReadEntity(req, in); err != nil {
			return nil, err
		}
		return GetLog(p.db, in.Id)
	}()

	openapi.HttpWriteEntity(resp, ret, err)
}

type Extra struct {
	Method    string
	Url       string
	PeerAddr  string
	TokenName string
	Data      interface{}
}

func (p *Module) Log(in *CreateLogInput) error {
	return CreateLog(p.db, in)
}

func (p *Module) Log5(req *restful.Request, action, target *string, data interface{}, err error) (e error) {
	token, ok := openapi.TokenFrom(req)
	if !ok {
		token = &openapi.BaseToken{}
	}

	addr := util.GetIPAdress(req.Request)

	// data maybe Marshal error
	if _, e := json.Marshal(data); e != nil {
		data = e.Error()
	}

	klog.V(6).Infof("%s, %s, err %v, userName %s, tokenName %s, ADDR %s, data %s",
		req.Request.Method,
		req.SelectedRoutePath(),
		err,
		token.GetUserName(),
		token.GetTokenName(),
		addr,
		util.JsonStr(data),
	)

	return CreateLog(p.db, &CreateLogInput{
		UserName: util.String(token.GetUserName()),
		Target:   target,
		Action:   action,
		PeerAddr: &addr,
		Extra: util.String(util.JsonStr(Extra{
			Method:    req.Request.Method,
			Url:       req.SelectedRoutePath(),
			PeerAddr:  addr,
			TokenName: token.GetTokenName(),
			Data:      data,
		})),
		Err:       util.ErrorString(err),
		CreatedAt: util.NowPtr(),
	})
}
