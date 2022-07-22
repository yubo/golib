package template

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/yubo/golib/util"
	"sigs.k8s.io/yaml"
)

var (
	FuncMap = map[string]interface{}{
		"hello":      func() string { return "hello!" },
		"env":        func(s string) string { return os.Getenv(s) },
		"expandenv":  func(s string) string { return os.ExpandEnv(s) },
		"base":       path.Base,
		"dir":        path.Dir,
		"clean":      path.Clean,
		"ext":        path.Ext,
		"isAbs":      path.IsAbs,
		"quote":      quote,
		"squote":     squote,
		"contains":   func(substr string, str string) bool { return strings.Contains(str, substr) },
		"hasPrefix":  func(substr string, str string) bool { return strings.HasPrefix(str, substr) },
		"hasSuffix":  func(substr string, str string) bool { return strings.HasSuffix(str, substr) },
		"trim":       strings.TrimSpace,
		"trimAll":    func(a, b string) string { return strings.Trim(b, a) },
		"trimSuffix": func(a, b string) string { return strings.TrimSuffix(b, a) },
		"trimPrefix": func(a, b string) string { return strings.TrimPrefix(b, a) },
		"split":      split,
		"splitList":  func(sep, orig string) []string { return strings.Split(orig, sep) },
		"toString":   strval,
		"toStrings":  strslice,
		"join":       join,
		"sortAlpha":  sortAlpha,
		"b64enc":     base64encode,
		"b64dec":     base64decode,
		"cat":        cat,
		"indent":     indent,
		"nindent":    nindent,
		"replace":    replace,
		"atoi":       func(a string) int { i, _ := strconv.Atoi(a); return i },
		"atob":       func(a string) bool { i, _ := strconv.ParseBool(a); return i },
		"int64":      toInt64,
		"int":        toInt,
		"float64":    toFloat64,
		"toJson":     toJson,
		"toYaml":     toYaml,
		"max":        max,
		"min":        min,
		"typeOf":     typeOf, // Reflection
		"typeIs":     typeIs,
		"typeIsLike": typeIsLike,
		"kindOf":     kindOf,
		"kindIs":     kindIs,
		"list":       list,   // Data Structures:
		"timeOf":     timeOf, // time to second
		"sizeOf":     sizeOf, // bytesize to byte
		"last":       last,
		"repeat":     repeat,
	}
)

var (
	parser *template.Template
)

func init() {
	parser = template.New("parser").Funcs(FuncMap)
}

func MustTpl(data string, input interface{}) string {
	buff := &bytes.Buffer{}

	if err := template.Must(template.New("").Parse(data)).Execute(buff, input); err != nil {
		panic(err)
	}

	return buff.String()
}

func ReadFileWithInclude(path string) (b []byte, err error) {
	buf := &bytes.Buffer{}

	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()

	if err := readFileWithInclude(buf, path); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readFileWithInclude(w io.Writer, path string, prefixs ...string) error {
	files, err := filepath.Glob(path)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`^(\s*)include\s+([^\s]+)\s*$`)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(f)

		prefix := strings.Join(prefixs, "")

		for scanner.Scan() {
			line := scanner.Text()
			match := re.FindStringSubmatch(line)

			if len(match) != 3 {
				w.Write([]byte(prefix + line + "\n"))
				continue
			}

			if err := readFileWithInclude(w,
				strings.Trim(match[2], "\""),
				append(prefixs, match[1])...); err != nil {
				return err
			}
		}
		f.Close()
	}

	return nil
}

func ParseTemplateFile(values interface{}, filename string) (b []byte, err error) {
	if filename = strings.TrimSpace(filename); filename == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = ReadFileWithInclude(filename)
	}
	if err != nil {
		return []byte{}, err
	}
	return ParseTemplateText(values, string(b))
}

func ParseTemplateText(values interface{}, text string) ([]byte, error) {
	var b bytes.Buffer

	tpl, err := parser.Parse(text)
	if err != nil {
		return b.Bytes(), err
	}

	if err = tpl.Execute(&b, values); err != nil {
		return b.Bytes(), err
	}

	return []byte(expandEnv(b.String())), nil
}

