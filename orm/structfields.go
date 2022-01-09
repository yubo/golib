package orm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/yubo/golib/util"
)

var (
	fieldCache sync.Map // map[reflect.Type]s
)

type StructFields struct {
	Fields    []*StructField
	nameIndex map[string]int
}

func (p StructFields) String() (ret string) {
	for k, v := range p.Fields {
		ret += fmt.Sprintf("%d %s\n", k, v)
	}
	return ret
}

type StructField struct {
	Set
	Type  reflect.Type
	Index []int

	FieldName string
	Name      string
	Where     bool
	Skip      bool
	Inline    bool
	//key       string // use name instead of key

	// from tag
	DataType              DataType
	AutoCreatetime        TimeType // auto_createtime
	AutoUpdatetime        TimeType // auto_updatetime
	PrimaryKey            bool
	AutoIncrement         bool
	AutoIncrementNum      int64
	HasDefaultValue       bool
	DefaultValue          string
	DefaultValueInterface interface{}
	Size                  *int64
	Precision             *int64
	Scale                 *int64
	NotNull               *bool
	Unique                *bool
	Comment               *string

	// index
	IndexComment string
	IndexClass   string
	IndexOption  string
	IndexKey     bool
	IndexName    string

	DriverDataType string
	Class          string
}

func (p StructField) String() string {
	return fmt.Sprintf("name %s skip %v where %v",
		p.Name, p.Skip, p.Where)
}

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type, driver Driver) StructFields {
	if f, ok := fieldCache.Load(t); ok {
		return f.(StructFields)
	}
	f, _ := fieldCache.LoadOrStore(t, parseTypeFields(t, driver))

	return f.(StructFields)
}

// parseTypeFields returns a list of fields that JSON should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
func parseTypeFields(t reflect.Type, driver Driver) StructFields {
	// Anonymous fields to explore at the current level and the next.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	current := []StructField{}
	next := []StructField{{Type: t}}

	// Count of queued names for current level and the next.
	var count, nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []*StructField

	// Buffer to run HTMLEscape on field names.
	// var nameEscBuf bytes.Buffer

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.Type] {
				continue
			}
			visited[f.Type] = true

			// Scan f.typ for fields to include.
			for i := 0; i < f.Type.NumField(); i++ {
				sf := f.Type.Field(i)
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Ptr {
						t = t.Elem()
					}
					if !sf.IsExported() && t.Kind() != reflect.Struct {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}
					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if !sf.IsExported() {
					// Ignore unexported non-embedded fields.
					continue
				}

				field, err := parseStructField(sf)
				if err != nil {
					panic(err)
				}
				if field.Skip {
					continue
				}
				index := make([]int, len(f.Index)+1)
				copy(index, f.Index)
				index[len(f.Index)] = i

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Ptr {
					// Follow pointer.
					ft = ft.Elem()
				}

				// Record found field and index sequence.
				// if opt.name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
				if field.Name != "" && !sf.Anonymous {
					field.Index = index
					field.Type = ft

					// parsed by driver
					driver.ParseField(field)

					fields = append(fields, field)
					if count[f.Type] > 1 {
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
					next = append(next, StructField{Index: index, Type: ft})
				}
			}
		}
	}

	nameIndex := make(map[string]int, len(fields))
	for i, field := range fields {
		if _, ok := nameIndex[field.Name]; ok {
			panicType(field.Type, fmt.Sprintf("duplicate field %s", field.Name))
		}
		nameIndex[field.Name] = i
	}
	return StructFields{fields, nameIndex}
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

