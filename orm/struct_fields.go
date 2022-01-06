package orm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yubo/golib/util"
)

var fieldCache sync.Map // map[reflect.Type]structFields

// A field represents a single field found in a struct.
// `param:"query,required" format:"password" description:"aaa"`
type field struct {
	*FieldOptions
	typ   reflect.Type
	index []int
}

func (p field) String() string {
	return fmt.Sprintf("name %s index %v %s", p.name, p.index, p.FieldOptions)
}

type FieldOptions struct {
	Set

	fieldName string
	name      string
	where     bool
	skip      bool
	//key       string // use name instead of key

	// from tag
	dataType              DataType
	autoCreateTime        TimeType
	autoUpdateTime        TimeType
	primaryKey            bool
	autoIncrement         bool
	autoIncrementNum      int64
	hasDefaultValue       bool
	defaultValue          string
	defaultValueInterface interface{}
	size                  *int64
	precision             *int64
	scale                 *int64
	notNull               *bool
	unique                *bool
	comment               *string

	// index
	idxComment string
	idxClass   string
	idxOption  string

	driverDataType string

	// from driver

	indexKey  bool
	indexName string
	class     string
}

func (p FieldOptions) String() string {
	return fmt.Sprintf("name %s skip %v where %v",
		p.name, p.skip, p.where)
}

type structFields struct {
	list      []field
	nameIndex map[string]int
}

func (p structFields) String() (ret string) {
	for k, v := range p.list {
		ret += fmt.Sprintf("%d %s\n", k, v)
	}
	return ret
}

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type, driver Driver) structFields {
	if f, ok := fieldCache.Load(t); ok {
		return f.(structFields)
	}
	f, _ := fieldCache.LoadOrStore(t, typeFields(t, driver))

	return f.(structFields)
}

// typeFields returns a list of fields that JSON should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
func typeFields(t reflect.Type, driver Driver) structFields {
	// Anonymous fields to explore at the current level and the next.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	current := []field{}
	next := []field{{typ: t}}

	// Count of queued names for current level and the next.
	var count, nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []field

	// Buffer to run HTMLEscape on field names.
	// var nameEscBuf bytes.Buffer

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			// Scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				isUnexported := sf.PkgPath != ""
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Ptr {
						t = t.Elem()
					}
					if isUnexported && t.Kind() != reflect.Struct {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}
					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if isUnexported {
					// Ignore unexported non-embedded fields.
					continue
				}

				opt, err := parseStructField(sf)
				if err != nil {
					panic(err)
				}
				if opt.skip {
					continue
				}
				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Ptr {
					// Follow pointer.
					ft = ft.Elem()
				}

				// Record found field and index sequence.
				// if opt.name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
				if opt.name != "" || !sf.Anonymous {
					field := field{
						FieldOptions: opt,
						index:        index,
						typ:          ft,
					}

					// parsed by driver
					driver.ParseField(&field)

					fields = append(fields, field)
					if count[f.typ] > 1 {
						// If there were multiple instances, add a second,
						// so that the annihilation code will see a duplicate.
						// It only cares about the distinction between 1 or 2,
						// so don't bother generating any more copies.
						fields = append(fields, fields[len(fields)-1])
					}
					continue
				}

				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, field{index: index, typ: ft})
				}
			}
		}
	}

	nameIndex := make(map[string]int, len(fields))
	for i, field := range fields {
		if _, ok := nameIndex[field.name]; ok {
			panicType(field.typ, fmt.Sprintf("duplicate field %s", field.name))
		}
		nameIndex[field.name] = i
	}
	return structFields{fields, nameIndex}
}

