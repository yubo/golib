package openapi

import (
	"fmt"
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
		b.Metadata(Metadata(v.Scope))
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
		data := false
		sample := util.NewStructPtr(v.Input)
		if err := p.buildParam(sample, v.Consume, &data); err != nil {
			panic(err)
		}
		if data {
			p.b.Reads(reflect.Indirect(reflect.ValueOf(sample)).Interface())
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

func (p *RouteBuilder) buildParam(in interface{}, consume string, data *bool) error {
	rv := reflect.Indirect(reflect.ValueOf(in))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		panic("schema: interface must be a struct")
	}

	var parameter *restful.Parameter
	for i := 0; i < rt.NumField(); i++ {
		fv := rv.Field(i)
		ff := rt.Field(i)
		ft := ff.Type

		if !fv.CanSet() {
			continue
		}

		util.PrepareValue(fv, ft)
		if ft.Kind() == reflect.Ptr {
			fv = fv.Elem()
			ft = ft.Elem()
		}

		opt := getTagOpt(ff)
		// if rt.Name() == "CreateSPInput" {
		// 	klog.InfoDepth(4, fmt.Sprintf("%s getTagOpt %#v", rt.Name(), opt))
		// }
		if opt.skip {
			continue
		}

		if opt.inline {
			if err := p.buildParam(fv.Addr().Interface(), consume, data); err != nil {
				panic(err)
			}
			continue
		}

		if opt.inbody {
			// reads cur as body entry
			p.b.Reads(fv.Interface(), ff.Tag.Get("description"))
			return nil
		}

		if opt.typ == DataType {
			if consume == MIME_URL_ENCODED {
				if err := urlencoded.RouteBuilderReads(p.b, fv, ff, ft); err != nil {
					panic(err)
				}
			} else {
				*data = true
				// p.b.Reads(fv.Interface(), ff.Tag.Get("description"))
			}
			continue
		}

		switch opt.typ {
		case PathType:
			parameter = restful.PathParameter(opt.name, ff.Tag.Get("description"))
		case QueryType:
			parameter = restful.QueryParameter(opt.name, ff.Tag.Get("description"))
		case HeaderType:
			parameter = restful.HeaderParameter(opt.name, ff.Tag.Get("description"))
		default:
			panic(fmt.Sprintf("invalid param kind %s %s at %s",
				opt.typ, ff.Type.String(), rt.Name()))
		}

		switch fv.Kind() {
		case reflect.String:
			parameter.DataType("string")
		case reflect.Bool:
			parameter.DataType("bool")
		case reflect.Uint, reflect.Int, reflect.Int32, reflect.Int64:
			parameter.DataType("integer")
		case reflect.Slice:
			if typeName := fv.Type().Elem().Name(); typeName != "string" {
				panic(fmt.Sprintf("unsupported param %s at %s",
					ff.Name, rt.Name()))
			}
		default:
			panic(fmt.Sprintf("unsupported type %s at %s", ft.String(), rt.Name()))
		}

		if opt.format != "" {
			parameter.DataFormat(opt.format)
		}

		p.b.Param(parameter)

	}
	return nil
}
