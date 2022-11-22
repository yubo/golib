package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	httpserver "github.com/yubo/golib/net/http"
)

type hw struct{}

func (h *hw) Hello(ctx context.Context) (string, error) {
	return "hello world", nil
}

type Args struct {
	A, B int
}

type Reply struct {
	C int
}
type Arith int

func (t *Arith) Add(ctx context.Context, args *Args) (*Reply, error) {
	return &Reply{C: args.A + args.B}, nil
}

func (t *Arith) Mul(ctx context.Context, args *Args) (*Reply, error) {
	return &Reply{C: args.A * args.B}, nil
}

func (t *Arith) Div(ctx context.Context, args Args) (*Reply, error) {
	if args.B == 0 {
		return nil, errors.New("divide by zero")
	}
	return &Reply{C: args.A / args.B}, nil
}

func (t *Arith) String(ctx context.Context, args *Args) (string, error) {
	return fmt.Sprintf("%d+%d=%d", args.A, args.B, args.A+args.B), nil
}

func (t *Arith) Scan(ctx context.Context, args *string) (reply *Reply, err error) {
	reply = &Reply{}
	_, err = fmt.Sscan(*args, &reply.C)
	return
}

func (t *Arith) Error(ctx context.Context, args *Args) (*Reply, error) {
	panic("ERROR")
}

func (t *Arith) SleepMilli(ctx context.Context, args *Args) error {
	time.Sleep(time.Duration(args.A) * time.Millisecond)
	return nil
}

func main() {
	server := httpserver.NewServer()
	if err := server.Register(new(hw)); err != nil {
		fmt.Println(err)
	}
	if err := server.Register(new(Arith)); err != nil {
		fmt.Println(err)
	}

	if err := http.ListenAndServe("0.0.0.0:8000", server); err != nil {
		os.Exit(1)
	}
}
