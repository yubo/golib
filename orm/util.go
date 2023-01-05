package orm

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yubo/golib/queries"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/clock"
	"k8s.io/klog/v2"
)

const (
	maxPacketSize = 1<<24 - 1
)

var (
	regRealDataType = regexp.MustCompile(`[^\d](\d+)[^\d]?`)
	regFullDataType = regexp.MustCompile(`[^\d]*(\d+)[^\d]?`)

	ErrSkip                      = errors.New("driver: skip fast-path; continue as if unimplemented")
	errSampleNil                 = errors.New("input sample is nil")
	errTableEmpty                = errors.New("table name is not set")
	errSelectorNil               = errors.New("selector is nil")
	errSelectorEmpty             = errors.New("selector is empty")
	defaultClock     clock.Clock = clock.RealClock{}
)

func dlog(format string, args ...interface{}) {
	if klog.V(6).Enabled() || DEBUG {
		klog.InfofDepth(2, format, args...)
	}
}

func elog(format string, args ...interface{}) {
	klog.ErrorfDepth(1, format, args...)
}

func dlogSql(query string, args ...interface{}) {
	if klog.V(10).Enabled() || DEBUG {
		prepared, err := interpolateParams(query, args)
		if err != nil {
			klog.ErrorDepth(1, err)
			return
		}
		klog.InfoDepth(2, prepared)
	}
}

// {1,2,3} => "(1,2,3)"
func Ints2sql(array []int64) string {
	buf := []byte("(")

	for i := 0; i < len(array); i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = strconv.AppendInt(buf, array[i], 10)
	}
	return string(append(buf, ')'))
}

// {"1","2","3"} => "('1', '2', '3')"
func Strings2sql(array []string) string {
	buf := []byte("(")
	for i := 0; i < len(array); i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '\'')
		buf = append(buf, array[i]...)
		buf = append(buf, '\'')
	}
	return string(append(buf, ')'))
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

	if rt.Kind() != reflect.Struct {
		return false
	}

	if rt.String() == "time.Time" {
		return false
	}

	if _, ok := in.(sql.Scanner); ok {
		return false
	}

	return true
}

type kv struct {
	k string
	v interface{}
}

func GenInsertSql(table string, sample interface{}, db Driver) (string, []interface{}, error) {
	if sample == nil {
		return "", nil, errSampleNil
	}
	if table == "" {
		return "", nil, errTableEmpty
	}

	values := []kv{}

	rv := reflect.Indirect(reflect.ValueOf(sample))

	if err := genInsertSql(rv, &values, db); err != nil {
		return "", nil, err
	}

	if len(values) == 0 {
		return "", nil, fmt.Errorf("INSERT INTO `%s` `VALUES` is empty", table)
	}

	buf := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	args := []interface{}{}

	buf.WriteString("INSERT INTO `" + table + "` (")

	for i, v := range values {
		if i != 0 {
			buf.WriteString(", ")
			buf2.WriteString(", ")
		}
		buf.WriteString("`" + v.k + "`")
		buf2.WriteString("?")
		args = append(args, v.v)
	}

	return buf.String() + ") VALUES (" + buf2.String() + ")", args, nil
}

func genInsertSql(rv reflect.Value, values *[]kv, db Driver) error {
	fields := cachedTypeFields(rv.Type(), db)
	curTime := defaultClock.Now()

	for _, f := range fields.Fields {
		if f.AutoCreatetime > 0 {
			*values = append(*values, kv{
				f.Name,
				NewCurTime(f.AutoCreatetime, curTime),
			})
			continue
		} else if f.AutoUpdatetime > 0 {
			*values = append(*values, kv{
				f.Name,
				NewCurTime(f.AutoUpdatetime, curTime),
			})
			continue
		}

		fv, err := getSubv(rv, f.Index, false)
		if err != nil || IsNil(fv) {
			continue
		}

		v, err := sqlInterface(fv)
		if err != nil {
			return err
		}
		*values = append(*values, kv{f.Name, v})
	}
	return nil
}

