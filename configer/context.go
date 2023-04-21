package configer

import "context"

// The key type is unexported to prevent collisions
type key int

const (
	configerKey key = iota
)

func WithConfiger(parent context.Context, cf ParsedConfiger) context.Context {
	if _, ok := ConfigerFrom(parent); ok {
		panic("configer has been exist")
	}
	return context.WithValue(parent, configerKey, cf)
}

func ConfigerFrom(ctx context.Context) (ParsedConfiger, bool) {
	cf, ok := ctx.Value(configerKey).(ParsedConfiger)
	return cf, ok
}

func ConfigerMustFrom(ctx context.Context) ParsedConfiger {
	cf, ok := ctx.Value(configerKey).(ParsedConfiger)
	if !ok {
		panic("unable to get configer from context")
	}
	return cf
}
