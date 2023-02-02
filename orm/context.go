package orm

import (
	"context"
)

type key int

const (
	dbKey key = iota
	interfaceKey
	sqlOutKey
)

func WithDB(ctx context.Context, orm Interface) context.Context {
	return context.WithValue(ctx, interfaceKey, orm)
}
func DBFrom(ctx context.Context) (Interface, bool) {
	i, ok := ctx.Value(interfaceKey).(Interface)
	return i, ok
}

func WithSqlOut(ctx context.Context, out *string) context.Context {
	return context.WithValue(ctx, sqlOutKey, out)
}

func SqlOutFrom(ctx context.Context) *string {
	out, _ := ctx.Value(sqlOutKey).(*string)
	return out
}
