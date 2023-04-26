// the configer is not thread safe,
// make sure not use it after call process.Start()
package configer

// ## value priority(desc order):
//  - WithOverrideYaml(), WithOverride()
//  - flag (os.Args)
//    - args flag
//    - fileValues (--set-file)
//    - value (--set, --set-string)
//    - valueFile (--values, -f)
//  - default
//    - RegisterConfigFields.sample.field.tags.env
//    - WithDefaultYaml(), WithDefault()
//    - RegisterConfigFields.WithTags(tags.default)
//    - RegisterConfigFields.sample.field.value
//    - RegisterConfigFields.sample.field.tags.default

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/spf13/pflag"
	"github.com/yubo/golib/util"
	"github.com/yubo/golib/util/strvals"
	"github.com/yubo/golib/util/template"
	"github.com/yubo/golib/util/validation/field"
	"github.com/yubo/golib/util/yaml"
)

var (
	DEBUG           = false
	DefaultConfiger = New()
)

type Configer interface {
	// Var: set config fields to yaml configfile reader and pflags.FlagSet from sample
	Var(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error
	// Parse
	Parse(opts ...ConfigerOption) (ParsedConfiger, error)
	// MustParse
	MustParse(opts ...ConfigerOption) ParsedConfiger
	// AddFlags: add configer flags to *pflag.FlagSet, like -f, --set, --set-string, --set-file
	AddFlags(fs *pflag.FlagSet)
	// ValueFiles: return files list, which set by --set-file
	ValueFiles() []string
	// Envs: return envs list from fields, used with flags
	Envs() []string
	// Envs: return flags list
	Flags() []string
}

func New() Configer {
	return newConfiger()
}

// Var: set config fields to yaml configfile reader and pflags.FlagSet from sample
func Var(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	return DefaultConfiger.Var(fs, path, sample, opts...)
}

// Parse:
func Parse(opts ...ConfigerOption) (ParsedConfiger, error) {
	return DefaultConfiger.Parse(opts...)
}

// MustParse:
func MustParse(opts ...ConfigerOption) ParsedConfiger {
	return DefaultConfiger.MustParse(opts...)
}

// AddFlags: add configer flags to *pflag.FlagSet, like -f, --set, --set-string, --set-file
func AddFlags(fs *pflag.FlagSet) {
	DefaultConfiger.AddFlags(fs)
}

// ValueFiles: return files list, which set by --set-file
func ValueFiles() []string {
	return DefaultConfiger.ValueFiles()
}

// Envs: return envs list from fields, used with flags
func Envs() []string {
	return DefaultConfiger.Envs()
}

// Envs: return flags list
func Flags() []string {
	return DefaultConfiger.Flags()
}

// FalgSet: set config fields to pflags.FlagSet from sample
//func FlagSet(fs *pflag.FlagSet, sample interface{}, opts ...ConfigFieldsOption) error {
//	return NewConfiger().Var(fs, "", sample, opts...)
//}

var _ Configer = new(configer)

func newConfiger() *configer {
	return &configer{
		ConfigerOptions: newConfigerOptions(),
		data:            map[string]interface{}{},
		env:             map[string]interface{}{},
		path:            []string{},
	}
}

type configer struct {
	*ConfigerOptions

	valueFiles   []string       // files, -f/--values
	values       []string       // values, --set servers[0].port=80
	stringValues []string       // values, --set-string servers[0].name=007
	fileValues   []string       // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	fields       []*configField // all of config fields

	data   map[string]interface{}
	env    map[string]interface{}
	path   []string
	parsed bool
}

func (p *configer) Path(names ...string) (ret *field.Path) {
	for _, v := range append(p.path, names...) {
		if v != "" {
			ret.Child(v)
		}
	}
	return ret
}

// ValueFiles: return files list, which set by --set-file
func (p *configer) ValueFiles() []string {
	return append(p.valueFiles, p.filesOverride...)
}

// Envs: return envs list from fields, used with flags
func (p *configer) Envs() (names []string) {
	if !p.enableEnv {
		return
	}
	for _, f := range p.fields {
		if f.envName != "" {
			names = append(names, f.envName)
		}
	}
	return
}

// Envs: return flags list
func (p *configer) Flags() (names []string) {
	for _, f := range p.fields {
		if f.flag != "" {
			names = append(names, f.flag)
		}
	}
	return
}

// AddFlags: add configer flags to *pflag.FlagSet, like -f, --set, --set-string, --set-file
func (p *configer) AddFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&p.valueFiles, "values", "f", p.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&p.values, "set", p.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.stringValues, "set-string", p.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.fileValues, "set-file", p.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

// Var: set config fields to yaml configfile reader and pflags.FlagSet from sample
// defualt priority sample > tagsGetter > tags
func (p *configer) Var(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	if p == nil {
		return fmt.Errorf("configer pointer is nil")
	}

	o := newConfigFieldsOptions(p)
	for _, opt := range opts {
		opt(o)
	}
	o.prefixPath = path
	if o.tagsGetter != nil {
		o.tags = o.tagsGetter()
	}

	if values, err := objToValues(sample); err != nil {
		return err
	} else {
		o.defaultValues = pathValueToValues(path, values)
	}

	rv := reflect.ValueOf(sample)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("configer.Var() except a ptr to %s", rv.Kind())
	}

	rv = rv.Elem()
	rt := rv.Type()

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("Addflag: sample must be a struct, got %v/%v", rv.Kind(), rt)
	}

	if err := p.setVar(parsePath(path), fs, rv, rt, o); err != nil {
		return err
	}

	return nil
}