func NewCurTime(t TimeType, cur time.Time) interface{} {
	switch t {
	case UnixTime:
		return cur
	case UnixSecond:
		return cur.Unix()
	case UnixMillisecond:
		return cur.UnixNano() / 1e6
	case UnixNanosecond:
		return cur.UnixNano()
	default:
		klog.Errorf("unsupported timetype %d", t)
		return nil
	}
}

func GenListSql(table string, cols []string, selector queries.Selector, orderby []string, offset, limit int) (string, string, []interface{}, error) {
	if table == "" {
		return "", "", nil, errTableEmpty
	}

	// SELECT *
	buf := bytes.NewBufferString("SELECT")
	// SELECT count(*)
	buf2 := bytes.NewBufferString("SELECT COUNT(*) FROM `" + table + "`")
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
	buf.WriteString(" FROM `" + table + "`")

	// selector
	if selector != nil {
		if q, a := selector.Sql(); q != "" {
			buf.WriteString(" WHERE " + q)
			buf2.WriteString(" WHERE " + q)
			args = a
		}
	}

	// order
	if len(orderby) > 0 {
		buf.WriteString(" ORDER BY " + strings.Join(orderby, ", "))
	}

	// limit
	if limit > 0 {
		fmt.Fprintf(buf, " LIMIT %d, %d", offset, limit)
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
		return "", nil, errSelectorEmpty
	}

	// SELECT *
	buf := bytes.NewBufferString("SELECT")

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
	buf.WriteString(" FROM `" + table + "` WHERE " + query)

	return buf.String(), args, nil
}

func GenUpdateSql(table string, sample interface{}, db Driver, selector queries.Selector) (string, []interface{}, error) {
	if table == "" {
		return "", nil, errTableEmpty
	}
	if sample == nil {
		return "", nil, errSampleNil
	}

	set := []kv{}
	where := []kv{}

	rv := reflect.Indirect(reflect.ValueOf(sample))

	if err := genUpdateSql(rv, &set, &where, db); err != nil {
		return "", nil, err
	}

	if len(set) == 0 {
		return "", nil, fmt.Errorf("UPDATE `%s` `SET` is empty", table)
	}

	buf := bytes.NewBufferString("UPDATE `" + table + "` SET")
	args := []interface{}{}
	for i, v := range set {
		if i != 0 {
			buf.WriteString(",")
		}
		buf.WriteString(" `" + v.k + "` = ?")
		args = append(args, v.v)
	}

	buf.WriteString(" WHERE")

	// selector
	if selector != nil {
		query, args2 := selector.Sql()
		if query == "" {
			return "", nil, errSelectorEmpty
		}
		buf.WriteString(" " + query)
		args = append(args, args2...)
	} else if len(where) > 0 {
		for i, v := range where {
			if i != 0 {
				buf.WriteString(" AND")
			}
			buf.WriteString(" `" + v.k + "` = ?")
			args = append(args, v.v)
		}
	} else {
		return "", nil, fmt.Errorf("UPDATE `%s` `WHERE` is empty", table)
	}

	return buf.String(), args, nil
}

func genUpdateSql(rv reflect.Value, set, where *[]kv, db Driver) error {
	fields := cachedTypeFields(rv.Type(), db)
	curTime := defaultClock.Now()
	for _, f := range fields.Fields {
		if f.AutoCreatetime > 0 {
			continue
		} else if f.AutoUpdatetime > 0 {
			*set = append(*set, kv{
				f.Name,
				NewCurTime(f.AutoUpdatetime, curTime),
			})
			continue
		}

		fv, err := getSubv(rv, f.Index, false)
		if err != nil || IsNil(fv) {
			continue
		}

		if fv.Kind() == reflect.Ptr {
			fv = fv.Elem()
		}

		v, err := sqlInterface(fv)
		if err != nil {
			return err
		}

		if f.Where {
			*where = append(*where, kv{f.Name, v})
		} else {
			*set = append(*set, kv{f.Name, v})
		}
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
		return "", nil, errSelectorEmpty
	}

	return fmt.Sprintf("DELETE FROM `%s` WHERE %s", table, query), args, nil
}

