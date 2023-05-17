package orm

import (
	"context"
	"io"
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

func WithSqlWriter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, sqlOutKey, func(sql string) {
		w.Write([]byte(sql))
	})
}

func WithSqlOut(ctx context.Context, out func(string)) context.Context {
	return context.WithValue(ctx, sqlOutKey, out)
}

func SqlOutFrom(ctx context.Context) func(string) {
	out, _ := ctx.Value(sqlOutKey).(func(string))
	return out
}
