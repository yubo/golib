package proc

import (
	"context"
	"sync"
)

// The key type is unexported to prevent collisions
type key int

const (
	hookOptsKey key = iota
	wgKey
	attrKey // attributes
)

func NewContext() context.Context {
	return context.TODO()
}

// WithValue returns a copy of parent in which the value associated with key is val.
func WithValue(parent context.Context, key interface{}, val interface{}) context.Context {
	return context.WithValue(parent, key, val)
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
		panic("unable to get attr from context")
	}
	return attr
}