func quote(str ...interface{}) string {
	out := make([]string, len(str))
	for i, s := range str {
		out[i] = fmt.Sprintf("%q", strval(s))
	}
	return strings.Join(out, " ")
}

func squote(str ...interface{}) string {
	out := make([]string, len(str))
	for i, s := range str {
		out[i] = fmt.Sprintf("'%v'", s)
	}
	return strings.Join(out, " ")
}

func cat(v ...interface{}) string {
	r := strings.TrimSpace(strings.Repeat("%v ", len(v)))
	return fmt.Sprintf(r, v...)
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}

func nindent(spaces int, v string) string {
	return "\n" + indent(spaces, v)
}

func replace(old, new, src string) string {
	return strings.Replace(src, old, new, -1)
}

func join(sep string, v interface{}) string {
	return strings.Join(strslice(v), sep)
}

func split(sep, orig string) map[string]string {
	parts := strings.Split(orig, sep)
	res := make(map[string]string, len(parts))
	for i, v := range parts {
		res["_"+strconv.Itoa(i)] = v
	}
	return res
}

func sortAlpha(list interface{}) []string {
	k := reflect.Indirect(reflect.ValueOf(list)).Kind()
	switch k {
	case reflect.Slice, reflect.Array:
		a := strslice(list)
		s := sort.StringSlice(a)
		s.Sort()
		return s
	}
	return []string{strval(list)}
}

func repeat(n int, str string) []string {
	res := make([]string, n)
	for i := 0; i < n; i++ {
		res[i] = str
	}
	return res
}

func strslice(v interface{}) []string {
	switch v := v.(type) {
	case []string:
		return v
	case []interface{}:
		l := len(v)
		b := make([]string, l)
		for i := 0; i < l; i++ {
			b[i] = strval(v[i])
		}
		return b
	default:
		val := reflect.ValueOf(v)
		switch val.Kind() {
		case reflect.Array, reflect.Slice:
			l := val.Len()
			b := make([]string, l)
			for i := 0; i < l; i++ {
				b[i] = strval(val.Index(i).Interface())
			}
			return b
		default:
			return []string{strval(v)}
		}
	}
}

func strval(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toFloat64 converts 64-bit floats
func toFloat64(v interface{}) float64 {
	if str, ok := v.(string); ok {
		iv, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return 0
		}
		return iv
	}

	val := reflect.Indirect(reflect.ValueOf(v))
	switch val.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return float64(val.Int())
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return float64(val.Uint())
	case reflect.Uint, reflect.Uint64:
		return float64(val.Uint())
	case reflect.Float32, reflect.Float64:
		return val.Float()
	case reflect.Bool:
		if val.Bool() == true {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func toInt(v interface{}) int {
	//It's not optimal. Bud I don't want duplicate toInt64 code.
	return int(toInt64(v))
}

// toInt64 converts integer types to 64-bit integers
func toInt64(v interface{}) int64 {
	if str, ok := v.(string); ok {
		iv, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return 0
		}
		return iv
	}

	val := reflect.Indirect(reflect.ValueOf(v))
	switch val.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return val.Int()
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return int64(val.Uint())
	case reflect.Uint, reflect.Uint64:
		tv := val.Uint()
		if tv <= math.MaxInt64 {
			return int64(tv)
		}
		return math.MaxInt64
	case reflect.Float32, reflect.Float64:
		return int64(val.Float())
	case reflect.Bool:
		if val.Bool() == true {
			return 1
		}
		return 0
	default:
		return 0
	}
}
func max(a interface{}, i ...interface{}) int64 {
	aa := toInt64(a)
	for _, b := range i {
		bb := toInt64(b)
		if bb > aa {
			aa = bb
		}
	}
	return aa
}