func (p *configer) setVar(path []string, fs *pflag.FlagSet, sampleRv reflect.Value, sampleRt reflect.Type, opt *configFieldsOptions) error {
	if len(path) > p.maxDepth {
		return fmt.Errorf("path.depth is larger than the maximum allowed depth of %d", p.maxDepth)
	}

	for i := 0; i < sampleRv.NumField(); i++ {
		sf := sampleRt.Field(i)
		rv := sampleRv.Field(i)
		rt := rv.Type()

		if !rv.CanSet() {
			continue
		}

		rv, rt = indirectValue(rv, rt)

		tag := opt.getTagOpts(sf, path)
		if tag.skip {
			continue
		}

		curPath := make([]string, len(path))
		copy(curPath, path)

		if len(tag.json) > 0 {
			curPath = append(curPath, tag.json)
		}

		// env
		ps := joinPath(curPath...)
		def := opt.getDefaultValue(ps, tag, p.ConfigerOptions)
		var field *configField

		switch value := rv.Addr().Interface().(type) {
		case pflag.Value:
			field = newConfigFieldByValue(value, fs, ps, tag, def)
		case *net.IP:
			var df net.IP
			if def != "" {
				df = net.ParseIP(def)
			}
			field = newConfigField(value, fs, ps, tag, fs.IPVar, fs.IPVarP, df)
		case *bool:
			field = newConfigField(value, fs, ps, tag, fs.BoolVar, fs.BoolVarP, util.ToBool(def))
		case *string:
			field = newConfigField(value, fs, ps, tag, fs.StringVar, fs.StringVarP, util.ToString(def))
		case *int:
			field = newConfigField(value, fs, ps, tag, fs.IntVar, fs.IntVarP, util.ToInt(def))
		case *int8:
			field = newConfigField(value, fs, ps, tag, fs.Int8Var, fs.Int8VarP, util.ToInt8(def))
		case *int16:
			field = newConfigField(value, fs, ps, tag, fs.Int16Var, fs.Int16VarP, util.ToInt16(def))
		case *int32:
			field = newConfigField(value, fs, ps, tag, fs.Int32Var, fs.Int32VarP, util.ToInt32(def))
		case *int64:
			field = newConfigField(value, fs, ps, tag, fs.Int64Var, fs.Int64VarP, util.ToInt64(def))
		case *uint:
			field = newConfigField(value, fs, ps, tag, fs.UintVar, fs.UintVarP, util.ToUint(def))
		case *uint8:
			field = newConfigField(value, fs, ps, tag, fs.Uint8Var, fs.Uint8VarP, util.ToUint8(def))
		case *uint16:
			field = newConfigField(value, fs, ps, tag, fs.Uint16Var, fs.Uint16VarP, util.ToUint16(def))
		case *uint32:
			field = newConfigField(value, fs, ps, tag, fs.Uint32Var, fs.Uint32VarP, util.ToUint32(def))
		case *uint64:
			field = newConfigField(value, fs, ps, tag, fs.Uint64Var, fs.Uint64VarP, util.ToUint64(def))
		case *float32:
			field = newConfigField(value, fs, ps, tag, fs.Float32Var, fs.Float32VarP, util.ToFloat32(def))
		case *float64:
			field = newConfigField(value, fs, ps, tag, fs.Float64Var, fs.Float64VarP, util.ToFloat64(def))
		case *time.Duration:
			field = newConfigField(value, fs, ps, tag, fs.DurationVar, fs.DurationVarP, util.ToDuration(def))
		case *[]string:
			field = newConfigField(value, fs, ps, tag, fs.StringArrayVar, fs.StringArrayVarP, ToStringArrayVar(def))
		case *[]int:
			field = newConfigField(value, fs, ps, tag, fs.IntSliceVar, fs.IntSliceVarP, ToIntSlice(def))
		case *[]float64:
			field = newConfigField(value, fs, ps, tag, fs.Float64SliceVar, fs.Float64SliceVarP, ToFloat64Slice(def))
		case *map[string]string:
			field = newConfigField(value, fs, ps, tag, fs.StringToStringVar, fs.StringToStringVarP, ToStringMapString(def))
		default:
			if len(tag.Flag) > 0 {
				panic(fmt.Sprintf("add config unsupported type %s path %s kind %s", rt.String(), ps, rt.Kind()))
			}

			switch rt.Kind() {
			// iterate struct{}
			case reflect.Struct:
				if err := p.setVar(curPath, fs, rv, rt, opt); err != nil {
					return err
				}
				continue
			default:
				// set field.default
				field = newConfigField(value, fs, ps, tag, nil, nil, util.ToStringMapString(def))
			}

		}
		p.fields = append(p.fields, field)
	}
	return nil
}

