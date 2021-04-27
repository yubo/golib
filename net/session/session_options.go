package session

import (
	"context"

	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/staging/util/clock"
)

type options struct {
	ctx    context.Context
	cancel context.CancelFunc
	clock  clock.Clock
	db     *orm.DB
	mem    bool
}

type Option interface {
	apply(*options)
}

type funcOption struct {
	f func(*options)
}

func (p *funcOption) apply(opt *options) {
	p.f(opt)
}

func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

func WithCtx(ctx context.Context) Option {
	return newFuncOption(func(o *options) {
		o.ctx = ctx
		o.cancel = nil
	})
}

func WithDB(db *orm.DB) Option {
	return newFuncOption(func(o *options) {
		o.db = db
	})
}

func WithClock(clock clock.Clock) Option {
	return newFuncOption(func(o *options) {
		o.clock = clock
	})
}

func WithMem() Option {
	return newFuncOption(func(o *options) {
		o.mem = true
	})
}
