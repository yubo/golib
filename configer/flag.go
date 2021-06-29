package configer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	cliflag "github.com/yubo/golib/staging/cli/flag"
)

var (
	flags    []*param
	maxDepth = 5
)

type param struct {
	flag     string      // flag
	shothand string      // flag shothand
	path     string      // json path
	value    interface{} // flag's value
	env      string
}

// once called by Prepare
func (p *Configer) parseFlag() {
	if !p.enableFlag {
		return
	}

	for _, f := range flags {
		val := p.getFlagValue(f)
		if val == nil {
			continue
		}
		mergeValues(p.data, pathValueToTable(joinPath(append(p.path, f.path)...), val))
	}
}

func pathValueToTable(path string, val interface{}) map[string]interface{} {
	paths := parsePath(path)
	p := val

	for i := len(paths) - 1; i >= 0; i-- {
		p = map[string]interface{}{paths[i]: p}
	}
	return p.(map[string]interface{})
}

func (p *Configer) getFlagValue(f *param) interface{} {
	if p.fs.Changed(f.flag) {
		return reflect.ValueOf(f.value).Elem().Interface()
	}

	if !p.enableEnv || f.env == "" {
		return nil
	}

	val, ok := p.getEnv(f.env)
	if !ok {
		return nil
	}

	switch reflect.ValueOf(f.value).Elem().Interface().(type) {
	case bool:
		return cast.ToBool(val)
	case string:
		return cast.ToString(val)
	case int32, int16, int8, int:
		return cast.ToInt(val)
	case uint:
		return cast.ToUint(val)
	case uint32:
		return cast.ToUint32(val)
	case uint64:
		return cast.ToUint64(val)
	case int64:
		return cast.ToInt64(val)
	case float64, float32:
		return cast.ToFloat64(val)
	//case time.Time:
	//	return cast.ToTime(val)
	case time.Duration:
		return cast.ToDuration(val)
	case []string:
		return cast.ToStringSlice(val)
	case []int:
		return cast.ToIntSlice(val)
	default:
		panic(fmt.Sprintf("unsupported type %s", reflect.TypeOf(f.value).Name()))
	}
}

func AddFlags(fs *pflag.FlagSet, path string, sample interface{}) error {
	rv := reflect.Indirect(reflect.ValueOf(sample))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		return fmt.Errorf("Addflag: sample must be a struct, got %v/%v", rv.Kind(), rt)
	}

	return addFlag(fs, parsePath(path), rt)
}

func addFlag(fs *pflag.FlagSet, path []string, rt reflect.Type) error {
	if len(path) > maxDepth {
		return fmt.Errorf("path.depth(%s) is larger than the maximum allowed depth of %d", path, maxDepth)
	}

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
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
		} else if isUnexported {
			// Ignore unexported non-embedded fields.
			continue
		}

		opt := getTagOpt(sf)
		if opt.skip {
			continue
		}

		ft := sf.Type
		if ft.Name() == "" && ft.Kind() == reflect.Ptr {
			// Follow pointer.
			ft = ft.Elem()
		}

		if ft.Kind() == reflect.Struct {
			if opt.json == "" {
				// anonymous
				if err := addFlag(fs, path, ft); err != nil {
					return err
				}
				continue
			}

			if err := addFlag(fs, append(path, opt.json), ft); err != nil {
				return err
			}
			continue
		}

		ps := strings.Join(append(path, opt.json), ".")

		switch reflect.New(ft).Elem().Interface().(type) {
		case bool:
			addFlagCall(fs, ps, opt, fs.Bool, fs.BoolP, cast.ToBool(opt.def))
		case string:
			addFlagCall(fs, ps, opt, fs.String, fs.StringP, cast.ToString(opt.def))
		case int32, int16, int8, int:
			addFlagCall(fs, ps, opt, fs.Int, fs.IntP, cast.ToInt(opt.def))
		case int64:
			addFlagCall(fs, ps, opt, fs.Int64, fs.Int64P, cast.ToInt64(opt.def))
		case uint:
			addFlagCall(fs, ps, opt, fs.Uint, fs.UintP, cast.ToUint(opt.def))
		case uint8:
			addFlagCall(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint8(opt.def))
		case uint32:
			addFlagCall(fs, ps, opt, fs.Uint32, fs.Uint32P, cast.ToUint32(opt.def))
		case uint64:
			addFlagCall(fs, ps, opt, fs.Uint64, fs.Uint64P, cast.ToUint64(opt.def))
		case float32, float64:
			addFlagCall(fs, ps, opt, fs.Float64, fs.Float64P, cast.ToFloat64(opt.def))
		case time.Duration:
			addFlagCall(fs, ps, opt, fs.Duration, fs.DurationP, cast.ToDuration(opt.def))
		case []string:
			addFlagCall(fs, ps, opt, fs.StringArray, fs.StringArrayP, cast.ToStringSlice(opt.def))
		case []int:
			addFlagCall(fs, ps, opt, fs.IntSlice, fs.IntSliceP, cast.ToIntSlice(opt.def))
		default:
			panic(fmt.Sprintf("unsupported type %s", ft.Name()))

		}
	}
	return nil
}

type tagOpt struct {
	json        string // for path
	flag        []string
	def         string
	flagShort   string
	env         string
	description string
	skip        bool
}

func (p tagOpt) String() string {
	return fmt.Sprintf("json %s flag %v env %s description %s",
		p.json, p.flag, p.env, p.description)
}

func getTagOpt(sf reflect.StructField) (opt *tagOpt) {
	opt = &tagOpt{}
	if sf.Anonymous {
		return
	}

	if json, opts := parseTag(sf.Tag.Get("json")); json == "-" {
		opt.skip = true
		return
	} else if json != "" {
		opt.json = json
	} else if opts.Contains("inline") {
		return
	} else {
		opt.skip = true
		return
	}

	if flag := strings.Split(strings.TrimSpace(sf.Tag.Get("flag")), ","); len(flag) == 0 || flag[0] == "" {
		opt.skip = true
	} else {
		opt.flag = flag
	}
	opt.def = sf.Tag.Get("default")
	opt.description = sf.Tag.Get("description")
	opt.env = sf.Tag.Get("env")
	if opt.env != "" {
		opt.description = fmt.Sprintf("%s (env %s)", opt.description, opt.env)
	}

	return
}

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
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

func addFlagCall(fs *pflag.FlagSet, path string, opt *tagOpt, varFn, varPFn, def interface{}) {
	var ret []reflect.Value
	if len(opt.flag) == 1 {
		ret = reflect.ValueOf(varFn).Call([]reflect.Value{
			reflect.ValueOf(opt.flag[0]),
			reflect.ValueOf(def),
			reflect.ValueOf(opt.description),
		})
	}
	if len(opt.flag) > 1 {
		ret = reflect.ValueOf(varPFn).Call([]reflect.Value{
			reflect.ValueOf(opt.flag[0]),
			reflect.ValueOf(opt.flag[1]),
			reflect.ValueOf(def),
			reflect.ValueOf(opt.description),
		})
	}

	var val interface{}
	if ret[0].CanInterface() {
		val = ret[0].Interface()
	}

	if val != nil {
		flags = append(flags, &param{
			flag:     opt.flag[0],
			shothand: "",
			path:     path,
			value:    val,
			env:      opt.env,
		})
	}
}

func NamedFlagSets() *cliflag.NamedFlagSets {
	return &Setting.namedFlagSets
}
