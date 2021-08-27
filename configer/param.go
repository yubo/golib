package configer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

type param struct {
	envName      string      // env name
	flag         string      // flag
	shothand     string      // flag shothand
	configPath   string      // config path
	flagValue    interface{} // flag's value
	defaultValue interface{} // flag's default value
}

func pathValueToTable(path string, val interface{}) map[string]interface{} {
	paths := parsePath(path)
	p := val

	for i := len(paths) - 1; i >= 0; i-- {
		p = map[string]interface{}{paths[i]: p}
	}
	return p.(map[string]interface{})
}

func (p *Configer) Envs() (names []string) {
	if !p.enableEnv {
		return
	}
	for _, f := range p.params {
		if f.envName != "" {
			names = append(names, f.envName)
		}
	}
	return
}

func (p *Configer) Flags() (names []string) {
	if !p.enableFlag {
		return
	}
	for _, f := range p.params {
		if f.flag != "" {
			names = append(names, f.flag)
		}
	}
	return
}

func (p *Configer) mergeDefaultValues(into map[string]interface{}) {
	for _, f := range p.params {
		if v := f.defaultValue; v != nil {
			klog.V(7).InfoS("def", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToTable(joinPath(append(p.path, f.configPath)...), v))
		}
	}
}

func (p *Configer) mergeFlagValues(into map[string]interface{}) {
	if !p.enableFlag {
		return
	}
	for _, f := range p.params {
		if v := p.getFlagValue(f); v != nil {
			klog.V(7).InfoS("flag", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToTable(joinPath(append(p.path, f.configPath)...), v))
		}
	}
}

func (p *Configer) mergeEnvValues(into map[string]interface{}) {
	if !p.enableEnv {
		return
	}
	for _, f := range p.params {
		if v := p.getEnvValue(f); v != nil {
			klog.V(7).InfoS("env", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToTable(joinPath(append(p.path, f.configPath)...), v))
		}
	}
}

func (p *Configer) getFlagValue(f *param) interface{} {
	if f.flag == "" {
		return nil
	}

	if p.flagSet.Changed(f.flag) {
		return reflect.ValueOf(f.flagValue).Elem().Interface()
	}

	return nil
}

func (p *Configer) getEnvValue(f *param) interface{} {
	if f.envName == "" {
		return nil
	}

	val, ok := p.getEnv(f.envName)
	if !ok {
		return nil
	}

	switch reflect.ValueOf(f.flagValue).Elem().Interface().(type) {
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
		panic(fmt.Sprintf("unsupported type %s", reflect.TypeOf(f.flagValue).Name()))
	}
}

// addConfigs: add flags and env from sample's tags
func AddConfigs(fs *pflag.FlagSet, path string, sample interface{}) error {
	rv := reflect.Indirect(reflect.ValueOf(sample))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		return fmt.Errorf("Addflag: sample must be a struct, got %v/%v", rv.Kind(), rt)
	}

	return Options.addConfigs(parsePath(path), fs, rt)
}

