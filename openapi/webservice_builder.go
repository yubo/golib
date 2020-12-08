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
	Filter      restful.FilterFunction
	Filters     []restful.FilterFunction
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

func (p *WsBuilder) Build(opt *WsOption) (err error) {
	var article string

	if opt.Kind != "" {
		article = util.GetArticleForNoun(opt.Kind, " ")
	}

	if opt.ResourceKey == "" {
		opt.ResourceKey = "name"
	}

	rb := NewRouteBuilder(opt.Ws)

	for i, _ := range opt.Routes {
		route := &opt.Routes[i]

		switch route.Action {
		case "list":
			route.SubPath = "/"
			route.Method = "GET"
			route.Desc = "list objects of kind " + opt.Kind
			if route.Output == nil {
				route.Output = util.MakeSlice(opt.Obj)
			}
		case "create":
			route.SubPath = "/"
			route.Method = "POST"
			route.Desc = "create" + article + opt.Kind
			if route.Output == nil {
				route.Output = opt.Obj
			}
		case "get":
			route.SubPath = "/{" + opt.ResourceKey + "}"
			route.Method = "GET"
			route.Desc = "read the specified " + opt.Kind
			if route.Output == nil {
				route.Output = opt.Obj
			}
		case "update":
			route.SubPath = "/{" + opt.ResourceKey + "}"
			route.Method = "PUT"
			route.Desc = "update the specified  " + opt.Kind
			if route.Output == nil {
				route.Output = opt.Obj
			}
		case "delete":
			route.SubPath = "/{" + opt.ResourceKey + "}"
			route.Method = "DELETE"
			route.Desc = "delete" + article + opt.Kind
			if route.Output == nil {
				route.Output = opt.Obj
			}
		case "":
		default:
			panic("unsupported action " + route.Action)
		}

		route.SubPath = opt.PrefixPath + route.SubPath

		if route.Filter != nil {
			route.Filters = append(route.Filters, route.Filter)
		}

		if route.Acl != "" && opt.Acl != nil {
			var filter restful.FilterFunction
			if filter, route.Scope, err = opt.Acl(route.Acl); err != nil {
				panic(err)
			}
			route.Filters = append(route.Filters, filter)
		}

		if len(opt.Filters) > 0 {
			route.Filters = append(route.Filters, opt.Filters...)
		}

		if opt.Filter != nil {
			route.Filters = append(route.Filters, opt.Filter)
		}

		if route.Acl != "" {
			route.Desc += " acl(" + route.Acl + ")"
		}

		if route.Scope != "" {
			route.Desc += " scope(" + route.Scope + ")"
		}

		if route.Output == nil && opt.Output != nil {
			route.Output = opt.Output
		}

		if route.Tags == nil && opt.Tags != nil {
			route.Tags = opt.Tags
		}

		rb.Build(route)
	}
	return nil
}
