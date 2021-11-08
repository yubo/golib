package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yubo/golib/api/errors"
	"k8s.io/klog/v2"
)

type DB interface {
	RawDB() *sql.DB
	Close() error
	Begin() (Tx, error)
	BeginTx(ctx context.Context, ops *sql.TxOptions) (Tx, error)
	ExecRows(bytes []byte) error // like mysql < a.sql

	DBWrapper
}

type Tx interface {
	Tx() *sql.Tx
	Rollback() error
	Commit() error

	DBWrapper
}

type rawDB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type DBWrapper interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecLastId(sql string, args ...interface{}) (int64, error)
	ExecNum(sql string, args ...interface{}) (int64, error)
	ExecNumErr(s string, args ...interface{}) error
	Query(query string, args ...interface{}) *Rows
	Update(sample interface{}, opts ...SqlOption) error
	Insert(sample interface{}, opts ...SqlOption) error
	InsertLastId(sample interface{}, opts ...SqlOption) (int64, error)
}

type ormDB struct {
	*Options
	db *sql.DB // DB

	dbWrapper
}

func Open(driverName, dataSourceName string, opts ...Option) (DB, error) {
	o := &Options{}
	for _, opt := range append(opts, WithDirver(driverName), WithDsn(dataSourceName)) {
		opt(o)
	}

	if err := o.Validate(); err != nil {
		return nil, err
	}

	return open(o)

}

func open(opts *Options) (DB, error) {
	db, err := sql.Open(opts.driver, opts.dsn)
	if err != nil {
		return nil, err
	}

	if !opts.withoutPing {
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, err
		}
	}

	if opts.ctx != nil {
		go func() {
			<-opts.ctx.Done()
			db.Close()
		}()
	}

	if opts.maxIdleCount != nil {
		db.SetMaxIdleConns(*opts.maxIdleCount)
	}
	if opts.maxOpenConns != nil {
		db.SetMaxOpenConns(*opts.maxOpenConns)
	}
	if opts.connMaxLifetime != nil {
		db.SetConnMaxLifetime(*opts.connMaxLifetime)
	}
	if opts.connMaxIdletime != nil {
		db.SetConnMaxIdleTime(*opts.connMaxIdletime)
	}

	return &ormDB{
		Options: opts,
		db:      db,
		dbWrapper: dbWrapper{
			Options: opts,
			rawDB:   db,
		},
	}, nil
}

func (p *ormDB) RawDB() *sql.DB {
	return p.db
}

func (p *ormDB) Close() error {
	return p.db.Close()
}

func (p *ormDB) Begin() (Tx, error) {
	return p.BeginTx(context.Background(), nil)
}

func (p *ormDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := p.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &ormTx{tx: tx,
		dbWrapper: dbWrapper{
			Options: p.Options,
			rawDB:   tx,
		},
	}, nil
}

