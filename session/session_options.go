package session

import (
	"context"

	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util/clock"
)

type sessionOptions struct {
	ctx    context.Context
	cancel context.CancelFunc
	clock  clock.Clock
	db     *orm.Db
	mem    bool
}

type SessionOption interface {
	apply(*sessionOptions)
}

type funcSessionOption struct {
	f func(*sessionOptions)
}

func (p *funcSessionOption) apply(opt *sessionOptions) {
	p.f(opt)
}

func newFuncSessionOption(f func(*sessionOptions)) *funcSessionOption {
	return &funcSessionOption{
		f: f,
	}
}

func WithCtx(ctx context.Context) SessionOption {
	return newFuncSessionOption(func(o *sessionOptions) {
		o.ctx = ctx
		o.cancel = nil
	})
}

func WithDb(db *orm.Db) SessionOption {
	return newFuncSessionOption(func(o *sessionOptions) {
		o.db = db
	})
}

func WithClock(clock clock.Clock) SessionOption {
	return newFuncSessionOption(func(o *sessionOptions) {
		o.clock = clock
	})
}

func WithMem() SessionOption {
	return newFuncSessionOption(func(o *sessionOptions) {
		o.mem = true
	})
}
