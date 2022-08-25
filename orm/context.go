package orm

import (
	"context"
)

type key int

const (
	dbKey key = iota
	interfaceKey
)

//func WithDB(ctx context.Context, db DB) context.Context {
//	return context.WithValue(ctx, dbKey, db)
//}
//func DBFrom(ctx context.Context) (DB, bool) {
//	db, ok := ctx.Value(dbKey).(DB)
//	return db, ok
//}

func WithDB(ctx context.Context, orm Interface) context.Context {
	return context.WithValue(ctx, interfaceKey, orm)
}
func DBFrom(ctx context.Context) (Interface, bool) {
	i, ok := ctx.Value(interfaceKey).(Interface)
	return i, ok
}
