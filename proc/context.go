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
	attrKey // attributes
)

func NewContext() context.Context {
	return context.TODO()
}

// WithValue returns a copy of parent in which the value associated with key is val.
func WithValue(parent context.Context, key interface{}, val interface{}) context.Context {
	return context.WithValue(parent, key, val)
}

func WithName(ctx context.Context, name string) context.Context {
	return WithValue(ctx, nameKey, name)
}

// NameFrom returns the value of the name key on the ctx
func NameFrom(ctx context.Context) string {
	name, ok := ctx.Value(nameKey).(string)
	if !ok {
		return os.Args[0]
	}
	return name
}

// WithWg returns a copy of parent in which the user value is set
func WithWg(ctx context.Context, wg *sync.WaitGroup) {
	AttrFrom(ctx)[wgKey] = wg
}

func WgFrom(ctx context.Context) *sync.WaitGroup {
	wg, ok := AttrFrom(ctx)[wgKey].(*sync.WaitGroup)
	if !ok {
		panic("unable to get waitGroup from context")
	}
	return wg
}

func WithConfiger(ctx context.Context, cf *configer.Configer) {
	AttrFrom(ctx)[configerKey] = cf
}

func ConfigerFrom(ctx context.Context) *configer.Configer {
	cf, ok := AttrFrom(ctx)[configerKey].(*configer.Configer)
	if !ok {
		panic("unable to get configer from context")
	}
	return cf
}

func WithConfigOps(parent context.Context, optsInput ...configer.Option) context.Context {
	opts, ok := parent.Value(configOptsKey).(*[]configer.Option)
	if ok {
		*opts = append(*opts, optsInput...)
		return parent
	}

	return WithValue(parent, configOptsKey, &optsInput)
}

func ConfigOptsFrom(ctx context.Context) ([]configer.Option, bool) {
	opts, ok := ctx.Value(configOptsKey).(*[]configer.Option)
	if ok {
		return *opts, true
	}
	return nil, false
}

func WithHookOps(parent context.Context, ops *HookOps) context.Context {
	return WithValue(parent, hookOptsKey, ops)
}

func HookOpsFrom(ctx context.Context) (*HookOps, bool) {
	ops, ok := ctx.Value(hookOptsKey).(*HookOps)
	return ops, ok
}

func WithAttr(ctx context.Context, attributes map[interface{}]interface{}) context.Context {
	return context.WithValue(ctx, attrKey, attributes)
}

func AttrFrom(ctx context.Context) map[interface{}]interface{} {
	attr, ok := ctx.Value(attrKey).(map[interface{}]interface{})
	if !ok {
		panic("unable to get attribute from context")
	}
	return attr
}