func getSubv(rv reflect.Value, index []int, allowCreate bool) (reflect.Value, error) {
	subv := rv
	for _, i := range index {
		if subv.Kind() == reflect.Ptr {
			if subv.IsNil() {
				if !allowCreate {
					return subv, fmt.Errorf("struct %v is nil", subv.Type().Elem())
				}

				if !subv.CanSet() {
					return subv, fmt.Errorf("getSubv: cannot set embedded pointer to unexported struct: %v", subv.Type().Elem())
				}
				subv.Set(reflect.New(subv.Type().Elem()))
			}
			subv = subv.Elem()
		}
		subv = subv.Field(i)
	}
	return subv, nil
}

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, Set) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], ParseFields(tag[idx+1:])
	}
	return tag, ParseFields("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}

// `param:"(path|header|param|data)?(,required)?"`
// `name:"keyName"`
// `json:"keyName"`
// `format:"password"`
// `description:"ooxxoo"`
// func getTags(ff reflect.StructField) (name, paramType, format string, skip, bool) {
func parseStructField(sf reflect.StructField) (*FieldOptions, error) {
	if sf.Anonymous {
		return nil, nil
	}

	opt := &FieldOptions{}

	tag := sf.Tag.Get("sql")
	if tag == "-" {
		opt.skip = true
		return opt, nil
	}

	set := ParseFields("name=" + tag)
	if set.Has("where") {
		opt.where = true
	}
	opt.Set = set

	//opt.key = snakeCasedName(sf.Name)
	opt.fieldName = sf.Name
	opt.name = set.Get("name")
	if opt.name == "" {
		opt.name = snakeCasedName(opt.fieldName)
	}
	opt.name = strings.ToLower(opt.name)

	if set.Has("index") {
		opt.indexKey = true
		opt.indexName = set.Get("index")
	}
	if set.Has("primary_key") {
		opt.primaryKey = true
	}

	if set.Has("auto_increment") {
		opt.autoIncrement = true
		opt.hasDefaultValue = true
		opt.autoIncrementNum, _ = strconv.ParseInt(set.Get("auto_increment"), 10, 64)
	}
	if set.Has("default") {
		opt.hasDefaultValue = true
		opt.defaultValue = set.Get("default")
	}
	if set.Has("size") {
		if size, err := strconv.ParseInt(set.Get("size"), 10, 64); err == nil {
			opt.size = util.Int64(size)
		}
	}
	if set.Has("precision") {
		if precision, err := strconv.ParseInt(set.Get("precision"), 10, 64); err == nil {
			opt.precision = util.Int64(precision)
		}
	}
	if set.Has("scale") {
		if scale, err := strconv.ParseInt(set.Get("scale"), 10, 64); err == nil {
			opt.scale = util.Int64(scale)
		}
	}
	if set.Has("not_null") || set.Has("notnull") {
		opt.notNull = util.Bool(true)
	}
	if set.Has("unique") {
		opt.unique = util.Bool(true)
	}
	if set.Has("comment") {
		opt.comment = util.String(set.Get("comment"))
	}

	opt.defaultValue = strings.TrimSpace(opt.defaultValue)
	skipParseDefaultValue := strings.Contains(opt.defaultValue, "(") &&
		strings.Contains(opt.defaultValue, ")") || strings.ToLower(opt.defaultValue) == "null" || opt.defaultValue == ""

	t := sf.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var err error
	switch t.Kind() {
	case reflect.Bool:
		opt.dataType = Bool
		if opt.hasDefaultValue && !skipParseDefaultValue {
			if opt.defaultValueInterface, err = strconv.ParseBool(opt.defaultValue); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for bool, got error: %v", opt.defaultValue, err)
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		opt.dataType = Int
		if opt.hasDefaultValue && !skipParseDefaultValue {
			if opt.defaultValueInterface, err = strconv.ParseInt(opt.defaultValue, 0, 64); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for int, got error: %v", opt.defaultValue, err)
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		opt.dataType = Uint
		if opt.hasDefaultValue && !skipParseDefaultValue {
			if opt.defaultValueInterface, err = strconv.ParseUint(opt.defaultValue, 0, 64); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for uint, got error: %v", opt.defaultValue, err)
			}
		}
	case reflect.Float32, reflect.Float64:
		opt.dataType = Float
		if opt.hasDefaultValue && !skipParseDefaultValue {
			if opt.defaultValueInterface, err = strconv.ParseFloat(opt.defaultValue, 64); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for float, got error: %v", opt.defaultValue, err)
			}
		}
	case reflect.String:
		opt.dataType = String

		if opt.hasDefaultValue && !skipParseDefaultValue {
			opt.defaultValue = strings.Trim(opt.defaultValue, "'")
			opt.defaultValue = strings.Trim(opt.defaultValue, `"`)
			opt.defaultValueInterface = opt.defaultValue
		}
	case reflect.Struct:
		if t.ConvertibleTo(reflect.TypeOf(time.Time{})) {
			opt.dataType = Time
		}
	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			opt.dataType = Bytes
		}
	}

	if set.Has("auto_createtime") || (opt.name == "created_at" && (opt.dataType == Time || opt.dataType == Int || opt.dataType == Uint)) {
		v := set.Get("auto_createtime")
		if opt.dataType == Time {
			opt.autoCreateTime = UnixTime
		} else if strings.ToLower(v) == "nano" {
			opt.autoCreateTime = UnixNanosecond
		} else if strings.ToLower(v) == "milli" {
			opt.autoCreateTime = UnixMillisecond
		} else {
			opt.autoCreateTime = UnixSecond
		}
	}

	if set.Has("auto_updatetime") || (opt.name == "updated_at" && (opt.dataType == Time || opt.dataType == Int || opt.dataType == Uint)) {
		v := set.Get("auto_updatetime")
		if opt.dataType == Time {
			opt.autoUpdateTime = UnixTime
		} else if strings.ToLower(v) == "nano" {
			opt.autoUpdateTime = UnixNanosecond
		} else if strings.ToLower(v) == "milli" {
			opt.autoUpdateTime = UnixMillisecond
		} else {
			opt.autoUpdateTime = UnixSecond
		}
	}

	if set.Has("type") {
		val := set.Get("type")
		switch DataType(strings.ToLower(val)) {
		case Bool, Int, Uint, Float, String, Time, Bytes:
			opt.dataType = DataType(strings.ToLower(val))
		default:
			opt.dataType = DataType(val)
		}
	}

	if opt.size == nil {
		switch t.Kind() {
		case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Float64:
			opt.size = util.Int64(64)
		case reflect.Int8, reflect.Uint8:
			opt.size = util.Int64(8)
		case reflect.Int16, reflect.Uint16:
			opt.size = util.Int64(16)
		case reflect.Int32, reflect.Uint32, reflect.Float32:
			opt.size = util.Int64(32)
		}
	}

	return opt, nil
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

func panicType(ft reflect.Type, args ...interface{}) {
	msg := fmt.Sprintf("type field %s %s", ft.PkgPath(), ft.Name())

	if len(args) > 0 {
		panic(fmt.Sprint(args...) + " " + msg)
	}
	panic(msg)
}
