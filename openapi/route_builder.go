package openapi

import (
	"net/http"
	"reflect"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/yubo/golib/openapi/urlencoded"
	"github.com/yubo/golib/util"
)

type WsRoute struct {
	Action string
	Acl    string
	//--
	Method      string
	SubPath     string
	Desc        string
	Scope       string
	Consume     string
	Produce     string
	Input       interface{}
	Output      interface{}
	Handle      restful.RouteFunction
	Filter      restful.FilterFunction
	ExtraOutput []ApiOutput
	Tags        []string
}

type ApiOutput struct {
	Code    int
	Message string
	Model   interface{}
}

// struct -> RouteBuilder do
type RouteBuilder struct {
	ws      *restful.WebService
	b       *restful.RouteBuilder
	consume string
}

func NewRouteBuilder(ws *restful.WebService) *RouteBuilder {
	return &RouteBuilder{ws: ws}
}

func (p *RouteBuilder) Build(v *WsRoute) error {
	var b *restful.RouteBuilder

	switch v.Method {
	case "GET":
		b = p.ws.GET(v.SubPath)
	case "POST":
		b = p.ws.POST(v.SubPath)
	case "PUT":
		b = p.ws.PUT(v.SubPath)
	case "DELETE":
		b = p.ws.DELETE(v.SubPath)
	default:
		panic("unsupported method " + v.Method)
	}
	p.b = b

	if v.Scope != "" {
		b.Metadata(SecurityDefinitionKey, v.Scope)
	}

	if v.Consume != "" {
		b.Consumes(v.Consume)
	}

	if v.Produce != "" {
		b.Produces(v.Produce)
	}

	if v.Filter != nil {
		b.Filter(v.Filter)
	}

	if v.Output != nil {
		b.Returns(http.StatusOK, "OK", v.Output)
	}
	for _, out := range v.ExtraOutput {
		b.Returns(out.Code, out.Message, out.Model)
	}

	if v.Input != nil {
		sample := util.NewStructPtr(v.Input)
		if err := p.buildParam(sample, v.Consume); err != nil {
			panic(err)
		}
	}

	if v.Handle != nil {
		b.To(v.Handle)
	} else {
		b.To(func(req *restful.Request, resp *restful.Response) {})
	}
	b.Doc(v.Desc)
	b.Metadata(restfulspec.KeyOpenAPITags, v.Tags)

	p.ws.Route(b)

	return nil
}

func (p *RouteBuilder) buildParam(in interface{}, consume string) error {
	rv := reflect.Indirect(reflect.ValueOf(in))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		panic("schema: interface must be a struct")
	}

	fields := cachedTypeFields(rt)
	//if rt.String() == "api.CreateClientInput" {
	//	klog.Infof("%s", fields)
	//}
	if fields.hasData {
		if consume == MIME_URL_ENCODED {
			if err := urlencoded.RouteBuilderReads(p.b, rv); err != nil {
				panic(err)
			}
		} else {
			p.b.Reads(rv.Interface())
		}
	}

	for _, f := range fields.list {
		if err := p.setParam(&f); err != nil {
			panic(err)
		}
	}

	return nil
}

func (p *RouteBuilder) setParam(f *field) error {
	var parameter *restful.Parameter

	switch f.paramType {
	case PathType:
		parameter = restful.PathParameter(f.key, f.description)
	case QueryType:
		parameter = restful.QueryParameter(f.key, f.description)
	case HeaderType:
		parameter = restful.HeaderParameter(f.key, f.description)
	default:
		panicType(f.typ, "setParam")
	}

	switch f.typ.Kind() {
	case reflect.String:
		parameter.DataType("string")
	case reflect.Bool:
		parameter.DataType("bool")
	case reflect.Uint, reflect.Int, reflect.Int32, reflect.Int64:
		parameter.DataType("integer")
	case reflect.Slice:
		if typeName := f.typ.Elem().Name(); typeName != "string" {
			panicType(f.typ, "unsupported param")
		}
	default:
		panicType(f.typ, "unsupported param")
	}

	if f.format != "" {
		parameter.DataFormat(f.format)
	}

	p.b.Param(parameter)

	return nil
}