// MustParse
func (p *configer) MustParse(opts ...ConfigerOption) ParsedConfiger {
	pc, err := p.Parse(opts...)
	if err != nil {
		panic(err)
	}

	return pc
}

// Parse
func (p *configer) Parse(opts ...ConfigerOption) (ParsedConfiger, error) {
	if p.parsed {
		return nil, errors.New("already parsed")
	}

	for _, opt := range opts {
		opt(p.ConfigerOptions)
	}

	if err := p.ConfigerOptions.Validate(); err != nil {
		return nil, err
	}

	if err := p.parse(); err != nil {
		return nil, err
	}

	return &parsedConfiger{p}, nil
}

func (p *configer) parse() (err error) {
	base := map[string]interface{}{}

	// merge default from RegisterConfigFields.sample
	base = mergePathFields(base, p.path, p.fields)

	// merge WithDefault values
	base = mergeValues(base, p.defaultValues)

	// merge env from RegisterConfigFields.sample
	base = mergeValues(base, p.env)

	// configFile & valueFile --values
	for _, filePath := range append(p.valueFiles, p.filesOverride...) {
		m := map[string]interface{}{}

		bytes, err := template.ParseTemplateFile(nil, filePath)
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal(bytes, &m); err != nil {
			return fmt.Errorf("failed to parse %s: %s", filePath, err)
		}
		// Merge with the previous map
		base = mergeValues(base, m)
	}

	// User specified a value via --set
	for _, value := range p.values {
		if err := strvals.ParseInto(value, base); err != nil {
			return fmt.Errorf("failed parsing --set data: %s", err)
		}
	}

	// User specified a value via --set-string
	for _, value := range p.stringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return fmt.Errorf("failed parsing --set-string data: %s", err)
		}
		dlog("config load", "filepath(string)", value)
	}

	// User specified a value via --set-file
	for _, value := range p.fileValues {
		reader := func(rs []rune) (interface{}, error) {
			bytes, err := template.ParseTemplateFile(nil, string(rs))
			return string(bytes), err
		}
		if err := strvals.ParseIntoFile(value, base, reader); err != nil {
			return fmt.Errorf("failed parsing --set-file data: %s", err)
		}
	}

	base = p.mergeFlagValues(base)

	// override
	base = mergeValues(base, p.overrideValues)

	p.data = base
	p.parsed = true
	return nil
}

// merge flag value into ${into}
func (p *configer) mergeFlagValues(into map[string]interface{}) map[string]interface{} {
	for _, f := range p.fields {
		if v := f.getFlagValue(); v != nil {
			dlog("flag", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToValues(joinPath(append(p.path, f.configPath)...), v))
		}
	}

	return into
}
