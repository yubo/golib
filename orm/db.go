package orm

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/yubo/golib/status"
	"google.golang.org/grpc/codes"
	"k8s.io/klog/v2"
)

const (
	MAX_ROWS = 1000
)

type db interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type Db struct {
	Greatest string
	Db       *sql.DB
	tx       *sql.Tx
	db       db
}

func printString(b []byte) string {
	s := make([]byte, len(b))

	for i := 0; i < len(b); i++ {
		if strconv.IsPrint(rune(b[i])) {
			s[i] = b[i]
		} else {
			s[i] = '.'
		}
	}
	return string(s)
}

func dlog(format string, args ...interface{}) {
	if klog.V(3).Enabled() {
		klog.InfoDepth(2, fmt.Sprintf(format, args...))
	}
}

func dlogSql(query string, args ...interface{}) {
	if klog.V(3).Enabled() {
		args2 := make([]interface{}, len(args))

		for i := 0; i < len(args2); i++ {
			rv := reflect.Indirect(reflect.ValueOf(args[i]))
			if rv.IsValid() && rv.CanInterface() {
				if b, ok := rv.Interface().([]byte); ok {
					args2[i] = printString(b)
				} else {
					args2[i] = rv.Interface()
				}
			}
		}
		klog.InfoDepth(2, "\n\t"+fmt.Sprintf(strings.Replace(query, "?", "`%v`", -1), args2...))
	}
}

func DbOpen(driverName, dataSourceName string) (*Db, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	ret := &Db{Db: db, db: db, Greatest: "greatest"}

	if driverName == "sqlite3" {
		ret.Greatest = "max"
	}

	return ret, nil
}

func DbOpenWithCtx(driverName, dsn string, ctx context.Context) (*Db, error) {

	db, err := DbOpen(driverName, dsn)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sql.Open() err: "+err.Error())
	}

	if err := db.Db.Ping(); err != nil {
		db.Db.Close()
		return nil, status.Errorf(codes.Internal, "db.Ping() err: "+err.Error())
	}

	go func() {
		<-ctx.Done()
		db.Db.Close()
	}()

	return db, nil
}

func (p *Db) Tx() bool {
	return p.tx != nil
}

func (p *Db) BeginWithCtx(ctx context.Context) (*Db, error) {
	if p.Tx() {
		return nil, status.Errorf(codes.Internal, "Already beginning a transaction")
	}
	if tx, err := p.Db.BeginTx(ctx, nil); err != nil {
		return nil, err
	} else {
		return &Db{tx: tx, db: tx, Greatest: p.Greatest}, nil
	}
}

func (p *Db) Rollback() error {
	if p.tx != nil {
		return p.tx.Rollback()
	}
	return status.Errorf(codes.Internal, "tx is nil")
}

func (p *Db) Commit() error {
	if p.tx != nil {
		return p.tx.Commit()
	}
	return status.Errorf(codes.Internal, "tx is nil")
}

func (p *Db) Begin() (*Db, error) {
	return p.BeginWithCtx(context.Background())
}

func (p *Db) SetConns(maxIdleConns, maxOpenConns int) {
	p.Db.SetMaxIdleConns(maxIdleConns)
	p.Db.SetMaxOpenConns(maxOpenConns)
}

func (p *Db) Close() {
	p.Db.Close()
}

func (p *Db) Query(query string, args ...interface{}) *Rows {
	dlogSql(query, args...)
	ret := &Rows{}
	ret.rows, ret.err = p.db.Query(query, args...)
	return ret
}

type Rows struct {
	rows *sql.Rows
	b    *binder
	err  error
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
			// klog.V(5).Infof("enter row scan struct")
			return p.scanRow(dst[0])
		}

		// klog.V(5).Infof("enter row scan")
		return p.rows.Scan(dst...)
	}
	return status.Errorf(codes.NotFound, "sql: no rows in result set")
}

// scanRow scan row result into dst struct
// dst must be struct, should be prechecked by isStructMode()
func (p *Rows) scanRow(dst interface{}) error {
	row := reflect.Indirect(reflect.ValueOf(dst))

	if !row.CanSet() {
		return status.Errorf(codes.InvalidArgument, "scan target can not be set")
	}

	b, err := p.genBinder(row.Type())
	if err != nil {
		return err
	}

	if err := b.scan(row); err != nil {
		return status.Errorf(codes.Internal, "Scan() err: "+err.Error())
	}

	return nil
}

