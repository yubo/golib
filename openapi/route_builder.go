package openapi

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/yubo/golib/openapi/urlencoded"
)

func WsRouteBuild(opt *WsOption, in []WsRoute) {
	opt.Routes = in
	NewWsBuilder().Build(opt)
}

// opt.Filter > opt.Filters > route.acl > route.filter > route.filters
type WsOption struct {
	Ws         *restful.WebService
	Acl        func(aclName string) (restful.FilterFunction, string, error)
	Filter     restful.FilterFunction
	Filters    []restful.FilterFunction
	PrefixPath string
	Tags       []string
	Routes     []WsRoute
}

type WsBuilder struct{}

func NewWsBuilder() *WsBuilder {
	return &WsBuilder{}
}

func (p *WsBuilder) Build(opt *WsOption) (err error) {
	rb := NewRouteBuilder(opt.Ws)

	for i, _ := range opt.Routes {
		route := &opt.Routes[i]

		route.SubPath = opt.PrefixPath + route.SubPath
		route.Filters = routeFilters(route, opt)

		if route.Acl != "" {
			route.Desc += " acl(" + route.Acl + ")"
		}

		if route.Scope != "" {
			route.Desc += " scope(" + route.Scope + ")"
		}

		if route.Tags == nil && opt.Tags != nil {
			route.Tags = opt.Tags
		}

		rb.Build(route)
	}
	return nil
}

// opt.Filter > opt.Filters > route.acl > route.filter > route.filters
func routeFilters(route *WsRoute, opt *WsOption) (filters []restful.FilterFunction) {
	var filter restful.FilterFunction
	var err error

	if opt.Filter != nil {
		filters = append(filters, opt.Filter)
	}

	if len(opt.Filters) > 0 {
		filters = append(filters, opt.Filters...)
	}

	if route.Acl != "" && opt.Acl != nil {
		if filter, route.Scope, err = opt.Acl(route.Acl); err != nil {
			panic(err)
		}
		filters = append(filters, filter)
	}

	if route.Filter != nil {
		filters = append(filters, route.Filter)
	}

	if len(route.Filters) > 0 {
		filters = append(filters, route.Filters...)
	}

	return filters
}

type WsRoute struct {
	//Action string
	Acl string
	//--
	Method      string
	SubPath     string
	Desc        string
	Scope       string
	Consume     string
	Produce     string
	Handle      interface{}
	Filter      restful.FilterFunction
	Filters     []restful.FilterFunction
	ExtraOutput []ApiOutput
	Tags        []string
	// Input       interface{}
	// Output      interface{}
	// Handle      restful.RouteFunction
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

func (p *RouteBuilder) Build(wr *WsRoute) error {
	var b *restful.RouteBuilder

	switch wr.Method {
	case "GET":
		b = p.ws.GET(wr.SubPath)
	case "POST":
		b = p.ws.POST(wr.SubPath)
	case "PUT":
		b = p.ws.PUT(wr.SubPath)
	case "DELETE":
		b = p.ws.DELETE(wr.SubPath)
	default:
		panic("unsupported method " + wr.Method)
	}
	p.b = b

	if wr.Scope != "" {
		b.Metadata(SecurityDefinitionKey, wr.Scope)
	}

	if wr.Consume != "" {
		b.Consumes(wr.Consume)
	}

	if wr.Produce != "" {
		b.Produces(wr.Produce)
	}

	for _, filter := range wr.Filters {
		b.Filter(filter)
	}

	for _, out := range wr.ExtraOutput {
		b.Returns(out.Code, out.Message, out.Model)
	}

	if err := p.registerHandle(b, wr); err != nil {
		panic(err)
	}

	b.Doc(wr.Desc)
	b.Metadata(restfulspec.KeyOpenAPITags, wr.Tags)

	p.ws.Route(b)

	return nil
}

func noneHandle(req *restful.Request, resp *restful.Response) {}

func (p *RouteBuilder) registerHandle(b *restful.RouteBuilder, wr *WsRoute) error {
	if wr.Handle == nil {
		b.To(noneHandle)
		return nil
	}

	// handle(req *restful.Request, resp *restful.Response)
	// handle(req *restful.Request, resp *restful.Response, in []struct)
	// handle(req *restful.Request, resp *restful.Response, in *struct)
	// handle(...) error
	// handle(...) (out struct, err error)

	v := reflect.ValueOf(wr.Handle)
	t := v.Type()

	nIn := t.NumIn()
	nOut := t.NumOut()

	if !((nIn == 2 || nIn == 3) && (nOut == 0 || nOut == 1 || nOut == 2)) {
		return fmt.Errorf("%s handle in num %d out num %d is Invalid", t.Name(), nIn, nOut)
	}

	reqInterfaceType := reflect.TypeOf((*restful.Request)(nil))
	if !t.In(0).ConvertibleTo(reqInterfaceType) {
		return fmt.Errorf("unable to get req *restful.Request at in(0) get %s", t.In(0).String())
	}

	respInterfaceType := reflect.TypeOf((*restful.Response)(nil))
	if !t.In(1).ConvertibleTo(respInterfaceType) {
		return fmt.Errorf("unable to get req *restful.Response at in(1)")
	}

	var inputType reflect.Type
	var isStruct bool
	if nIn == 3 {
		inputType = t.In(2)

		switch inputType.Kind() {
		case reflect.Ptr:
			inputType = inputType.Elem()
			if inputType.Kind() != reflect.Struct {
				return fmt.Errorf("must ptr to struct, got ptr -> %s", inputType.Kind())
			}
			isStruct = true
		case reflect.Slice:
		default:
			return fmt.Errorf("just support slice and ptr to struct")
		}

		if err := p.buildParam(reflect.New(inputType).Interface(), wr.Consume); err != nil {
			return err
		}
	}

	if nOut == 2 {
		ot := t.Out(0)
		if ot.Kind() == reflect.Ptr {
			ot = ot.Elem()
		}
		output := reflect.New(ot).Interface()
		b.Returns(http.StatusOK, "OK", output)
	}

	b.To(func(req *restful.Request, resp *restful.Response) {
		var (
			ret  []reflect.Value
			data interface{}
			err  error
		)

		if nIn == 3 {
			input := reflect.New(inputType).Interface()
			if err := ReadEntity(req, input); err != nil {
				HttpWriteData(resp, nil, err)
				return
			}

			inputValue := reflect.ValueOf(input)
			if !isStruct {
				inputValue = reflect.Indirect(inputValue)
			}

			ret = v.Call([]reflect.Value{
				reflect.ValueOf(req),
				reflect.ValueOf(resp),
				inputValue,
			})
		} else {
			ret = v.Call([]reflect.Value{
				reflect.ValueOf(req),
				reflect.ValueOf(resp),
			})
		}

		if nOut == 2 {
			if ret[0].CanInterface() {
				data = ret[0].Interface()
			}
			if !ret[1].IsNil() {
				err = ret[1].Interface().(error)
			}
		} else if nOut == 1 {
			if !ret[0].IsNil() {
				err = ret[0].Interface().(error)
			}
		}
		RespWriter(resp, data, err)
	})
	return nil
}

func (p *RouteBuilder) buildParam(in interface{}, consume string) error {
	rv := reflect.Indirect(reflect.ValueOf(in))
	rt := rv.Type()

	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Map || rv.Kind() == reflect.Array {
		p.b.Reads(rv.Interface())
		return nil
	}

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		panic(fmt.Sprintf("schema: interface must be a struct get %s %s", rt.Kind(), rt.String()))
	}

	fields := cachedTypeFields(rt)
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
