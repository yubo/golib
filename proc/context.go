package proc

import (
	"context"
	"os"
	"sync"

	"github.com/yubo/golib/configer"
)

// The key type is unexported to prevent collisions
type key int

const (
	nameKey key = iota
	wgKey
	configerKey
	configOptsKey
	hookOptsKey
)

func NewContext() context.Context {
	return context.TODO()
}

// WithValue returns a copy of parent in which the value associated with key is val.
func WithValue(parent context.Context, key interface{}, val interface{}) context.Context {
	return context.WithValue(parent, key, val)
}

// WithWg returns a copy of parent in which the user value is set
func WithWg(parent context.Context, wg *sync.WaitGroup) context.Context {
	return WithValue(parent, wgKey, wg)
}

// WgFrom returns the value of the WaitGroup key on the ctx
func WgFrom(ctx context.Context) (*sync.WaitGroup, bool) {
	wg, ok := ctx.Value(wgKey).(*sync.WaitGroup)
	return wg, ok
}

func WithConfiger(parent context.Context, cf *configer.Configer) context.Context {
	return WithValue(parent, configerKey, cf)
}

func ConfigerFrom(ctx context.Context) *configer.Configer {
	cf, ok := ctx.Value(configerKey).(*configer.Configer)
	if !ok {
		panic("unable to get configer from context")
	}
	return cf
}

func WithConfigOps(parent context.Context, opts_ ...configer.Option) context.Context {
	opts, ok := parent.Value(configOptsKey).(*[]configer.Option)
	if ok {
		*opts = append(*opts, opts_...)
		return parent
	}

	return WithValue(parent, configOptsKey, &opts_)
}

func ConfigOptsFrom(ctx context.Context) ([]configer.Option, bool) {
	opts, ok := ctx.Value(configOptsKey).(*[]configer.Option)
	if ok {
		return *opts, true
	}
	return nil, false
}

func MustWgFrom(ctx context.Context) *sync.WaitGroup {
	wg, ok := ctx.Value(wgKey).(*sync.WaitGroup)
	if !ok {
		panic("unable to get waitGroup from context")
	}
	return wg
}

// WithName returns a copy of parent in which the user value is set
func WithName(parent context.Context, name string) context.Context {
	return WithValue(parent, nameKey, name)
}

// NameFrom returns the value of the name key on the ctx
func NameFrom(ctx context.Context) string {
	name, ok := ctx.Value(nameKey).(string)
	if !ok {
		return os.Args[0]
	}
	return name
}
