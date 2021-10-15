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
	descriptionKey
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

func WithDescription(ctx context.Context, description string) context.Context {
	return WithValue(ctx, descriptionKey, description)
}

// DescriptionFrom returns the value of the description key on the ctx
func DescriptionFrom(ctx context.Context) string {
	description, ok := ctx.Value(descriptionKey).(string)
	if !ok {
		return os.Args[0]
	}
	return description
}

// WithWg returns a copy of parent in which the user value is set
func WithWg(ctx context.Context, wg *sync.WaitGroup) {
	AttrMustFrom(ctx)[wgKey] = wg
}

func WgFrom(ctx context.Context) (*sync.WaitGroup, bool) {
	wg, ok := AttrMustFrom(ctx)[wgKey].(*sync.WaitGroup)
	return wg, ok
}

func WgMustFrom(ctx context.Context) *sync.WaitGroup {
	wg, ok := AttrMustFrom(ctx)[wgKey].(*sync.WaitGroup)
	if !ok {
		panic("unable to get waitGroup from context")
	}
	return wg
}

func WithConfiger(ctx context.Context, cf *configer.Configer) {
	if _, ok := ConfigerFrom(ctx); ok {
		panic("configer has been exist")
	}
	AttrMustFrom(ctx)[configerKey] = cf
}

func ConfigerFrom(ctx context.Context) (*configer.Configer, bool) {
	cf, ok := AttrMustFrom(ctx)[configerKey].(*configer.Configer)
	return cf, ok
}

func ConfigerMustFrom(ctx context.Context) *configer.Configer {
	cf, ok := AttrMustFrom(ctx)[configerKey].(*configer.Configer)
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

func AttrFrom(ctx context.Context) (map[interface{}]interface{}, bool) {
	attr, ok := ctx.Value(attrKey).(map[interface{}]interface{})
	return attr, ok
}

func AttrMustFrom(ctx context.Context) map[interface{}]interface{} {
	attr, ok := ctx.Value(attrKey).(map[interface{}]interface{})
	if !ok {
		panic("unable to get attribute from context")
	}
	return attr
}