// `param:"(path|header|param|data)?(,required)?"`
// `name:"keyName"`
// `json:"keyName"`
// `format:"password"`
// `description:"ooxxoo"`
// func getTags(ff reflect.StructField) (name, paramType, format string, skip, bool) {
func parseStructField(sf reflect.StructField) (*StructField, error) {
	if sf.Anonymous {
		return &StructField{}, nil
	}

	opt := &StructField{}

	tag := sf.Tag.Get("sql")
	if tag == "-" {
		opt.Skip = true
		return opt, nil
	}

	set := ParseFields("name=" + tag)
	if set.Has("where") {
		opt.Where = true
	}
	if set.Has("inline") {
		opt.Inline = true
	}
	opt.Set = set

	//opt.key = snakeCasedName(sf.Name)
	opt.FieldName = sf.Name
	opt.Name = set.Get("name")
	if opt.Name == "" {
		opt.Name = util.SnakeCasedName(opt.FieldName)
	}
	opt.Name = strings.ToLower(opt.Name)

	if set.Has("index") {
		opt.IndexKey = true
		opt.IndexName = set.Get("index")
	}
	if set.Has("primary_key") {
		opt.PrimaryKey = true
	}

	if set.Has("auto_increment") {
		opt.AutoIncrement = true
		opt.HasDefaultValue = true
		opt.AutoIncrementNum, _ = strconv.ParseInt(set.Get("auto_increment"), 10, 64)
	}
	if set.Has("default") {
		opt.HasDefaultValue = true
		opt.DefaultValue = set.Get("default")
	}
	if set.Has("size") {
		if size, err := strconv.ParseInt(set.Get("size"), 10, 64); err == nil {
			opt.Size = util.Int64(size)
		}
	}
	if set.Has("precision") {
		if precision, err := strconv.ParseInt(set.Get("precision"), 10, 64); err == nil {
			opt.Precision = util.Int64(precision)
		}
	}
	if set.Has("scale") {
		if scale, err := strconv.ParseInt(set.Get("scale"), 10, 64); err == nil {
			opt.Scale = util.Int64(scale)
		}
	}
	if set.Has("not_null") || set.Has("notnull") {
		opt.NotNull = util.Bool(true)
	}
	if set.Has("unique") {
		opt.Unique = util.Bool(true)
	}
	if set.Has("comment") {
		opt.Comment = util.String(set.Get("comment"))
	}

	opt.DefaultValue = strings.TrimSpace(opt.DefaultValue)
	skipParseDefaultValue := strings.Contains(opt.DefaultValue, "(") &&
		strings.Contains(opt.DefaultValue, ")") || strings.ToLower(opt.DefaultValue) == "null" || opt.DefaultValue == ""

	t := sf.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var err error
	switch t.Kind() {
	case reflect.Bool:
		opt.DataType = Bool
		if opt.HasDefaultValue && !skipParseDefaultValue {
			if opt.DefaultValueInterface, err = strconv.ParseBool(opt.DefaultValue); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for bool, got error: %v", opt.DefaultValue, err)
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		opt.DataType = Int
		if opt.HasDefaultValue && !skipParseDefaultValue {
			if opt.DefaultValueInterface, err = strconv.ParseInt(opt.DefaultValue, 0, 64); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for int, got error: %v", opt.DefaultValue, err)
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		opt.DataType = Uint
		if opt.HasDefaultValue && !skipParseDefaultValue {
			if opt.DefaultValueInterface, err = strconv.ParseUint(opt.DefaultValue, 0, 64); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for uint, got error: %v", opt.DefaultValue, err)
			}
		}
	case reflect.Float32, reflect.Float64:
		opt.DataType = Float
		if opt.HasDefaultValue && !skipParseDefaultValue {
			if opt.DefaultValueInterface, err = strconv.ParseFloat(opt.DefaultValue, 64); err != nil {
				return nil, fmt.Errorf("failed to parse %s as default value for float, got error: %v", opt.DefaultValue, err)
			}
		}
	case reflect.String:
		opt.DataType = String

		if opt.HasDefaultValue && !skipParseDefaultValue {
			opt.DefaultValue = strings.Trim(opt.DefaultValue, "'")
			opt.DefaultValue = strings.Trim(opt.DefaultValue, `"`)
			opt.DefaultValueInterface = opt.DefaultValue
		}
	case reflect.Struct:
		if t.String() == "time.Time" {
			opt.DataType = Time
		} else {
			if opt.Inline {
				// unset name if has inline tag
				opt.Name = ""
			} else {
				opt.DataType = Bytes
			}
		}
	case reflect.Array, reflect.Slice, reflect.Map:
		//if t.Elem().Kind() == reflect.Uint8 {
		//	opt.DataType = Bytes
		//}
		opt.DataType = Bytes
	}

	if set.Has("auto_createtime") || (opt.Name == "created_at" && (opt.DataType == Time || opt.DataType == Int || opt.DataType == Uint)) {
		v := set.Get("auto_createtime")
		if opt.DataType == Time {
			opt.AutoCreatetime = UnixTime
		} else if strings.ToLower(v) == "nano" {
			opt.AutoCreatetime = UnixNanosecond
		} else if strings.ToLower(v) == "milli" {
			opt.AutoCreatetime = UnixMillisecond
		} else {
			opt.AutoCreatetime = UnixSecond
		}
	}

	if set.Has("auto_updatetime") || (opt.Name == "updated_at" && (opt.DataType == Time || opt.DataType == Int || opt.DataType == Uint)) {
		v := set.Get("auto_updatetime")
		if opt.DataType == Time {
			opt.AutoUpdatetime = UnixTime
		} else if strings.ToLower(v) == "nano" {
			opt.AutoUpdatetime = UnixNanosecond
		} else if strings.ToLower(v) == "milli" {
			opt.AutoUpdatetime = UnixMillisecond
		} else {
			opt.AutoUpdatetime = UnixSecond
		}
	}

	if set.Has("type") {
		val := set.Get("type")
		switch DataType(strings.ToLower(val)) {
		case Bool, Int, Uint, Float, String, Time, Bytes:
			opt.DataType = DataType(strings.ToLower(val))
		default:
			opt.DataType = DataType(val)
		}
	}

	if opt.Size == nil {
		switch t.Kind() {
		case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Float64:
			opt.Size = util.Int64(64)
		case reflect.Int8, reflect.Uint8:
			opt.Size = util.Int64(8)
		case reflect.Int16, reflect.Uint16:
			opt.Size = util.Int64(16)
		case reflect.Int32, reflect.Uint32, reflect.Float32:
			opt.Size = util.Int64(32)
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
