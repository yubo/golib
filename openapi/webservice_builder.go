package openapi

import (
	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/util"
)

func WsRouteBuild(opt *WsOption, in []WsRoute) {
	opt.Routes = in
	NewWsBuilder().Build(opt)
}

type WsOption struct {
	Ws          *restful.WebService
	Acl         func(aclName string) (restful.FilterFunction, string, error)
	Kind        string
	PrefixPath  string
	ResourceKey string // e.g. name => xxx/{name}/
	Tags        []string
	Obj         interface{}
	Output      interface{}
	Routes      []WsRoute
}

type WsBuilder struct {
}

func NewWsBuilder() *WsBuilder {
	return &WsBuilder{}
}

func (p *WsBuilder) Build(in *WsOption) (err error) {
	var article string

	if in.Kind != "" {
		article = util.GetArticleForNoun(in.Kind, " ")
	}

	if in.ResourceKey == "" {
		in.ResourceKey = "name"
	}

	rb := NewRouteBuilder(in.Ws)

	for i, _ := range in.Routes {
		v := &in.Routes[i]

		switch v.Action {
		case "list":
			v.SubPath = "/"
			v.Method = "GET"
			v.Desc = "list objects of kind " + in.Kind
			if v.Output == nil {
				v.Output = util.MakeSlice(in.Obj)
			}
		case "create":
			v.SubPath = "/"
			v.Method = "POST"
			v.Desc = "create" + article + in.Kind
			if v.Output == nil {
				v.Output = in.Obj
			}
		case "get":
			v.SubPath = "/{" + in.ResourceKey + "}"
			v.Method = "GET"
			v.Desc = "read the specified " + in.Kind
			if v.Output == nil {
				v.Output = in.Obj
			}
		case "update":
			v.SubPath = "/{" + in.ResourceKey + "}"
			v.Method = "PUT"
			v.Desc = "update the specified  " + in.Kind
			if v.Output == nil {
				v.Output = in.Obj
			}
		case "delete":
			v.SubPath = "/{" + in.ResourceKey + "}"
			v.Method = "DELETE"
			v.Desc = "delete" + article + in.Kind
			if v.Output == nil {
				v.Output = in.Obj
			}
		case "":
		default:
			panic("unsupported action " + v.Action)
		}

		v.SubPath = in.PrefixPath + v.SubPath

		if v.Acl != "" && v.Filter == nil {
			if in.Acl == nil {
				panic("acl handle is empty")
			}
			if v.Filter, v.Scope, err = in.Acl(v.Acl); err != nil {
				panic(err)
			}
		}

		if v.Acl != "" {
			v.Desc += " acl(" + v.Acl + ")"
		}

		if v.Scope != "" {
			v.Desc += " scope(" + v.Scope + ")"
		}

		if v.Output == nil && in.Output != nil {
			v.Output = in.Output
		}

		if v.Tags == nil && in.Tags != nil {
			v.Tags = in.Tags
		}

		rb.Build(v)
	}
	return nil
}
