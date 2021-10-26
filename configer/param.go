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
	defaultValue interface{} // field's default value
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
			//klog.V(7).InfoS("def", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToTable(joinPath(append(p.path, f.configPath)...), v))
		}
	}
}

// merge flag value into ${into}
func (p *Configer) mergeFlagValues(into map[string]interface{}) {
	if !p.enableFlag {
		return
	}
	for _, f := range p.params {
		if v := p.getFlagValue(f); v != nil {
			//klog.V(7).InfoS("flag", "path", joinPath(append(p.path, f.configPath)...), "value", v)
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

		opt := p.getTagOpts(sf, path)
		if opt.skip {
			continue
		}

		ft := sf.Type
		if ft.Kind() == reflect.Ptr {
			// Follow pointer.
			ft = ft.Elem()
		}

		curPath := make([]string, len(path))
		copy(curPath, path)

		if len(opt.json) > 0 {
			curPath = append(curPath, opt.json)
		}

		if len(opt.Flag) == 0 && ft.Kind() == reflect.Struct {
			if err := p.addConfigs(curPath, fs, ft); err != nil {
				return err
			}
			continue
		}

		ps := joinPath(curPath...)
		def := getDefaultValue(ps, opt, p)

		switch sample := reflect.New(ft).Interface().(type) {
		case pflag.Value:
			addConfigFieldByValue(fs, ps, opt, sample, def)
		case *bool:
			addConfigField(fs, ps, opt, fs.Bool, fs.BoolP, cast.ToBool(def))
		case *string:
			addConfigField(fs, ps, opt, fs.String, fs.StringP, cast.ToString(def))
		case *int32, *int16, *int8, *int:
			addConfigField(fs, ps, opt, fs.Int, fs.IntP, cast.ToInt(def))
		case *int64:
			addConfigField(fs, ps, opt, fs.Int64, fs.Int64P, cast.ToInt64(def))
		case *uint:
			addConfigField(fs, ps, opt, fs.Uint, fs.UintP, cast.ToUint(def))
		case *uint8:
			addConfigField(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint8(def))
		case *uint16:
			addConfigField(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint16(def))
		case *uint32:
			addConfigField(fs, ps, opt, fs.Uint32, fs.Uint32P, cast.ToUint32(def))
		case *uint64:
			addConfigField(fs, ps, opt, fs.Uint64, fs.Uint64P, cast.ToUint64(def))
		case *float32, *float64:
			addConfigField(fs, ps, opt, fs.Float64, fs.Float64P, cast.ToFloat64(def))
		case *time.Duration:
			addConfigField(fs, ps, opt, fs.Duration, fs.DurationP, cast.ToDuration(def))
		case *[]string:
			addConfigField(fs, ps, opt, fs.StringArray, fs.StringArrayP, cast.ToStringSlice(def))
		case *[]int:
			addConfigField(fs, ps, opt, fs.IntSlice, fs.IntSliceP, cast.ToIntSlice(def))
		case *map[string]string:
			addConfigField(fs, ps, opt, fs.StringToString, fs.StringToStringP, cast.ToStringMapString(def))
		default:
			klog.InfoS("add config unsupported", "type", ft.String(), "path", ps, "kind", ft.Kind())
		}
	}
	return nil
}

func (p *options) getTagOpts(sf reflect.StructField, paths []string) *TagOpts {
	opts := getTagOpts(sf, p)

	if p.tags != nil {
		path := strings.TrimPrefix(joinPath(append(paths, opts.json)...), p.prefixPath+".")
		if o := p.tags[path]; o != nil {
			if len(o.Flag) > 0 {
				opts.Flag = o.Flag
			}
			if len(o.Description) > 0 {
				opts.Description = o.Description
			}
			if len(o.Default) > 0 {
				opts.Default = o.Default
			}
			if len(o.Env) > 0 {
				opts.Env = o.Env
			}
		}
	}

	return opts
}

type TagOpts struct {
	name string // field name
	json string // json:"{json}"
	skip bool   // if json:"-"

	Flag        []string // flag:"{long},{short}"
	Default     string   // default:"{default}"
	Env         string   // env:"{env}"
	Description string   // description:"{description}"
	Arg         string   // arg:"{arg}"  args[0] arg1... -- arg2... (deprecated)

}

func (p TagOpts) Skip() bool {
	return p.skip
}

func (p TagOpts) String() string {
	return fmt.Sprintf("json %s flag %v env %s description %s",
		p.json, p.Flag, p.Env, p.Description)
}

func getTagOpts(sf reflect.StructField, o *options) (tag *TagOpts) {
	tag = &TagOpts{name: sf.Name}
	if sf.Anonymous {
		return
	}

	json, opts := parseTag(sf.Tag.Get("json"))
	if json == "-" {
		tag.skip = true
		return
	}

	if json != "" {
		tag.json = json
	} else {
		tag.json = sf.Name
	}

	if opts.Contains("arg1") {
		tag.Arg = "arg1"
	}
	if opts.Contains("arg2") {
		tag.Arg = "arg2"
	}

	if flag := strings.Split(strings.TrimSpace(sf.Tag.Get("flag")), ","); len(flag) > 0 && flag[0] != "" && flag[0] != "-" {
		tag.Flag = flag
	}

	tag.Default = sf.Tag.Get("default")
	tag.Description = sf.Tag.Get("description")
	tag.Env = strings.Replace(strings.ToUpper(sf.Tag.Get("env")), "-", "_", -1)
	if tag.Env != "" {
		tag.Description = fmt.Sprintf("%s (env %s)", tag.Description, tag.Env)
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

func addConfigFieldByValue(fs *pflag.FlagSet, path string, opt *TagOpts, value pflag.Value, defValue string) {
	rt := reflect.Indirect(reflect.ValueOf(value)).Type()
	def := reflect.New(rt).Interface().(pflag.Value)

	// set value
	if defValue != "" {
		def.Set(defValue)
		value.Set(defValue)
	}

	v := &param{
		configPath: path,
		envName:    opt.Env,
		flagValue:  value,
	}

	if opt.Default != "" {
		v.defaultValue = def
	}

	switch len(opt.Flag) {
	case 0:
	// nothing
	case 1:
		v.flag = opt.Flag[0]
		fs.Var(value, opt.Flag[0], opt.Description)
	case 2:
		v.flag = opt.Flag[0]
		v.shothand = opt.Flag[1]
		fs.VarP(value, opt.Flag[0], opt.Flag[1], opt.Description)
	default:
		panic("invalid flag value")
	}

	configerOptions.params = append(configerOptions.params, v)
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

	configerOptions.params = append(configerOptions.params, v)
}

// env > value from registered config > structField tag
func getDefaultValue(path string, opts *TagOpts, o *options) string {
	// env
	if o.enableEnv && opts.Env != "" {
		if def, ok := o.getEnv(opts.Env); ok {
			if len(def) > 0 {
				opts.Default = def
				return def
			}
		}
	}

	// sample value
	if v, err := Values(o.defaultValues).PathValue(path); err == nil {
		if !isZero(v) {
			if def := cast.ToString(v); len(def) > 0 {
				// ugly hack! for ignore time.Duration
				if def != "0s" {
					opts.Default = def
					return def
				}
			}
		}
	}

	return opts.Default
}

func isZero(in interface{}) bool {
	if in == nil {
		return true
	}

	return reflect.ValueOf(in).IsZero()
}
