package orm

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yubo/golib/api/errors"
	"github.com/yubo/golib/util"
)

type DBOptions struct {
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
	stringSize      int
	err             error
}

func NewDefaultDBOptions() *DBOptions {
	return &DBOptions{
		maxRows:    1000,
		stringSize: 255,
	}
}

type DBOption func(*DBOptions)

func (p *DBOptions) Validate() error {
	return p.err
}

func WithContext(ctx context.Context) DBOption {
	return func(o *DBOptions) {
		o.ctx = ctx
	}
}

func WithIgnoreNotFound() DBOption {
	return func(o *DBOptions) {
		o.ignoreNotFound = true
	}
}

func WithoutPing() DBOption {
	return func(o *DBOptions) {
		o.withoutPing = true
	}
}

func WithDirver(driver string) DBOption {
	return func(o *DBOptions) {
		o.driver = driver
	}
}

func WithMaxRows(n int) DBOption {
	return func(o *DBOptions) {
		o.maxRows = n
	}
}

func WithDsn(dsn string) DBOption {
	return func(o *DBOptions) {
		o.dsn = dsn
	}
}

func WithMaxIdleCount(n int) DBOption {
	return func(o *DBOptions) {
		o.maxIdleCount = &n
	}
}

func WithMaxOpenConns(n int) DBOption {
	return func(o *DBOptions) {
		o.maxOpenConns = &n
	}
}

func WithConnMaxLifetime(d time.Duration) DBOption {
	return func(o *DBOptions) {
		o.connMaxLifetime = &d
	}
}

func WithConnMaxIdletime(d time.Duration) DBOption {
	return func(o *DBOptions) {
		o.connMaxIdletime = &d
	}
}

func NewOptions(opts ...QueryOption) (*queryOptions, error) {
	o := &queryOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	if o.err != nil {
		return nil, o.err
	}

	if len(o.selectors) > 0 {
		selector, err := Parse(strings.Join(o.selectors, ","))
		if err != nil {
			return nil, err
		}

		o._selector = selector
	}

	return o, nil
}

type queryOptions struct {
	sample         interface{}
	total          *int
	table          string
	tableOptions   []string
	selectors      []string
	cols           []string
	orderby        []string
	offset         int
	limit          int
	ignoreNotFound bool
	sqlout         *string // dump sql into

	err       error
	_selector Selector
}

func (o *queryOptions) Error(err error) error {
	if err != nil && o.ignoreNotFound && errors.IsNotFound(err) {
		return nil
	}

	return err
}

func (o *queryOptions) Sample() interface{} {
	return o.sample
}

type QueryOption func(*queryOptions)

func WithTable(table string) QueryOption {
	return func(o *queryOptions) {
		o.table = table
	}
}

// for automigrate, add to `create table xxx () {tableoptions}`
func WithTableOptions(options ...string) QueryOption {
	return func(o *queryOptions) {
		o.tableOptions = append(o.tableOptions, options...)
	}
}

func WithIgnoreNotFoundErr(ignoreNotFound bool) QueryOption {
	return func(o *queryOptions) {
		o.ignoreNotFound = ignoreNotFound
	}
}

func WithTotal(total *int) QueryOption {
	return func(o *queryOptions) {
		o.total = total
	}
}

func WithSqlOutput(output *string) QueryOption {
	return func(o *queryOptions) {
		o.sqlout = output
	}
}

// WithSelector: use selector generate sql
// examples:
//   "user_name != tom, id < 10" --> "`user_name` != ? and `id` < ?"
//   "user_name in (tom, jerry)" --> "user_name in (?, ?)"
//   "user_name notin (tom, jerry)" --> "user_name not in (?, ?)"
//   "name ~= a" --> "`name` like '%?'"
//   "name ~ a" --> "`name` like '%?%'"
//   "name =~ a" --> "`name` like '?%'"
// operator:
//   =, ==, in, !=, notin, >, <, ~=, =~, ~
func WithSelector(selector ...string) QueryOption {
	return func(o *queryOptions) {
		for _, v := range selector {
			if v := strings.TrimSpace(v); len(v) > 0 {
				o.selectors = append(o.selectors, v)
			}
		}
	}
}

func WithSelectorf(format string, args ...interface{}) QueryOption {
	for i := range args {
		switch v := args[i].(type) {
		case []byte:
			continue
		default:
			switch rv := reflect.ValueOf(v); rv.Kind() {
			case reflect.Slice, reflect.Array:
				buf := bytes.NewBufferString("")
				for i := 0; i < rv.Len(); i++ {
					if i > 0 {
						buf.WriteByte(',')
					}
					fmt.Fprintf(buf, "%v", rv.Index(i).Interface())
				}
				args[i] = buf.String()
			}
		}
	}
	return WithSelector(fmt.Sprintf(format, args...))
}

func WithLimit(offset, limit int) QueryOption {
	return func(o *queryOptions) {
		o.offset = offset
		o.limit = limit
	}
}

func WithCols(cols ...string) QueryOption {
	return func(o *queryOptions) {
		o.cols = cols
	}
}

func WithOrderby(orderby ...string) QueryOption {
	return func(o *queryOptions) {
		o.orderby = orderby
	}
}

func WithSample(sample interface{}) QueryOption {
	return func(o *queryOptions) {
		o.sample = sample
	}
}

type Namer interface {
	Name() string
}

func (p *queryOptions) Table() string {
	if p.table != "" {
		return p.table
	}

	var tableName string
	if n, ok := p.sample.(Namer); ok {
		tableName = n.Name()
	} else {
		rt := reflect.TypeOf(p.sample)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}

		tableName = rt.Name()
	}

	p.table = util.SnakeCasedName(tableName)

	return p.table
}

func (p *queryOptions) GenListSql() (query, countQuery string, args []interface{}, err error) {
	return GenListSql(p.Table(), p.cols, p._selector, p.orderby, p.offset, p.limit)
}

func (p *queryOptions) GenGetSql() (string, []interface{}, error) {
	return GenGetSql(p.Table(), p.cols, p._selector)
}

func (p *queryOptions) GenUpdateSql(db Driver) (string, []interface{}, error) {
	return GenUpdateSql(p.Table(), p.sample, db, p._selector)
}

// TODO: generate selector from sample.fields, like GenUpdateSql
func (p *queryOptions) GenDeleteSql() (string, []interface{}, error) {
	return GenDeleteSql(p.Table(), p._selector)
}

func (p *queryOptions) GenInsertSql(db Driver) (string, []interface{}, error) {
	return GenInsertSql(p.Table(), p.sample, db)
}