// Rows([]struct{})
// Rows([]*struct{})
// Rows(*[]struct{})
// Rows(*[]*struct{})
// Rows([]string)
// Rows([]*string)
// Rows ignore notfound err msg
func (p *Rows) Rows(dst interface{}, opts ...int) error {
	if p.err != nil {
		return p.err
	}
	defer p.rows.Close()

	limit := MAX_ROWS
	if len(opts) > 0 && opts[0] > 0 {
		limit = opts[0]
	}

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
				return status.Errorf(codes.Internal, "Scan() err: "+err.Error())
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
		return rv, status.Errorf(codes.InvalidArgument, "scan target can not be set")
	}

	// for *[]struct{}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return rv, status.Errorf(codes.Internal, "needs a pointer to a slice")
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Slice {
		return rv, status.Errorf(codes.Internal, "needs a pointer to a slice")
	}

	return rv, nil
}

func (p *Db) Exec(sql string, args ...interface{}) (sql.Result, error) {
	dlogSql(sql, args...)

	ret, err := p.db.Exec(sql, args...)
	if err != nil {
		klog.V(3).Info(1, err)
		return nil, status.Errorf(codes.Internal, "Exec() err: "+err.Error())
	}

	return ret, nil
}

func (p *Db) ExecErr(sql string, args ...interface{}) error {
	dlogSql(sql, args...)

	_, err := p.db.Exec(sql, args...)
	if err != nil {
		klog.InfoDepth(1, err)
	}
	return err
}

func (p *Db) ExecLastId(sql string, args ...interface{}) (int64, error) {
	dlogSql(sql, args...)

	res, err := p.db.Exec(sql, args...)
	if err != nil {
		klog.InfoDepth(1, err)
		return 0, status.Errorf(codes.Internal, "Exec() err: "+err.Error())
	}

	if ret, err := res.LastInsertId(); err != nil {
		dlogSql("%v", err)
		return 0, status.Errorf(codes.Internal, "LastInsertId() err: "+err.Error())
	} else {
		return ret, nil
	}

}

func (p *Db) execNum(sql string, args ...interface{}) (int64, error) {
	res, err := p.db.Exec(sql, args...)
	if err != nil {
		dlogSql("%v", err)
		return 0, status.Errorf(codes.Internal, "Exec() err: "+err.Error())
	}

	if ret, err := res.RowsAffected(); err != nil {
		dlogSql("%v", err)
		return 0, status.Errorf(codes.Internal, "RowsAffected() err: "+err.Error())
	} else {
		return ret, nil
	}
}

func (p *Db) ExecNum(sql string, args ...interface{}) (int64, error) {
	dlogSql(sql, args...)
	return p.execNum(sql, args...)
}

func (p *Db) ExecNumErr(s string, args ...interface{}) error {
	dlogSql(s, args...)
	if n, err := p.execNum(s, args...); err != nil {
		return err
	} else if n == 0 {
		return status.Errorf(codes.NotFound, "no rows affected")
	} else {
		return nil
	}
}

func (p *Db) ExecRows(bytes []byte) (err error) {
	var (
		cmds []string
		tx   *sql.Tx
	)

	if tx, err = p.Db.Begin(); err != nil {
		return status.Errorf(codes.Internal, "Begin() err: "+err.Error())
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
			return status.Errorf(codes.Internal, "sql %s\nerr %s", cmds[i], err.Error())
		}
	}
	return nil
}

func (p *Db) Update(table string, sample interface{}) error {
	sql, args, err := GenUpdateSql(table, sample)
	if err != nil {
		dlog("%v", err)
		return err
	}

	dlogSql(sql, args...)
	_, err = p.db.Exec(sql, args...)
	if err != nil {
		dlog("%v", err)
	}
	return err
}

func (p *Db) Insert(table string, sample interface{}) error {
	sql, args, err := GenInsertSql(table, sample)
	if err != nil {
		return err
	}

	dlogSql(sql, args...)
	if _, err := p.db.Exec(sql, args...); err != nil {
		dlog("%v", err)
		return status.Errorf(codes.Internal,
			"Insert() err: "+err.Error())
	}
	return nil
}

func (p *Db) InsertLastId(table string, sample interface{}) (int64, error) {
	sql, args, err := GenInsertSql(table, sample)
	if err != nil {
		return 0, err
	}

	dlogSql(sql, args...)
	res, err := p.db.Exec(sql, args...)
	if err != nil {
		dlog("%v", err)
		return 0, status.Errorf(codes.Internal, "Exec() err: "+err.Error())
	}

	if ret, err := res.LastInsertId(); err != nil {
		dlog("%v", err)
		return 0, status.Errorf(codes.Internal, "LastInsertId() err: "+err.Error())
	} else {
		return ret, nil
	}
}

// utils
func snakeCasedName(name string) string {
	newstr := make([]rune, 0)
	firstTime := true

	for _, chr := range name {
		if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
			if firstTime == true {
				firstTime = false
			} else {
				newstr = append(newstr, '_')
			}
			chr -= ('A' - 'a')
		}
		newstr = append(newstr, chr)
	}

	return string(newstr)
}

