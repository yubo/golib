package rpc

import (
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/yubo/golib/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/naming"
)

var (
	ErrBalancerClosed = errors.New("grpc: balancer is closed")
)

/*
type RegisterServer func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error)

// newGateway returns a new gateway server which translates HTTP into gRPC.
func newGateway(registerServer RegisterServer, ctx context.Context, address string, opts ...runtime.ServeMuxOption) (http.Handler, error) {
	mux := runtime.NewServeMux(opts...)

	dialOpts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDialer(util.Dialer),
		grpc.WithBlock(),
	}

	err := registerServer(ctx, mux, address, dialOpts)
	if err != nil {
		return nil, err
	}
	return mux, nil
}

func Gateway(registerServer RegisterServer, ctx context.Context, mux *http.ServeMux, upstream string,
	opts ...runtime.ServeMuxOption) error {

	gw, err := newGateway(registerServer, ctx, upstream, opts...)
	if err != nil {
		return err
	}
	mux.Handle("/", gw)

	return nil
}
*/

type Watcher struct {
	// the channel to receives name resolution updates
	Update chan *naming.Update
	// the side channel to get to know how many updates in a batch
	Side chan int
	// the channel to notifiy update injector that the update reading is done
	readDone chan int
}

func (w *Watcher) Next() (updates []*naming.Update, err error) {
	n := <-w.Side
	if n == 0 {
		//return nil, fmt.Errorf("w.Side is closed")
		return nil, nil
	}
	for i := 0; i < n; i++ {
		u := <-w.Update
		if u != nil {
			updates = append(updates, u)
		}
	}
	w.readDone <- 0
	return
}

func (w *Watcher) Close() {
	close(w.Side)
}

// Inject naming resolution updates to the testWatcher.
func (w *Watcher) Inject(updates []*naming.Update) {
	w.Side <- len(updates)
	for _, u := range updates {
		w.Update <- u
	}
	<-w.readDone
}

type NameResolver struct {
	W     *Watcher
	Addrs []string
}

func (r *NameResolver) Resolve(target string) (naming.Watcher, error) {
	r.W = &Watcher{
		Update:   make(chan *naming.Update, 1),
		Side:     make(chan int, 1),
		readDone: make(chan int),
	}

	r.W.Side <- len(r.Addrs)
	for _, addr := range r.Addrs {
		r.W.Update <- &naming.Update{
			Op:   naming.Add,
			Addr: addr,
		}
	}

	go func() {
		<-r.W.readDone
	}()
	return r.W, nil
}

func (r *NameResolver) Add(addrs ...string) {
	var updates []*naming.Update
	for _, addr := range addrs {
		updates = append(updates, &naming.Update{
			Op:   naming.Add,
			Addr: addr,
		})
	}
	r.W.Inject(updates)
}

func (r *NameResolver) Delete(addrs ...string) {
	var updates []*naming.Update
	for _, addr := range addrs {
		updates = append(updates, &naming.Update{
			Op:   naming.Delete,
			Addr: addr,
		})
	}
	r.W.Inject(updates)
}

func randSlice(in []string, randinit bool) {
	size := len(in)
	if size < 1 {
		return
	}

	if randinit {
		rand.Seed(time.Now().Unix())
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

func DialRr(ctx context.Context, target string, rand bool, opt ...grpc.DialOption) (conn *grpc.ClientConn, r *NameResolver, err error) {
	r = &NameResolver{Addrs: strings.Split(target, ",")}

	if rand {
		randSlice(r.Addrs, false)
	}

	opt = append(opt,
		grpc.WithInsecure(),
		grpc.WithDialer(util.Dialer),
		grpc.WithBlock(),
		grpc.WithBalancer(grpc.RoundRobin(r)),
		grpc.WithUnaryInterceptor(interceptor(ctx)),
	)

	for i := 0; ; i++ {
		timeout, _ := context.WithTimeout(ctx, time.Second)
		conn, err = grpc.DialContext(timeout, "", opt...)
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