func GetFields(sample interface{}, driver Driver) StructFields {
	return cachedTypeFields(reflect.Indirect(reflect.ValueOf(sample)).Type(), driver)
}

func GetField(sample interface{}, field string, driver Driver) *StructField {
	fields := GetFields(sample, driver)

	if n, ok := fields.nameIndex[field]; ok {
		return fields.Fields[n]
	}

	return nil
}

func IsNil(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// scanInterface input is struct's field
func scanInterface(rv reflect.Value, tran *[]*transfer) (interface{}, error) {
	rt := rv.Type()
	ptr := false

	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		ptr = true
	}

	iface := rv.Addr().Interface()

	switch iface.(type) {
	case *time.Time:
		return iface, nil
	case **time.Time:
		return iface, nil
	case sql.Scanner:
		return iface, nil
	case *[]byte:
		return iface, nil
	case **[]byte:
		return iface, nil
	}

	if rt.Kind() == reflect.Map ||
		rt.Kind() == reflect.Struct ||
		rt.Kind() == reflect.Slice {
		// json decode support *struct{}, but not **struct{}, so should adapt it
		node := &transfer{dst: iface, ptr: ptr}
		*tran = append(*tran, node)
		return &node.dstProxy, nil
	}

	return iface, nil
}

// sqlInterface: rv should not be ptr, return interface for use in sql's args
func sqlInterface(rv reflect.Value) (interface{}, error) {
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	iface := rv.Interface()

	switch iface.(type) {
	case time.Time:
		return iface, nil
	case sql.Scanner:
		return iface, nil
	case []byte:
		return iface, nil
	}

	if rv.Kind() == reflect.Map ||
		rv.Kind() == reflect.Struct ||
		rv.Kind() == reflect.Slice {
		return json.Marshal(iface)
	}

	return iface, nil
}

func AddSqlArgs(sql string, args []interface{},
	intoSql *string, intoArgs *[]interface{}) {
	if n := len(args); n > 0 {
		s := strings.Repeat("?,", n)
		*intoSql += fmt.Sprintf(strings.Replace(sql, "?", "%s", 1), s[:len(s)-1])
		*intoArgs = append(*intoArgs, args...)
		return
	}

	*intoSql += sql
	*intoArgs = append(*intoArgs, nil)
}

func SetClock(clock clock.Clock) {
	defaultClock = clock
}

func typeOfArray(in interface{}) string {
	rt := reflect.TypeOf(in)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if k := rt.Kind(); k == reflect.Slice || k == reflect.Array {
		rt = rt.Elem()
	}
	return util.SnakeCasedName(rt.Name())
}