func (p *ormDB) ExecRows(bytes []byte) (err error) {
	var cmds []string
	var tx *sql.Tx

	if tx, err = p.db.Begin(); err != nil {
		return fmt.Errorf("Begin() err: %s", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	lines := strings.Split(string(bytes), "\n")
	for cmd, in, i := "", false, 0; i < len(lines); i++ {
		line := lines[i]
		if len(line) == 0 || strings.HasPrefix(line, "-- ") {
			continue
		}

		if in {
			cmd += " " + strings.TrimSpace(line)
			if cmd[len(cmd)-1] == ';' {
				cmds = append(cmds, cmd)
				in = false
			}
		} else {
			n := strings.Index(line, " ")
			if n <= 0 {
				continue
			}

			switch line[:n] {
			case "SET", "CREATE", "INSERT", "DROP":
				cmd = line
				if line[len(line)-1] == ';' {
					cmds = append(cmds, cmd)
				} else {
					in = true
				}
			}
		}
	}

	for i := 0; i < len(cmds); i++ {
		_, err := tx.Exec(cmds[i])
		if err != nil {
			klog.V(3).Infof("%v", err)
			return fmt.Errorf("sql %s\nerr %s", cmds[i], err)
		}
	}
	return nil
}

type ormTx struct {
	*Options
	tx *sql.Tx

	dbWrapper
}

func (p *ormTx) Tx() *sql.Tx {
	return p.tx
}

func (p *ormTx) Rollback() error {
	return p.tx.Rollback()
}

func (p *ormTx) Commit() error {
	return p.tx.Commit()
}

type Rows struct {
	*Options
	db    DBWrapper
	query string
	args  []interface{}
	count *int64

	maxRows int
	rows    *sql.Rows
	b       *binder
	err     error
}

// Row(*int, *int, ...)
// Row(*struct{})
// Row(**struct{})
func (p *Rows) Row(dst ...interface{}) error {
	if p.err != nil {
		return p.err
	}
	defer p.rows.Close()

	if p.rows.Next() {
		if len(dst) == 1 && isStructMode(dst[0]) {
			return p.scanRow(dst[0])
		}

		return p.rows.Scan(dst...)
	}

	if !p.ignoreNotFound {
		return errors.NewNotFound("rows")
	}

	return nil
}

// scanRow scan row result into dst struct
// dst must be struct, should be prechecked by isStructMode()
func (p *Rows) scanRow(dst interface{}) error {
	row := reflect.Indirect(reflect.ValueOf(dst))

	if !row.CanSet() {
		return fmt.Errorf("scan target can not be set")
	}

	b, err := p.genBinder(row.Type())
	if err != nil {
		return err
	}

	if err := b.scan(row); err != nil {
		return fmt.Errorf("rows.scan() err: %s", err)
	}

	return nil
}

func (p *Rows) Count(count *int64) *Rows {
	p.count = count
	return p
}

// Rows([]struct{})
// Rows([]*struct{})
// Rows(*[]struct{})
// Rows(*[]*struct{})
// Rows([]string)
// Rows([]*string)
// Rows ignore notfound err msg
func (p *Rows) Rows(dst interface{}) error {
	if p.err != nil {
		return p.err
	}
	defer p.rows.Close()

	if p.count != nil {
		countQuery, err := genCountQuery(p.query)
		if err != nil {
			return err
		}
		err = p.db.Query(countQuery, p.args...).Row(p.count)
		if err != nil {
			return err
		}
	}

	limit := p.maxRows

	rv, err := rowsInputValue(dst)
	if err != nil {
		return err
	}

	// sample is slice elem type
	sample := rv.Type().Elem()
	n := 0

	if !isStructMode(reflect.New(sample).Interface()) {
		// e.g. []string or []*string
		for p.rows.Next() {
			row := reflect.New(sample).Elem()

			if err := p.rows.Scan(row.Addr().Interface()); err != nil {
				return fmt.Errorf("rows.scan() err: %s", err)
			}

			rv.Set(reflect.Append(rv, row))

			if n += 1; n >= limit {
				break
			}
		}
		return nil
	}

	// elem is struct
	b, err := p.genBinder(reflect.New(sample).Elem().Type())
	if err != nil {
		return err
	}

	for p.rows.Next() {
		row := reflect.New(sample).Elem()
		b.scan(row)
		rv.Set(reflect.Append(rv, row))

		if n += 1; n >= limit {
			break
		}
	}
	return nil
}

func rowsInputValue(sample interface{}) (rv reflect.Value, err error) {
	rv = reflect.Indirect(reflect.ValueOf(sample))

	if !rv.CanSet() {
		return rv, fmt.Errorf("scan target can not be set")
	}

	// for *[]struct{}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return rv, fmt.Errorf("needs a pointer to a slice")
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Slice {
		return rv, fmt.Errorf("needs a pointer to a slice")
	}

	return rv, nil
}

func (p *Rows) genBinder(rt reflect.Type) (*binder, error) {
	if p.rows == nil {
		return nil, fmt.Errorf("rows is nil")
	}

	fields, err := p.rows.Columns()
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]int{}
	for i, name := range fields {
		fieldMap[name] = i
	}

	var empty interface{}
	dest := make([]interface{}, len(fields))
	for i := 0; i < len(dest); i++ {
		dest[i] = &empty
	}

	// klog.V(5).Infof("dest len %d", len(dest))
	return &binder{
		fields:   cachedTypeFields(rt),
		dest:     dest,
		fieldMap: fieldMap,
		rows:     p.rows,
	}, nil

}

type binder struct {
	fields   structFields
	dest     []interface{}
	fieldMap map[string]int
	rows     *sql.Rows
}

func (p binder) scan(sample reflect.Value) error {
	tran, err := p.bind(sample)
	if err != nil {
		return err
	}

	if err := p.rows.Scan(p.dest...); err != nil {
		return fmt.Errorf("Scan() err: %s", err)
	}

	for _, v := range tran {
		if err := v.unmarshal(); err != nil {
			return err
		}
	}

	return nil
}

type transfer struct {
	dstProxy interface{} // byte
	dst      interface{} // raw
	ptr      bool
}

// json -> dst
func (p *transfer) unmarshal() error {
	if p.dstProxy == nil {
		return nil
	}

	rv := reflect.Indirect(reflect.ValueOf(p.dst))
	if p.ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	// time.Time
	if i, ok := p.dstProxy.(int64); ok {
		t := time.Unix(i, 0)
		if dst, ok := rv.Addr().Interface().(*time.Time); ok {
			*dst = t
		}
		return nil
	}

	if jsonStr, ok := p.dstProxy.([]byte); ok {
		if err := json.Unmarshal(jsonStr, rv.Addr().Interface()); err != nil {
			dlog(2, "json.Unmarshal() error %s", err)
		}
	}

	return nil
}

func (p *binder) bind(rv reflect.Value) ([]*transfer, error) {
	tran := []*transfer{}
	for _, f := range p.fields.list {
		if i, ok := p.fieldMap[f.key]; ok {
			fv, err := getSubv(rv, f.index, true)
			if err != nil {
				return nil, err
			}
			if p.dest[i], err = scanInterface(fv, &tran); err != nil {
				return nil, err
			}
		}
	}

	return tran, nil
}

// sqlInterface: rv should not be ptr, return interface for use in sql's args
func sqlInterface(rv reflect.Value) (interface{}, error) {
	if rv.Type().String() == "time.Time" {
		return rv.Interface().(time.Time).Unix(), nil
	} else if rv.Kind() == reflect.Struct || rv.Kind() == reflect.Map ||
		(rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() != reflect.Uint8) {
		if b, err := json.Marshal(rv.Interface()); err != nil {
			return nil, err
		} else {
			return b, nil
		}
	}

	// if rv.Kind() == reflect.Ptr {
	// 	panicType(rv.Type(), "rv is ptr")
	// }

	return rv.Interface(), nil
}

// scanInterface input is struct's field
func scanInterface(rv reflect.Value, tran *[]*transfer) (interface{}, error) {
	rt := rv.Type()
	ptr := false

	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		ptr = true
	}

	if rt.Kind() == reflect.Struct || rt.Kind() == reflect.Map ||
		(rt.Kind() == reflect.Slice && rt.Elem().Kind() != reflect.Uint8) {
		//if rt.Kind() == reflect.Slice || rt.Kind() == reflect.Map || rt.Kind() == reflect.Struct {
		dst := rv.Addr().Interface()
		// json decode support *struct{}, but not **struct{}, so should adapt it
		node := &transfer{dst: dst, ptr: ptr}
		*tran = append(*tran, node)
		return &node.dstProxy, nil
	}

	return rv.Addr().Interface(), nil
}

func isNil(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

type RowsIter interface {
	Close() error
	Next() bool
	Row(dest ...interface{}) error
}

func (p *Rows) Iterator() (RowsIter, error) {
	if p.err != nil {
		return nil, p.err
	}

	return &rowsIterator{Rows: p}, nil
}

type rowsIterator struct {
	*Rows
}

func (p *rowsIterator) Close() error {
	return p.rows.Close()
}

func (p *rowsIterator) Next() bool {
	return p.rows.Next()
}

func (p *rowsIterator) Row(dst ...interface{}) error {
	if p.err != nil {
		return p.err
	}

	if len(dst) == 1 && isStructMode(dst[0]) {
		return p.Rows.scanRow(dst[0])
	}

	return p.rows.Scan(dst...)
}
