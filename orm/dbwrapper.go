package orm

import (
	"database/sql"
	"fmt"

	"github.com/yubo/golib/api/errors"
	"k8s.io/klog/v2"
)

var _ DBWrapper = &dbWrapper{}

type dbWrapper struct {
	*Options
	rawDB
}

func (p *dbWrapper) Exec(sql string, args ...interface{}) (sql.Result, error) {
	dlogSql(2, sql, args...)

	ret, err := p.rawDB.Exec(sql, args...)
	if err != nil {
		klog.V(3).Info(1, err)
		return nil, fmt.Errorf("Exec() err: %s", err)
	}

	return ret, nil
}

func (p *dbWrapper) ExecLastId(sql string, args ...interface{}) (int64, error) {
	dlogSql(2, sql, args...)

	res, err := p.Exec(sql, args...)
	if err != nil {
		klog.InfoDepth(1, err)
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if ret, err := res.LastInsertId(); err != nil {
		dlogSql(2, "%v", err)
		return 0, fmt.Errorf("LastInsertId() err: %s", err)
	} else {
		return ret, nil
	}

}

func (p *dbWrapper) execNum(sql string, args ...interface{}) (int64, error) {
	res, err := p.Exec(sql, args...)
	if err != nil {
		dlogSql(2, "%v", err)
		return 0, fmt.Errorf("Exec() err: %s", err)
	}

	if ret, err := res.RowsAffected(); err != nil {
		dlogSql(2, "%v", err)
		return 0, fmt.Errorf("RowsAffected() err: %s", err)
	} else {
		return ret, nil
	}
}

func (p *dbWrapper) ExecNum(sql string, args ...interface{}) (int64, error) {
	dlogSql(2, sql, args...)
	return p.execNum(sql, args...)
}

func (p *dbWrapper) ExecNumErr(s string, args ...interface{}) error {
	dlogSql(2, s, args...)
	if n, err := p.execNum(s, args...); err != nil {
		return err
	} else if n == 0 {
		return errors.NewNotFound("rows")
	} else {
		return nil
	}
}

func (p *dbWrapper) Query(query string, args ...interface{}) *Rows {
	dlogSql(2, query, args...)
	ret := &Rows{
		db:      p,
		query:   query,
		args:    args,
		maxRows: p.maxRows,
	}
	ret.rows, ret.err = p.rawDB.Query(query, args...)
	return ret
}

func (p *dbWrapper) Update(sample interface{}, opts ...SqlOption) error {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(sample)) {
		opt(o)
	}

	sql, args, err := o.GenUpdateSql()
	if err != nil {
		dlog(2, "%v", err)
		return err
	}

	dlogSql(2, sql, args...)
	_, err = p.rawDB.Exec(sql, args...)
	if err != nil {
		dlog(2, "%v", err)
	}
	return err
}

func (p *dbWrapper) Insert(sample interface{}, opts ...SqlOption) error {
	_, err := p.insert(sample, opts...)
	return err
}

func (p *dbWrapper) insert(sample interface{}, opts ...SqlOption) (sql.Result, error) {
	o := &SqlOptions{}
	for _, opt := range append(opts, WithSample(sample)) {
		opt(o)
	}

	sql, args, err := o.GenInsertSql()
	if err != nil {
		return nil, err
	}

	dlogSql(3, sql, args...)

	return p.rawDB.Exec(sql, args...)
}

func (p *dbWrapper) InsertLastId(sample interface{}, opts ...SqlOption) (int64, error) {
	res, err := p.insert(sample, opts...)
	if err != nil {
		dlog(2, "%v", err)
		return 0, err
	}

	if ret, err := res.LastInsertId(); err != nil {
		dlog(2, "%v", err)
		return 0, fmt.Errorf("LastInsertId() err: %s", err)
	} else {
		return ret, nil
	}
}