// {1,2,3} => "(1,2,3)"
func Ints2sql(array []int64) string {
	out := bytes.NewBuffer([]byte("("))

	for i := 0; i < len(array); i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(out, "%d", array[i])
	}
	out.WriteByte(')')
	return out.String()
}

// {"1","2","3"} => "('1', '2', '3')"
func Strings2sql(array []string) string {
	out := bytes.NewBuffer([]byte("("))

	for i := 0; i < len(array); i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteByte('\'')
		out.Write([]byte(array[i]))
		out.WriteByte('\'')
	}
	out.WriteByte(')')
	return out.String()
}

// struct{}, *struct{}, **struct{} return true
func isStructMode(in interface{}) bool {
	rt := reflect.TypeOf(in)

	// depth 2
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	return rt.Kind() == reflect.Struct && rt.String() != "time.Time"
}

type kv struct {
	k string
	v interface{}
}

func GenUpdateSql(table string, sample interface{}) (string, []interface{}, error) {
	set := []kv{}
	where := []kv{}

	rv := reflect.Indirect(reflect.ValueOf(sample))

	if err := genUpdateSql(rv, &set, &where); err != nil {
		return "", nil, err
	}

	if len(set) == 0 {
		return "", nil, status.Errorf(codes.InvalidArgument, "update %s `set` is empty", table)
	}
	if len(where) == 0 {
		return "", nil, status.Errorf(codes.InvalidArgument, "update %s `where` is empty", table)
	}

	buf := &bytes.Buffer{}
	buf.WriteString("update " + table + " set ")

	args := []interface{}{}
	for i, v := range set {
		if i != 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(v.k + "=?")
		args = append(args, v.v)
	}

	buf.WriteString(" where ")
	for i, v := range where {
		if i != 0 {
			buf.WriteString(" and ")
		}
		buf.WriteString(v.k + "=?")
		args = append(args, v.v)
	}

	return buf.String(), args, nil
}

func genUpdateSql(rv reflect.Value, set, where *[]kv) error {
	fields := cachedTypeFields(rv.Type())
	for _, f := range fields.list {
		fv, err := getSubv(rv, f.index, false)
		if err != nil || isNil(fv) {
			continue
		}

		if fv.Kind() == reflect.Ptr {
			fv = fv.Elem()
		}

		if f.where {
			*where = append(*where, kv{f.key, fv.Interface()})
			continue
		}

		v, err := sqlInterface(fv)
		if err != nil {
			return err
		}
		*set = append(*set, kv{f.key, v})
	}
	return nil
}

func GenInsertSql(table string, sample interface{}) (string, []interface{}, error) {
	values := []kv{}

	rv := reflect.Indirect(reflect.ValueOf(sample))

	if err := genInsertSql(rv, &values); err != nil {
		return "", nil, err
	}

	if len(values) == 0 {
		return "", nil, status.Errorf(codes.InvalidArgument, "insert into %s `values` is empty", table)
	}

	buf := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	args := []interface{}{}

	buf.WriteString("insert into " + table + " (")

	for i, v := range values {
		if i != 0 {
			buf.WriteString(", ")
			buf2.WriteString(", ")
		}
		buf.WriteString("`" + v.k + "`")
		buf2.WriteString("?")
		args = append(args, v.v)
	}

	return buf.String() + ") values (" + buf2.String() + ")", args, nil
}

func genInsertSql(rv reflect.Value, values *[]kv) error {
	fields := cachedTypeFields(rv.Type())
	for _, f := range fields.list {
		fv, err := getSubv(rv, f.index, false)
		if err != nil || isNil(fv) {
			continue
		}

		if fv.Kind() == reflect.Ptr {
			fv = fv.Elem()
		}

		v, err := sqlInterface(fv)
		if err != nil {
			return err
		}
		*values = append(*values, kv{f.key, v})
	}
	return nil
}

func (p *Rows) genBinder(rt reflect.Type) (*binder, error) {
	if p.rows == nil {
		return nil, status.Errorf(codes.Internal, "rows is nil")
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
		return status.Errorf(codes.Internal, "Scan() err: "+err.Error())
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

	jsonStr, ok := p.dstProxy.([]byte)
	if !ok {
		return nil
	}

	rv := reflect.Indirect(reflect.ValueOf(p.dst))
	if p.ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if err := json.Unmarshal(jsonStr, rv.Addr().Interface()); err != nil {
		dlog("json.Unmarshal() error %s", err)
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
	if rv.Kind() == reflect.Struct || rv.Kind() == reflect.Map ||
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
