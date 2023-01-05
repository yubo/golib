package orm

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yubo/golib/api/errors"
)

var _ Interface = &baseInterface{}

func NewBaseInterface(driver Driver, db RawDB, opts *DBOptions) Interface {
	if driver == nil {
		driver = &nonDriver{}
	}
	return &baseInterface{opts, driver, db}
}

type baseInterface struct {
	*DBOptions
	Driver
	db RawDB
}

func (p *baseInterface) RawDB() RawDB {
	return p.db
}

func (p *baseInterface) WithRawDB(raw RawDB) Interface {
	return NewBaseInterface(p.Driver, raw, p.DBOptions)
}

func (p *baseInterface) rawDBFrom(ctx context.Context) RawDB {
	if i, ok := ctx.Value(interfaceKey).(Interface); ok {
		return i.RawDB()
	}
	return p.RawDB()
}

func (p *baseInterface) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	dlogSql(query, args...)

	//ret, err := p.db.Exec(ctx, query, args...)
	ret, err := p.rawDBFrom(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("Exec() err: %s", err)
	}

	return ret, nil
}

func (p *baseInterface) ExecLastId(ctx context.Context, query string, args ...interface{}) (int64, error) {
	dlogSql(query, args...)
	return p.execLastId(ctx, query, args...)
}

func (p *baseInterface) execLastId(ctx context.Context, query string, args ...interface{}) (int64, error) {
	//res, err := p.db.Exec(ctx, query, args...)
	res, err := p.rawDBFrom(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if id, err := res.LastInsertId(); err != nil {
		return 0, fmt.Errorf("LastInsertId() err: %s", err)
	} else {
		return id, nil
	}
}

func (p *baseInterface) ExecNum(ctx context.Context, query string, args ...interface{}) (int64, error) {
	dlogSql(query, args...)
	return p.execNum(ctx, query, args...)
}

func (p *baseInterface) execNum(ctx context.Context, query string, args ...interface{}) (int64, error) {
	//res, err := p.db.Exec(ctx, query, args...)
	res, err := p.rawDBFrom(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return 0, fmt.Errorf("RowsAffected() err: %s", err)
	} else {
		return n, nil
	}
}

func (p *baseInterface) ExecNumErr(ctx context.Context, query string, args ...interface{}) error {
	dlogSql(query, args...)
	return p.execNumErr(ctx, query, args...)
}

func (p *baseInterface) execNumErr(ctx context.Context, query string, args ...interface{}) error {
	if n, err := p.execNum(ctx, query, args...); err != nil {
		return err
	} else if n == 0 {
		return errors.NewNotFound("object")
	} else {
		return nil
	}
}

func (p *baseInterface) Query(ctx context.Context, query string, args ...interface{}) *Rows {
	dlogSql(query, args...)
	return p.query(ctx, query, args...)
}

func (p *baseInterface) query(ctx context.Context, query string, args ...interface{}) *Rows {
	ret := &Rows{
		db:        p,
		DBOptions: p.DBOptions,
		query:     query,
		args:      args,
		maxRows:   p.maxRows,
	}
	ret.rows, ret.err = p.rawDBFrom(ctx).QueryContext(ctx, query, args...)
	return ret
}

func (p *baseInterface) Insert(ctx context.Context, sample interface{}, opts ...Option) error {
	o, err := NewOptions(append(opts, WithSample(sample))...)
	if err != nil {
		return err
	}

	query, args, err := o.GenInsertSql(p)
	if err != nil {
		return err
	}

	dlogSql(query, args...)
	return p.execNumErr(ctx, query, args...)
}

func (p *baseInterface) InsertLastId(ctx context.Context, sample interface{}, opts ...Option) (int64, error) {
	o, err := NewOptions(append(opts, WithSample(sample))...)
	if err != nil {
		return 0, err
	}

	query, args, err := o.GenInsertSql(p)
	if err != nil {
		return 0, err
	}

	dlogSql(query, args...)
	return p.execLastId(ctx, query, args...)
}

func (p *baseInterface) List(ctx context.Context, into interface{}, opts ...Option) error {
	o, err := NewOptions(append(opts, WithSample(into))...)
	if err != nil {
		return err
	}

	if o.table == "" {
		o.table = typeOfArray(into)
	}

	querySql, countSql, args, err := o.GenListSql()
	if err != nil {
		return err
	}

	dlogSql(querySql, args...)
	if err := p.query(ctx, querySql, args...).Rows(into); err != nil {
		return err
	}

	if o.total != nil {
		dlogSql(countSql, args...)
		if err := p.query(ctx, countSql, args...).Row(o.total); err != nil {
			return err
		}
	}

	return nil
}

func (p *baseInterface) Get(ctx context.Context, into interface{}, opts ...Option) error {
	o, err := NewOptions(append(opts, WithSample(into))...)
	if err != nil {
		return err
	}

	query, args, err := o.GenGetSql()
	if err != nil {
		return err
	}

	dlogSql(query, args...)
	return o.Error(p.query(ctx, query, args...).Row(into))
}

func (p *baseInterface) Update(ctx context.Context, sample interface{}, opts ...Option) error {
	o, err := NewOptions(append(opts, WithSample(sample))...)
	if err != nil {
		return err
	}

	query, args, err := o.GenUpdateSql(p)
	if err != nil {
		return err
	}

	dlogSql(query, args...)
	return o.Error(p.execNumErr(ctx, query, args...))
}

func (p *baseInterface) Delete(ctx context.Context, sample interface{}, opts ...Option) error {
	o, err := NewOptions(append(opts, WithSample(sample))...)
	if err != nil {
		return err
	}

	query, args, err := o.GenDeleteSql()
	if err != nil {
		return err
	}

	dlogSql(query, args...)
	return o.Error(p.execNumErr(ctx, query, args...))
}