// interpolateParams from go-sql-driver/mysql
func interpolateParams(query string, args []interface{}) (string, error) {
	// Number of ? should be same to len(args)
	if strings.Count(query, "?") != len(args) {
		return "", ErrSkip
	}

	buf := []byte{}
	argPos := 0
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return "", err
	}

	for i := 0; i < len(query); i++ {
		q := strings.IndexByte(query[i:], '?')
		if q == -1 {
			buf = append(buf, query[i:]...)
			break
		}
		buf = append(buf, query[i:i+q]...)
		i += q

		arg := args[argPos]
		argPos++

		if arg == nil {
			buf = append(buf, "NULL"...)
			continue
		}

		arg, err = driver.DefaultParameterConverter.ConvertValue(arg)
		if err != nil {
			return "", err
		}

		switch v := arg.(type) {
		case int64:
			buf = strconv.AppendInt(buf, v, 10)
		case uint64:
			buf = strconv.AppendUint(buf, v, 10)
		case float32:
			buf = strconv.AppendFloat(buf, float64(v), 'g', -1, 64)
		case float64:
			buf = strconv.AppendFloat(buf, v, 'g', -1, 64)
		case bool:
			if v {
				buf = append(buf, '1')
			} else {
				buf = append(buf, '0')
			}
		case time.Time:
			if v.IsZero() {
				buf = append(buf, "'0000-00-00'"...)
			} else {
				buf = append(buf, '\'')
				buf, err = appendDateTime(buf, v.In(loc))
				if err != nil {
					return "", err
				}
				buf = append(buf, '\'')
			}
		case json.RawMessage:
			buf = append(buf, '\'')
			buf = escapeBytesBackslash(buf, v)
			//buf = escapeBytesQuotes(buf, v)
			buf = append(buf, '\'')
		case []byte:
			if v == nil {
				buf = append(buf, "NULL"...)
			} else {
				buf = append(buf, "_binary'"...)
				//buf = escapeBytesBackslash(buf, v)
				buf = escapeBytesQuotes(buf, v)
				buf = append(buf, '\'')
			}
		case string:
			buf = append(buf, '\'')
			//buf = escapeStringBackslash(buf, v)
			buf = escapeStringQuotes(buf, v)
			buf = append(buf, '\'')
		default:
			elog("unsupport print type %v", reflect.TypeOf(v))
			return "", ErrSkip
		}

		if len(buf)+4 > maxPacketSize {
			return "", ErrSkip
		}
	}
	if argPos != len(args) {
		return "", ErrSkip
	}
	return string(buf), nil
}

func appendDateTime(buf []byte, t time.Time) ([]byte, error) {
	nsec := t.Nanosecond()
	// to round under microsecond
	if nsec%1000 >= 500 { // save half of time.Time.Add calls
		t = t.Add(500 * time.Nanosecond)
		nsec = t.Nanosecond()
	}
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	micro := nsec / 1000

	if year < 1 || year > 9999 {
		return buf, errors.New("year is not in the range [1, 9999]: " + strconv.Itoa(year)) // use errors.New instead of fmt.Errorf to avoid year escape to heap
	}
	year100 := year / 100
	year1 := year % 100

	var localBuf [26]byte // does not escape
	localBuf[0], localBuf[1], localBuf[2], localBuf[3] = digits10[year100], digits01[year100], digits10[year1], digits01[year1]
	localBuf[4] = '-'
	localBuf[5], localBuf[6] = digits10[month], digits01[month]
	localBuf[7] = '-'
	localBuf[8], localBuf[9] = digits10[day], digits01[day]

	if hour == 0 && min == 0 && sec == 0 && micro == 0 {
		return append(buf, localBuf[:10]...), nil
	}

	localBuf[10] = ' '
	localBuf[11], localBuf[12] = digits10[hour], digits01[hour]
	localBuf[13] = ':'
	localBuf[14], localBuf[15] = digits10[min], digits01[min]
	localBuf[16] = ':'
	localBuf[17], localBuf[18] = digits10[sec], digits01[sec]

	if micro == 0 {
		return append(buf, localBuf[:19]...), nil
	}

	micro10000 := micro / 10000
	micro100 := (micro / 100) % 100
	micro1 := micro % 100
	localBuf[19] = '.'
	localBuf[20], localBuf[21], localBuf[22], localBuf[23], localBuf[24], localBuf[25] =
		digits10[micro10000], digits01[micro10000], digits10[micro100], digits01[micro100], digits10[micro1], digits01[micro1]

	return append(buf, localBuf[:]...), nil
}

// escapeStringQuotes is similar to escapeBytesQuotes but for string.
func escapeStringQuotes(buf []byte, v string) []byte {
	pos := len(buf)
	buf = reserveBuffer(buf, len(v)*2)

	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '\'' {
			buf[pos] = '\''
			buf[pos+1] = '\''
			pos += 2
		} else {
			buf[pos] = c
			pos++
		}
	}

	return buf[:pos]
}

