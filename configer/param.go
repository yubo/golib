package configer

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
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

type TagOpts struct {
	name string // field name
	json string // json:"{json}"
	skip bool   // if json:"-"

	Flag        []string // flag:"{long},{short}"
	Default     string   // default:"{default}"
	Env         string   // env:"{env}"
	Description string   // description:"{description}"
	Deprecated  string   // deprecated:""
	Arg         string   // arg:"{arg}"  args[0] arg1... -- arg2... (deprecated)

}

func (p TagOpts) Skip() bool {
	return p.skip
}

func (p TagOpts) String() string {
	return fmt.Sprintf("json %s flag %v env %s description %s",
		p.json, p.Flag, p.Env, p.Description)
}

func getTagOpts(sf reflect.StructField, o *Options) (tag *TagOpts) {
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
	tag.Deprecated = sf.Tag.Get("deprecated")
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

type defaultSetter interface {
	SetDefault(string) error
}

func addConfigFieldByValue(fs *pflag.FlagSet, path string, opt *TagOpts, value pflag.Value, defValue string) {
	rt := reflect.Indirect(reflect.ValueOf(value)).Type()
	def := reflect.New(rt).Interface().(pflag.Value)

	// set value
	if defValue != "" {
		if d, ok := def.(defaultSetter); ok {
			d.SetDefault(defValue)
			value.(defaultSetter).SetDefault(defValue)
		} else {
			// the changed flag may be affected
			def.Set(defValue)
			value.Set(defValue)
		}
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

	if len(v.flag) > 0 && len(opt.Deprecated) > 0 {
		fs.MarkDeprecated(v.flag, opt.Deprecated)
		fs.Lookup(v.flag).Hidden = false
	}

	DefaultOptions.params = append(DefaultOptions.params, v)
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

	if len(v.flag) > 0 && len(opt.Deprecated) > 0 {
		fs.MarkDeprecated(v.flag, opt.Deprecated)
		fs.Lookup(v.flag).Hidden = false
	}

	DefaultOptions.params = append(DefaultOptions.params, v)
}

// env > value from registered config > structField tag
func getDefaultValue(path string, opts *TagOpts, o *Options) string {
	// env
	if o.enableEnv && opts.Env != "" {
		if def, ok := o.getEnv(opts.Env); ok {
			if len(def) > 0 {
				opts.Default = def
				return def
			}
		}
	}

	if v, err := Values(o.defaultValues).PathValue(path); err == nil {
		if !isZero(v) {
			if def := cast.ToString(v); len(def) > 0 {
				opts.Default = def
				return def
			}
		}
	}

	return opts.Default
}

type zeroChecker interface {
	IsZero() bool
}

func isZero(in interface{}) bool {
	if in == nil {
		return true
	}
	if f, ok := in.(zeroChecker); ok {
		return f.IsZero()
	}

	return reflect.ValueOf(in).IsZero()
}
