package orm

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/yubo/golib/queries"
	"k8s.io/klog/v2"
)

var (
	errSampleNil   = errors.New("input sample is nil")
	errTableEmpty  = errors.New("table name is not set")
	errSelectorNil = errors.New("selector is nil")
	errQueryEmpty  = errors.New("query is empty")
)

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

func dlog(depth int, format string, args ...interface{}) {
	if klog.V(6).Enabled() {
		klog.InfoDepth(depth, fmt.Sprintf(format, args...))
	}
}

func elog(depth int, format string, args ...interface{}) {
	klog.ErrorDepth(depth, fmt.Sprintf(format, args...))
}

func dlogSql(depth int, query string, args ...interface{}) {
	if klog.V(10).Enabled() {
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
		klog.InfoDepth(depth, "\n\t"+fmt.Sprintf(strings.Replace(query, "?", "`%v`", -1), args2...))
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
		out.Write([]byte("'" + array[i] + "'"))
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

func GenInsertSql(table string, sample interface{}) (string, []interface{}, error) {
	if sample == nil {
		return "", nil, errSampleNil
	}
	if table == "" {
		return "", nil, errTableEmpty
	}

	table = "`" + table + "`"
	values := []kv{}

	rv := reflect.Indirect(reflect.ValueOf(sample))

	if err := genInsertSql(rv, &values); err != nil {
		return "", nil, err
	}

	if len(values) == 0 {
		return "", nil, fmt.Errorf("insert into %s `values` is empty", table)
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

func GenListSql(table string, cols []string, selector queries.Selector, orderby []string, offset, limit *int64) (string, string, []interface{}, error) {
	if table == "" {
		return "", "", nil, errTableEmpty
	}

	table = "`" + table + "`"

	// select *
	buf := bytes.NewBufferString("select")
	// select count(*)
	buf2 := bytes.NewBufferString("select count(*) from " + table)
	args := []interface{}{}

	// cols
	if len(cols) == 0 {
		buf.WriteString(" *")
	} else {
		for i, col := range cols {
			if i != 0 {
				buf.WriteString(",")
			}
			buf.WriteString(" `" + col + "`")
		}
	}

	// table
	buf.WriteString(" from " + table)

	// selector
	if selector != nil {
		if q, a := selector.Sql(); q != "" {
			buf.WriteString(" where " + q)
			buf2.WriteString(" where " + q)
			args = a
		}
	}

	// order
	if len(orderby) > 0 {
		buf.WriteString(" order by " + strings.Join(orderby, ", "))
	}

	// limit
	if offset != nil && limit != nil {
		fmt.Fprintf(buf, " limit %d, %d", *offset, *limit)
	}

	return buf.String(), buf2.String(), args, nil
}

func GenGetSql(table string, cols []string, selector queries.Selector) (string, []interface{}, error) {
	if table == "" {
		return "", nil, errTableEmpty
	}
	if selector == nil {
		return "", nil, errSelectorNil
	}

	query, args := selector.Sql()
	if query == "" {
		return "", nil, errQueryEmpty
	}

	// select *
	buf := bytes.NewBufferString("select")

	// cols
	if len(cols) == 0 {
		buf.WriteString(" *")
	} else {
		for i, col := range cols {
			if i != 0 {
				buf.WriteString(",")
			}
			buf.WriteString(" `" + col + "`")
		}
	}

	// table
	buf.WriteString(" from `" + table + "` where " + query)

	return buf.String(), args, nil
}

func GenUpdateSql(table string, sample interface{}) (string, []interface{}, error) {
	if table == "" {
		return "", nil, errTableEmpty
	}
	if sample == nil {
		return "", nil, errSampleNil
	}

	table = "`" + table + "`"
	set := []kv{}
	where := []kv{}

	rv := reflect.Indirect(reflect.ValueOf(sample))

	if err := genUpdateSql(rv, &set, &where); err != nil {
		return "", nil, err
	}

	if len(set) == 0 {
		return "", nil, fmt.Errorf("Update %s `set` is empty", table)
	}
	if len(where) == 0 {
		return "", nil, fmt.Errorf("update %s `where` is empty", table)
	}

	buf := bytes.NewBufferString("update " + table + " set")
	args := []interface{}{}
	for i, v := range set {
		if i != 0 {
			buf.WriteString(",")
		}
		buf.WriteString(" `" + v.k + "` = ?")
		args = append(args, v.v)
	}

	buf.WriteString(" where")
	for i, v := range where {
		if i != 0 {
			buf.WriteString(" and")
		}
		buf.WriteString(" `" + v.k + "` = ?")
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

func GenDeleteSql(table string, selector queries.Selector) (string, []interface{}, error) {
	if table == "" {
		return "", nil, errTableEmpty
	}
	if selector == nil {
		return "", nil, errSelectorNil
	}

	query, args := selector.Sql()
	if query == "" {
		return "", nil, errQueryEmpty
	}

	return fmt.Sprintf("delete from `%s` where %s", table, query), args, nil
}