// escapeStringBackslash is similar to escapeBytesBackslash but for string.
func escapeStringBackslash(buf []byte, v string) []byte {
	pos := len(buf)
	buf = reserveBuffer(buf, len(v)*2)

	for i := 0; i < len(v); i++ {
		c := v[i]
		switch c {
		case '\x00':
			buf[pos] = '\\'
			buf[pos+1] = '0'
			pos += 2
		case '\n':
			buf[pos] = '\\'
			buf[pos+1] = 'n'
			pos += 2
		case '\r':
			buf[pos] = '\\'
			buf[pos+1] = 'r'
			pos += 2
		case '\x1a':
			buf[pos] = '\\'
			buf[pos+1] = 'Z'
			pos += 2
		case '\'':
			buf[pos] = '\\'
			buf[pos+1] = '\''
			pos += 2
		case '"':
			buf[pos] = '\\'
			buf[pos+1] = '"'
			pos += 2
		case '\\':
			buf[pos] = '\\'
			buf[pos+1] = '\\'
			pos += 2
		default:
			buf[pos] = c
			pos++
		}
	}

	return buf[:pos]
}

// escapeBytesBackslash escapes []byte with backslashes (\)
// This escapes the contents of a string (provided as []byte) by adding backslashes before special
// characters, and turning others into specific escape sequences, such as
// turning newlines into \n and null bytes into \0.
// https://github.com/mysql/mysql-server/blob/mysql-5.7.5/mysys/charset.c#L823-L932
func escapeBytesBackslash(buf, v []byte) []byte {
	pos := len(buf)
	buf = reserveBuffer(buf, len(v)*2)

	for _, c := range v {
		switch c {
		case '\x00':
			buf[pos] = '\\'
			buf[pos+1] = '0'
			pos += 2
		case '\n':
			buf[pos] = '\\'
			buf[pos+1] = 'n'
			pos += 2
		case '\r':
			buf[pos] = '\\'
			buf[pos+1] = 'r'
			pos += 2
		case '\x1a':
			buf[pos] = '\\'
			buf[pos+1] = 'Z'
			pos += 2
		case '\'':
			buf[pos] = '\\'
			buf[pos+1] = '\''
			pos += 2
		case '"':
			buf[pos] = '\\'
			buf[pos+1] = '"'
			pos += 2
		case '\\':
			buf[pos] = '\\'
			buf[pos+1] = '\\'
			pos += 2
		default:
			buf[pos] = c
			pos++
		}
	}

	return buf[:pos]
}

// escapeBytesQuotes escapes apostrophes in []byte by doubling them up.
// This escapes the contents of a string by doubling up any apostrophes that
// it contains. This is used when the NO_BACKSLASH_ESCAPES SQL_MODE is in
// effect on the server.
// https://github.com/mysql/mysql-server/blob/mysql-5.7.5/mysys/charset.c#L963-L1038
func escapeBytesQuotes(buf, v []byte) []byte {
	pos := len(buf)
	buf = reserveBuffer(buf, len(v)*2)

	for _, c := range v {
		if c == '\'' {
			buf[pos] = '\''
			buf[pos+1] = '\''
			pos += 2
		} else {
			buf[pos] = c
			pos++
		}
	}

	return buf[:pos]
}

// reserveBuffer checks cap(buf) and expand buffer to len(buf) + appendSize.
// If cap(buf) is not enough, reallocate new buffer.
func reserveBuffer(buf []byte, appendSize int) []byte {
	newSize := len(buf) + appendSize
	if cap(buf) < newSize {
		// Grow buffer exponentially
		newBuf := make([]byte, len(buf)*2+appendSize)
		copy(newBuf, buf)
		buf = newBuf
	}
	return buf[:newSize]
}

const digits01 = "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"
const digits10 = "0000000000111111111122222222223333333333444444444455555555556666666666777777777788888888889999999999"
