package orm

import (
	"context"
	"reflect"
	"time"
)

const (
	DefaultMaxRows = 1000
)

type Options struct {
	ctx             context.Context
	greatest        string
	driver          string
	dsn             string
	withoutPing     bool
	maxRows         int
	maxIdleCount    *int
	maxOpenConns    *int
	connMaxLifetime *time.Duration
	connMaxIdletime *time.Duration
	err             error
}

type Option func(*Options)

func (p *Options) Validate() error {
	if p.maxRows == 0 {
		p.maxRows = DefaultMaxRows
	}
	return p.err
}

func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.ctx = ctx
	}
}

func WithoutPing() Option {
	return func(o *Options) {
		o.withoutPing = true
	}
}

func WithDirver(driver string) Option {
	return func(o *Options) {
		o.driver = driver
		switch driver {
		case "sqlite3":
			o.greatest = "max"
		default: // mysql
			o.greatest = "greatest"
		}
	}
}

func WithMaxRows(n int) Option {
	return func(o *Options) {
		o.maxRows = n
	}
}

func WithDsn(dsn string) Option {
	return func(o *Options) {
		o.dsn = dsn
	}
}

func WithMaxIdleCount(n int) Option {
	return func(o *Options) {
		o.maxIdleCount = &n
	}
}
func WithMxaOpenConns(n int) Option {
	return func(o *Options) {
		o.maxOpenConns = &n
	}
}
func WithConnMaxLifetime(d time.Duration) Option {
	return func(o *Options) {
		o.connMaxLifetime = &d
	}
}
func WithConnMaxIdletime(d time.Duration) Option {
	return func(o *Options) {
		o.connMaxIdletime = &d
	}
}

type SqlOptions struct {
	table  string
	offset *int64
	limit  *int64
	cols   []string
	sample interface{}
}

type SqlOption func(*SqlOptions)

func WithTable(table string) SqlOption {
	return func(o *SqlOptions) {
		o.table = table
	}
}

func WithLimit(offset, limit int64) SqlOption {
	return func(o *SqlOptions) {
		o.offset = &offset
		o.limit = &limit
	}
}
func WithCols(cols ...string) SqlOption {
	return func(o *SqlOptions) {
		o.cols = cols
	}
}

func WithSample(sample interface{}) SqlOption {
	return func(o *SqlOptions) {
		o.sample = sample
	}
}
func (p *SqlOptions) GenUpdateSql() (string, []interface{}, error) {
	table := p.table
	if table == "" {
		table = snakeCasedName(reflect.TypeOf(p.sample).Name())
	}

	return GenUpdateSql(table, p.sample)
}

func (p *SqlOptions) GenInsertSql() (string, []interface{}, error) {
	table := p.table
	if table == "" {
		table = snakeCasedName(reflect.TypeOf(p.sample).Name())
	}

	return GenInsertSql(table, p.sample)
}