func min(a interface{}, i ...interface{}) int64 {
	aa := toInt64(a)
	for _, b := range i {
		bb := toInt64(b)
		if bb < aa {
			aa = bb
		}
	}
	return aa
}

func toJson(v interface{}) string {
	output, _ := json.Marshal(v)
	return string(output)
}

func toYaml(v interface{}) string {
	output, _ := yaml.Marshal(v)
	return string(output)
}

func base64encode(v string) string {
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func base64decode(v string) string {
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// typeIs returns true if the src is the type named in target.
func typeIs(target string, src interface{}) bool {
	return target == typeOf(src)
}

func typeIsLike(target string, src interface{}) bool {
	t := typeOf(src)
	return target == t || "*"+target == t
}

func typeOf(src interface{}) string {
	return fmt.Sprintf("%T", src)
}

func kindIs(target string, src interface{}) bool {
	return target == kindOf(src)
}

func kindOf(src interface{}) string {
	return reflect.ValueOf(src).Kind().String()
}

func list(v ...interface{}) []interface{} {
	return v
}

func timeOf(v string) string {
	return strconv.FormatInt(util.TimeOf(v), 10) + "s"
}

func sizeOf(v string) string {
	return strconv.FormatUint(util.SizeOf(v), 10)
}

func last(x int, a interface{}) bool {
	return x == reflect.ValueOf(a).Len()-1
}

func expandEnv(s string) string {
	return Expand(s, func(str string) string {
		// This allows escaping environment variable substitution via $$, e.g.
		// - $FOO will be substituted with env var FOO
		// - $$FOO will be replaced with $FOO
		// - $$$FOO will be replaced with $ + substituted env var FOO
		if str == "$" {
			return "$"
		}
		return os.Getenv(str)
	})
}

// Expand replaces ${var} or $var in the string based on the mapping function.
// For example, os.ExpandEnv(s) is equivalent to os.Expand(s, os.Getenv).
func Expand(s string, mapping func(string) string) string {
	var buf []byte
	// $() is all ASCII, so bytes are fine for this operation.
	i := 0
	for j := 0; j < len(s); j++ {
		if s[j] == '$' && j+1 < len(s) && s[j+1] == '(' {
			if buf == nil {
				buf = make([]byte, 0, 2*len(s))
			}
			buf = append(buf, s[i:j]...)
			name, w := getShellName(s[j+1:])
			if name == "" && w > 0 {
				// Encountered invalid syntax; eat the
				// characters.
			} else if name == "" {
				// Valid syntax, but $ was not followed by a
				// name. Leave the dollar character untouched.
				buf = append(buf, s[j])
			} else {
				buf = append(buf, mapping(name)...)
			}
			j += w
			i = j + 1
		}
	}
	if buf == nil {
		return s
	}
	return string(buf) + s[i:]
}

// getShellName returns the name that begins the string and the number of bytes
// consumed to extract it. If the name is enclosed in {}, it's part of a ${}
// expansion and two more bytes are needed than the length of the name.
func getShellName(s string) (string, int) {
	switch {
	case s[0] == '(':
		if len(s) > 2 && isShellSpecialVar(s[1]) && s[2] == ')' {
			return s[1:2], 3
		}
		// Scan to closing brace
		for i := 1; i < len(s); i++ {
			if s[i] == ')' {
				if i == 1 {
					return "", 2 // Bad syntax; eat "${}"
				}
				return s[1:i], i + 1
			}
		}
		return "", 1 // Bad syntax; eat "${"
	case isShellSpecialVar(s[0]):
		return s[0:1], 1
	}
	// Scan alphanumerics.
	var i int
	for i = 0; i < len(s) && isAlphaNum(s[i]); i++ {
	}
	return s[:i], i
}

// isShellSpecialVar reports whether the character identifies a special
// shell variable such as $*.
func isShellSpecialVar(c uint8) bool {
	switch c {
	case '*', '#', '$', '@', '!', '?', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}

// isAlphaNum reports whether the byte is an ASCII letter, number, or underscore
func isAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}
