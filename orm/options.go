package orm

import (
	"context"
	"reflect"
	"time"

	"github.com/yubo/golib/api/errors"
	"github.com/yubo/golib/queries"
	"github.com/yubo/golib/util"
)

const (
	DefaultMaxRows = 1000
)

type Options struct {
	ctx context.Context
	//greatest        string
	driver          string
	dsn             string
	ignoreNotFound  bool
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

func WithIgnoreNotFound() Option {
	return func(o *Options) {
		o.ignoreNotFound = true
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

func WithMaxOpenConns(n int) Option {
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
	err            error
	sample         interface{}
	total          *int64
	table          string
	selector       queries.Selector
	cols           []string
	orderby        []string
	offset         *int64
	limit          *int64
	ignoreNotFound bool
}

func (o *SqlOptions) Error(err error) error {
	if err != nil && o.ignoreNotFound && errors.IsNotFound(err) {
		return nil
	}

	return err
}

func (o *SqlOptions) Sample() interface{} {
	return o.sample
}

type SqlOption func(*SqlOptions)

func WithTable(table string) SqlOption {
	return func(o *SqlOptions) {
		o.table = table
	}
}

func WithIgnoreNotFoundErr() SqlOption {
	return func(o *SqlOptions) {
		o.ignoreNotFound = true
	}
}

func WithTotal(total *int64) SqlOption {
	return func(o *SqlOptions) {
		o.total = total
	}
}

func WithSelector(selector string) SqlOption {
	return func(o *SqlOptions) {
		o.selector, o.err = queries.Parse(selector)
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

func WithOrderby(orderby ...string) SqlOption {
	return func(o *SqlOptions) {
		o.orderby = orderby
	}
}

func WithSample(sample interface{}) SqlOption {
	return func(o *SqlOptions) {
		o.sample = sample
	}
}

func (p *SqlOptions) Table() string {
	if p.table != "" {
		return p.table
	}

	rt := reflect.TypeOf(p.sample)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	p.table = util.SnakeCasedName(rt.Name())
	return p.table
}

func (p *SqlOptions) GenListSql() (query, countQuery string, args []interface{}, err error) {
	return GenListSql(p.Table(), p.cols, p.selector, p.orderby, p.offset, p.limit)
}

func (p *SqlOptions) GenGetSql() (string, []interface{}, error) {
	return GenGetSql(p.Table(), p.cols, p.selector)
}

func (p *SqlOptions) GenUpdateSql(db Driver) (string, []interface{}, error) {
	return GenUpdateSql(p.Table(), p.sample, db)
}

// TODO: generate selector from sample.fields, like GenUpdateSql
func (p *SqlOptions) GenDeleteSql() (string, []interface{}, error) {
	return GenDeleteSql(p.Table(), p.selector)
}

func (p *SqlOptions) GenInsertSql(db Driver) (string, []interface{}, error) {
	return GenInsertSql(p.Table(), p.sample, db)
}
