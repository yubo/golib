package orm

import (
	"database/sql"
	"fmt"

	"github.com/yubo/golib/api/errors"
)

var _ Interface = &baseInterface{}

func NewBaseInterface(driver Driver, db RawDB, opts *Options) Interface {
	if driver == nil {
		driver = &nonDriver{}
	}
	return &baseInterface{opts, driver, db}
}

type baseInterface struct {
	*Options
	Driver
	db RawDB
}

func (p *baseInterface) RawDB() RawDB {
	return p.db
}

func (p *baseInterface) WithRawDB(raw RawDB) Interface {
	return NewBaseInterface(p.Driver, raw, p.Options)
}

func (p *baseInterface) Exec(query string, args ...interface{}) (sql.Result, error) {
	dlogSql(1, query, args...)

	ret, err := p.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("Exec() err: %s", err)
	}

	return ret, nil
}

func (p *baseInterface) ExecLastId(query string, args ...interface{}) (int64, error) {
	dlogSql(1, query, args...)
	return p.execLastId(query, args...)
}

func (p *baseInterface) execLastId(query string, args ...interface{}) (int64, error) {
	res, err := p.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if id, err := res.LastInsertId(); err != nil {
		return 0, fmt.Errorf("LastInsertId() err: %s", err)
	} else {
		return id, nil
	}
}

func (p *baseInterface) ExecNum(query string, args ...interface{}) (int64, error) {
	dlogSql(1, query, args...)
	return p.execNum(query, args...)
}

func (p *baseInterface) execNum(query string, args ...interface{}) (int64, error) {
	res, err := p.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return 0, fmt.Errorf("RowsAffected() err: %s", err)
	} else {
		return n, nil
	}
}

func (p *baseInterface) ExecNumErr(query string, args ...interface{}) error {
	dlogSql(1, query, args...)
	return p.execNumErr(query, args...)
}

func (p *baseInterface) execNumErr(query string, args ...interface{}) error {
	if n, err := p.execNum(query, args...); err != nil {
		return err
	} else if n == 0 {
		return errors.NewNotFound("object")
	} else {
		return nil
	}
}

func (p *baseInterface) Query(query string, args ...interface{}) *Rows {
	dlogSql(1, query, args...)
	return p.query(query, args...)
}

func (p *baseInterface) query(query string, args ...interface{}) *Rows {
	ret := &Rows{
		db:      p,
		Options: p.Options,
		query:   query,
		args:    args,
		maxRows: p.maxRows,
	}
	ret.rows, ret.err = p.db.Query(query, args...)
	return ret
}

func (p *baseInterface) Insert(sample interface{}, opts ...SqlOption) error {
	o, err := NewSqlOptions(sample, opts)
	if err != nil {
		return err
	}

	query, args, err := o.GenInsertSql(p)
	if err != nil {
		return err
	}

	dlogSql(1, query, args...)
	return p.execNumErr(query, args...)
}

func (p *baseInterface) InsertLastId(sample interface{}, opts ...SqlOption) (int64, error) {
	o, err := NewSqlOptions(sample, opts)
	if err != nil {
		return 0, err
	}

	query, args, err := o.GenInsertSql(p)
	if err != nil {
		return 0, err
	}

	dlogSql(1, query, args...)
	return p.execLastId(query, args...)
}

func (p *baseInterface) List(into interface{}, opts ...SqlOption) error {
	o, err := NewSqlOptions(into, opts)
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

	dlogSql(1, querySql, args...)
	if err := p.query(querySql, args...).Rows(into); err != nil {
		return err
	}

	if o.total != nil {
		if err := p.query(countSql, args...).Row(o.total); err != nil {
			return err
		}
	}

	return nil
}

func (p *baseInterface) Get(into interface{}, opts ...SqlOption) error {
	o, err := NewSqlOptions(into, opts)
	if err != nil {
		return err
	}

	query, args, err := o.GenGetSql()
	if err != nil {
		return err
	}

	dlogSql(1, query, args...)
	return o.Error(p.query(query, args...).Row(into))
}

func (p *baseInterface) Update(sample interface{}, opts ...SqlOption) error {
	o, err := NewSqlOptions(sample, opts)
	if err != nil {
		return err
	}

	query, args, err := o.GenUpdateSql(p)
	if err != nil {
		return err
	}

	dlogSql(1, query, args...)
	return o.Error(p.execNumErr(query, args...))
}

func (p *baseInterface) Delete(sample interface{}, opts ...SqlOption) error {
	o, err := NewSqlOptions(sample, opts)
	if err != nil {
		return err
	}

	query, args, err := o.GenDeleteSql()
	if err != nil {
		return err
	}

	dlogSql(1, query, args...)
	return o.Error(p.execNumErr(query, args...))
}
