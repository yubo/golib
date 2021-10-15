package rpc

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/yubo/golib/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
)

var (
	grpcScheme        = "grpc_scheme"
	serviceSeq uint32 = 1
)

func randSlice(in []string) {
	size := len(in)
	if size < 1 {
		return
	}

	for i := 0; i < size-1; i++ {
		//addr[size-i] <-> [0, size-i)
		src := size - i - 1
		dst := rand.Intn(src + 1)

		t := in[src]
		in[src] = in[dst]
		in[dst] = t
	}
}

func DialRr(ctx context.Context, target string, rand bool, opt ...grpc.DialOption) (conn *grpc.ClientConn, err error) {

	addrs := strings.Split(target, ",")
	if rand {
		randSlice(addrs)
	}

	serviceName := strconv.FormatUint(uint64(atomic.AddUint32(&serviceSeq, 1)), 10) + ".lb.grpc.io"

	resolver.Register(&lbResolverBuilder{
		scheme:      grpcScheme,
		serviceName: serviceName,
		addrs:       addrs,
	})

	opt = append(opt,
		grpc.WithInsecure(),
		grpc.WithDialer(util.Dialer),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(interceptor(ctx)),
	)

	for i := 0; ; i++ {
		timeout, _ := context.WithTimeout(ctx, time.Second)
		conn, err = grpc.DialContext(timeout,
			fmt.Sprintf("%s:///%s", grpcScheme, serviceName),
			opt...,
		)
		if err == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func interceptor(in context.Context) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, resp interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		sp := opentracing.SpanFromContext(in)
		if sp == nil {
			return invoker(ctx, method, req, resp, cc, opts...)
		}

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, TextMapCarrier{md})
		if err != nil {
			grpclog.Errorf("inject to metadata err %v", err)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
		if err = invoker(ctx, method, req, resp, cc, opts...); err != nil {
			sp.LogFields(log.Error(err))
		}
		return err
	}
}

type TextMapCarrier struct {
	metadata.MD
}

// ForeachKey conforms to the TextMapReader interface.
func (c TextMapCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, v := range c.MD {
		for _, v2 := range v {
			if err := handler(k, v2); err != nil {
				return err
			}
		}
	}
	return nil
}

// Set implements Set() of opentracing.TextMapWriter
func (c TextMapCarrier) Set(key, val string) {
	key = strings.ToLower(key)
	c.MD[key] = append(c.MD[key], val)
}

type lbResolverBuilder struct {
	scheme      string
	serviceName string
	addrs       []string
}

func (p *lbResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &lbResolver{
		target: target,
		cc:     cc,
		addrsStore: map[string][]string{
			p.serviceName: p.addrs,
		},
	}
	r.start()
	return r, nil
}
func (p *lbResolverBuilder) Scheme() string { return p.scheme }

type lbResolver struct {
	target     resolver.Target
	cc         resolver.ClientConn
	addrsStore map[string][]string
}

func (r *lbResolver) start() {
	addrStrs := r.addrsStore[r.target.Endpoint]
	addrs := make([]resolver.Address, len(addrStrs))
	for i, s := range addrStrs {
		addrs[i] = resolver.Address{Addr: s}
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}
func (*lbResolver) ResolveNow(o resolver.ResolveNowOptions) {}
func (*lbResolver) Close()                                  {}
