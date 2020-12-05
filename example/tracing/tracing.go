// this is a sample echo rest api module
package tracing

import (
	"context"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/rpc"
	"k8s.io/klog"
)

const (
	moduleName = "tracing"
)

type Module struct {
	Name string
	http proc.HttpServer
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

	if server := popts.Grpc(); server != nil {
		RegisterServiceServer(server, &grpcserver{})
	}

	p.installWs()
	return nil
}

func (p *Module) installWs() {
	openapi.SwaggerTagRegister("tracing", "tracing demo")

	ws := new(restful.WebService)

	openapi.WsRouteBuild(&openapi.WsOption{
		Ws:   ws.Path("/tracing").Produces(openapi.MIME_JSON).Consumes("*/*"),
		Tags: []string{"tracing"},
	}, []openapi.WsRoute{{
		Method: "GET", SubPath: "/a",
		Desc:   "a -> a1",
		Handle: p.a,
	}, {
		Method: "GET", SubPath: "/b",
		Desc:   "b -> b1",
		Handle: p.b,
	}, {
		Method: "GET", SubPath: "/b1",
		Desc:   "b1",
		Handle: p.b1,
	}, {
		Method: "GET", SubPath: "/c",
		Desc:   "c -> C1(grpc)",
		Handle: p.c,
	}})

	p.http.Add(ws)
}

func delay() {
	time.Sleep(time.Millisecond * 100)
}

// a -> a1
func (p *Module) a(req *restful.Request, resp *restful.Response) {

	sp, ctx := opentracing.StartSpanFromContext(
		req.Request.Context(), "helo.tracing.a",
	)
	defer sp.Finish()

	sp.LogFields(log.String("msg", "from a"))
	//delay()

	a1(ctx)

	openapi.HttpWriteEntity(resp, nil, nil)
}

func a1(ctx context.Context) {
	sp, _ := opentracing.StartSpanFromContext(
		ctx, "helo.tracing.a1",
	)
	defer sp.Finish()

	sp.LogFields(log.String("msg", "from a1"))
	//delay()
}

// b -> b1
func (p *Module) b(req *restful.Request, resp *restful.Response) {
	sp, ctx := opentracing.StartSpanFromContext(
		req.Request.Context(), "helo.tracing.b",
	)
	defer sp.Finish()

	sp.LogFields(log.String("msg", "from b"))
	delay()

	// call b1
	_, _, err := openapi.HttpRequest(&openapi.RequestOption{
		Url:    "http://localhost:8080/tracing/b1",
		Method: "GET",
		Ctx:    ctx,
	})

	openapi.HttpWriteEntity(resp, nil, err)
}

func (p *Module) b1(req *restful.Request, resp *restful.Response) {
	sp, _ := opentracing.StartSpanFromContext(
		req.Request.Context(), "helo.tracing.b1",
	)
	defer sp.Finish()

	sp.LogFields(log.String("msg", "from b1"))
	delay()

	openapi.HttpWriteEntity(resp, nil, nil)
}

func (p *Module) c(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	sp, _ := opentracing.StartSpanFromContext(ctx, "helo.tracing.c")
	defer sp.Finish()

	sp.LogFields(log.String("msg", "from c"))
	delay()

	//time.Sleep(time.Second * 1)
	conn, _, err := rpc.DialRr(ctx, "127.0.0.1:8081", false)
	if err != nil {
		klog.Errorf("Dial err %v\n", err)
		return
	}
	defer conn.Close()

	c := NewServiceClient(conn)
	ret, err := c.C1(ctx, &Request{Name: "tom"})

	openapi.HttpWriteEntity(resp, ret, err)
}

type grpcserver struct{}

func (s *grpcserver) C1(ctx context.Context, in *Request) (*Response, error) {
	klog.Infof("receive req : %v \n", *in)

	sp, _ := opentracing.StartSpanFromContext(ctx, "helo.tracing.C1")
	defer sp.Finish()

	sp.LogFields(log.String("msg", "from C1"))

	return &Response{Message: "Hello " + in.Name}, nil
}

func init() {
	proc.RegisterHooks(hookOps)
}
