package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/rpc"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
)

const (
	moduleName = "sys.grpc"
)

type Config struct {
	Addr           string `json:"addr"`
	MaxRecvMsgSize int    `json:"maxRecvMsgSize"`
}

type Module struct {
	*Config
	name   string
	ctx    context.Context
	cancel context.CancelFunc

	*grpc.Server
}

var (
	_module = &Module{name: moduleName}
	hookOps = []proc.HookOps{{
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.test,
		Owner:    moduleName,
		HookNum:  proc.ACTION_TEST,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.stop,
		Owner:    moduleName,
		HookNum:  proc.ACTION_STOP,
		Priority: proc.PRI_SYS,
	}, {
		Hook:     _module.preStart,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_PRE_SYS,
	}, {
		Hook:     _module.start,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_SYS,
	}}
)

func (p *Module) test(ops *proc.HookOps, configer *proc.Configer) error {
	cf := &Config{}
	if err := configer.Read(p.name, cf); err != nil {
		return fmt.Errorf("%s read config err: %s", p.name, err)
	}
	if util.AddrIsDisable(cf.Addr) {
		return nil
	}

	return nil
}

func (p *Module) preStart(ops *proc.HookOps, configer *proc.Configer) (err error) {
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())

	popts := ops.Options()

	cf := &Config{}
	if err := configer.Read(p.name, cf); err != nil {
		return err
	}
	p.Config = cf

	// grpc api
	p.Server = newServer(cf, grpc.UnaryInterceptor(interceptor))
	popts = popts.SetGrpc(p.Server)

	ops.SetOptions(popts)
	return nil
}

func (p *Module) start(ops *proc.HookOps, configer *proc.Configer) error {
	popts := ops.Options()
	cf := p.Config
	server := p.Server

	if util.AddrIsDisable(cf.Addr) {
		return nil
	}

	ln, err := net.Listen(util.CleanSockFile(util.ParseAddr(cf.Addr)))
	if err != nil {
		return err
	}
	klog.V(5).Infof("grpcServer Listen addr %s", cf.Addr)

	reflection.Register(server)

	go func() {
		wg := popts.Wg()
		wg.Add(1)
		defer wg.Add(-1)

		if err := server.Serve(ln); err != nil {
			return
		}
	}()

	go func() {
		<-p.ctx.Done()
		server.GracefulStop()
	}()

	return nil

}

func (p *Module) stop(ops *proc.HookOps, cf *proc.Configer) error {
	p.cancel()
	return nil
}

func newServer(cf *Config, opt ...grpc.ServerOption) *grpc.Server {
	if cf.MaxRecvMsgSize > 0 {
		klog.V(5).Infof("set grpc server max recv msg size %s",
			util.ByteSize(cf.MaxRecvMsgSize).HumanReadable())
		opt = append(opt, grpc.MaxRecvMsgSize(cf.MaxRecvMsgSize))
	}

	return grpc.NewServer(opt...)
}

func interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {

	if opentracing.IsGlobalTracerRegistered() {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		tr := opentracing.GlobalTracer()
		spanContext, _ := tr.Extract(opentracing.TextMap, rpc.TextMapCarrier{md})
		sp := tr.StartSpan(info.FullMethod,
			ext.RPCServerOption(spanContext), ext.SpanKindRPCServer)
		defer sp.Finish()
		ctx = opentracing.ContextWithSpan(ctx, sp)
	}

	return handler(ctx, req)
}

func init() {
	proc.RegisterHooks(hookOps)
}
