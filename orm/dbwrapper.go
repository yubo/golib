package orm

import (
	"database/sql"
	"fmt"

	"github.com/yubo/golib/api/errors"
	"k8s.io/klog/v2"
)

var _ Interface = &dbWrapper{}

type dbWrapper struct {
	*Options
	rawDB
}

func (p *dbWrapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	dlogSql(2, query, args...)

	ret, err := p.rawDB.Exec(query, args...)
	if err != nil {
		klog.V(3).Info(1, err)
		return nil, fmt.Errorf("Exec() err: %s", err)
	}

	return ret, nil
}

func (p *dbWrapper) ExecLastId(query string, args ...interface{}) (int64, error) {
	dlogSql(2, query, args...)
	return p.execLastId(query, args...)
}

func (p *dbWrapper) execLastId(query string, args ...interface{}) (int64, error) {
	res, err := p.rawDB.Exec(query, args...)
	if err != nil {
		klog.InfoDepth(1, err)
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if id, err := res.LastInsertId(); err != nil {
		return 0, fmt.Errorf("LastInsertId() err: %s", err)
	} else {
		return id, nil
	}
}

func (p *dbWrapper) ExecNum(query string, args ...interface{}) (int64, error) {
	dlogSql(2, query, args...)
	return p.execNum(query, args...)
}

func (p *dbWrapper) execNum(query string, args ...interface{}) (int64, error) {
	res, err := p.rawDB.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if n, err := res.RowsAffected(); err != nil {
		return 0, fmt.Errorf("RowsAffected() err: %s", err)
	} else {
		return n, nil
	}
}

func (p *dbWrapper) ExecNumErr(query string, args ...interface{}) error {
	dlogSql(2, query, args...)
	return p.execNumErr(query, args...)
}

func (p *dbWrapper) execNumErr(query string, args ...interface{}) error {
	dlogSql(2, query, args...)
	if n, err := p.execNum(query, args...); err != nil {
		return err
	} else if n == 0 {
		return errors.NewNotFound("object")
	} else {
		return nil
	}
}

func (p *dbWrapper) Query(query string, args ...interface{}) *Rows {
	dlogSql(2, query, args...)
	return p.query(query, args...)
}

func (p *dbWrapper) query(query string, args ...interface{}) *Rows {
	ret := &Rows{
		db:      p,
		Options: p.Options,
		query:   query,
		args:    args,
		maxRows: p.maxRows,
	}
	ret.rows, ret.err = p.rawDB.Query(query, args...)
	return ret
}

func (p *dbWrapper) Insert(sample interface{}, opts ...SqlOption) error {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(sample)) {
		opt(o)
	}

	query, args, err := o.GenInsertSql()
	if err != nil {
		dlog(2, "%v", err)
		return err
	}

	dlogSql(2, query, args...)

	return p.execNumErr(query, args...)
}

func (p *dbWrapper) InsertLastId(sample interface{}, opts ...SqlOption) (int64, error) {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(sample)) {
		opt(o)
	}

	query, args, err := o.GenInsertSql()
	if err != nil {
		dlog(2, "%v", err)
		return 0, err
	}

	dlogSql(2, query, args...)

	return p.execLastId(query, args...)
}

func (p *dbWrapper) List(into interface{}, opts ...SqlOption) error {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(into)) {
		opt(o)
	}

	query, count, args, err := o.GenListSql()
	if err != nil {
		dlog(2, "%v", err)
		return err
	}

	dlogSql(2, query, args...)
	if err := p.query(query, args).Rows(into); err != nil {
		return err
	}

	if o.total != nil {
		if err := p.query(count, args).Row(o.total); err != nil {
			return err
		}
	}

	return nil
}

func (p *dbWrapper) Get(into interface{}, opts ...SqlOption) error {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(into)) {
		opt(o)
	}

	query, args, err := o.GenGetSql()
	if err != nil {
		dlog(2, "%v", err)
		return err
	}

	dlogSql(2, query, args...)

	return o.Error(p.query(query, args...).Row(into))
}

func (p *dbWrapper) Update(sample interface{}, opts ...SqlOption) error {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(sample)) {
		opt(o)
	}

	query, args, err := o.GenUpdateSql()
	if err != nil {
		dlog(2, "%v", err)
		return err
	}

	dlogSql(2, query, args...)

	return o.Error(p.execNumErr(query, args...))
}

func (p *dbWrapper) Delete(sample interface{}, opts ...SqlOption) error {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(sample)) {
		opt(o)
	}

	query, args, err := o.GenDeleteSql()
	if err != nil {
		dlog(2, "%v", err)
		return err
	}

	dlogSql(2, query, args...)

	return o.Error(p.execNumErr(query, args...))
}