func (p *options) addConfigs(path []string, fs *pflag.FlagSet, rt reflect.Type) error {
	if len(path) > p.maxDepth {
		return fmt.Errorf("path.depth is larger than the maximum allowed depth of %d", p.maxDepth)
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

		opt := GetTagOpts(sf)
		if opt.Skip {
			continue
		}

		ft := sf.Type
		if ft.Kind() == reflect.Ptr {
			// Follow pointer.
			ft = ft.Elem()
		}

		if ft.Kind() == reflect.Struct {
			if opt.Json == "" {
				// anonymous
				if err := p.addConfigs(path, fs, ft); err != nil {
					return err
				}
				continue
			}

			if err := p.addConfigs(append(path, opt.Json), fs, ft); err != nil {
				return err
			}
			continue
		}

		ps := strings.Join(append(path, opt.Json), ".")

		switch t := reflect.New(ft).Elem().Interface(); t.(type) {
		case bool:
			addConfigField(fs, ps, opt, fs.Bool, fs.BoolP, cast.ToBool(opt.Default))
		case string:
			addConfigField(fs, ps, opt, fs.String, fs.StringP, cast.ToString(opt.Default))
		case int32, int16, int8, int:
			addConfigField(fs, ps, opt, fs.Int, fs.IntP, cast.ToInt(opt.Default))
		case int64:
			addConfigField(fs, ps, opt, fs.Int64, fs.Int64P, cast.ToInt64(opt.Default))
		case uint:
			addConfigField(fs, ps, opt, fs.Uint, fs.UintP, cast.ToUint(opt.Default))
		case uint8:
			addConfigField(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint8(opt.Default))
		case uint16:
			addConfigField(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint16(opt.Default))
		case uint32:
			addConfigField(fs, ps, opt, fs.Uint32, fs.Uint32P, cast.ToUint32(opt.Default))
		case uint64:
			addConfigField(fs, ps, opt, fs.Uint64, fs.Uint64P, cast.ToUint64(opt.Default))
		case float32, float64:
			addConfigField(fs, ps, opt, fs.Float64, fs.Float64P, cast.ToFloat64(opt.Default))
		case time.Duration:
			addConfigField(fs, ps, opt, fs.Duration, fs.DurationP, cast.ToDuration(opt.Default))
		case []string:
			addConfigField(fs, ps, opt, fs.StringArray, fs.StringArrayP, cast.ToStringSlice(opt.Default))
		case []int:
			addConfigField(fs, ps, opt, fs.IntSlice, fs.IntSliceP, cast.ToIntSlice(opt.Default))
		case map[string]string:
			addConfigField(fs, ps, opt, fs.StringToString, fs.StringToStringP, cast.ToStringMapString(opt.Default))
		default:
			klog.V(6).InfoS("add config unsupported", "type", ft.String(), "path", joinPath(path...), "kind", ft.Kind())
		}
	}
	return nil
}

type TagOpts struct {
	Name        string   // field name
	Json        string   // json:"{json}"
	Flag        []string // flag:"{long},{short}"
	Default     string   // default:"{default}"
	Env         string   // env:"{env}"
	Description string   // description:"{description}"
	Skip        bool     // if json:"-"
	Arg         string   // arg:"{arg}"
}

func (p TagOpts) String() string {
	return fmt.Sprintf("json %s flag %v env %s description %s",
		p.Json, p.Flag, p.Env, p.Description)
}

func GetTagOpts(sf reflect.StructField) (opt *TagOpts) {
	opt = &TagOpts{Name: sf.Name}
	if sf.Anonymous {
		return
	}

	json, _ := parseTag(sf.Tag.Get("json"))
	if json == "-" {
		opt.Skip = true
		return
	}

	if json != "" {
		opt.Json = json
	}

	if flag := strings.Split(strings.TrimSpace(sf.Tag.Get("flag")), ","); len(flag) > 0 && flag[0] != "" && flag[0] != "-" {
		opt.Flag = flag
	}

	opt.Default = sf.Tag.Get("default")
	opt.Description = sf.Tag.Get("description")
	opt.Arg = sf.Tag.Get("arg")
	opt.Env = sf.Tag.Get("env")
	if opt.Env != "" {
		opt.Description = fmt.Sprintf("%s (env %s)", opt.Description, opt.Env)
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

func addConfigField(fs *pflag.FlagSet, path string, opt *TagOpts, varFn, varPFn, def interface{}) {
	v := &param{
		configPath: path,
		envName:    opt.Env,
	}

	if opt.Default != "" {
		v.defaultValue = def
	}

	// add flag
	switch len(opt.Flag) {
	case 0:
		// nothing
	case 1:
		v.flag = opt.Flag[0]
		ret := reflect.ValueOf(varFn).Call([]reflect.Value{
			reflect.ValueOf(opt.Flag[0]),
			reflect.ValueOf(def),
			reflect.ValueOf(opt.Description),
		})
		v.flagValue = ret[0].Interface()
	case 2:
		v.flag = opt.Flag[0]
		v.shothand = opt.Flag[1]
		ret := reflect.ValueOf(varPFn).Call([]reflect.Value{
			reflect.ValueOf(opt.Flag[0]),
			reflect.ValueOf(opt.Flag[1]),
			reflect.ValueOf(def),
			reflect.ValueOf(opt.Description),
		})
		v.flagValue = ret[0].Interface()
	default:
		panic("invalid flag value")
	}

	Options.params = append(Options.params, v)
}
