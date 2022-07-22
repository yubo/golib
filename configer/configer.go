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
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"github.com/yubo/golib/util/strvals"
	"github.com/yubo/golib/util/template"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type Configer interface {
	// Var: set config fields to yaml configfile reader and pflags.FlagSet from sample
	Var(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error

	Parse(opts ...ConfigerOption) (ParsedConfiger, error)
	// AddFlags: add configer flags to *pflag.FlagSet, like -f, --set, --set-string, --set-file
	AddFlags(fs *pflag.FlagSet)
	// ValueFiles: return files list, which set by --set-file
	ValueFiles() []string
	// Envs: return envs list from fields, used with flags
	Envs() []string
	// Envs: return flags list
	Flags() []string
}

type ParsedConfiger interface {
	// ValueFiles: return files list, which set by --set-file
	ValueFiles() []string
	// Envs: return envs list from fields, used with flags
	Envs() []string
	// Envs: return flags list
	Flags() []string

	//FlagSet() *pflag.FlagSet
	Set(path string, v interface{}) error
	GetConfiger(path string) ParsedConfiger
	GetRaw(path string) interface{}
	GetString(path string) string
	GetBool(path string) (bool, error)
	GetBoolDef(path string, def bool) bool
	GetFloat64(path string) (float64, error)
	GetFloat64Def(path string, def float64) float64
	GetInt64(path string) (int64, error)
	GetInt64Def(path string, def int64) int64
	GetInt(path string) (int, error)
	GetIntDef(path string, def int) int
	IsSet(path string) bool
	Read(path string, into interface{}) error
	String() string
}

var (
	DefaultConfiger = NewConfiger()
)

func NewConfiger() Configer {
	return newConfiger()
}

func Parse(opts ...ConfigerOption) (ParsedConfiger, error) {
	return DefaultConfiger.Parse(opts...)
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
func FlagSet(fs *pflag.FlagSet, sample interface{}, opts ...ConfigFieldsOption) error {
	return NewConfiger().Var(fs, "", sample, opts...)
}

// Var: set config fields to yaml configfile reader and pflags.FlagSet from sample
func Var(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	return DefaultConfiger.Var(fs, path, sample, opts...)
}

type configer struct {
	*ConfigerOptions

	// factory options
	valueFiles   []string       // files, -f/--values
	values       []string       // values, --set
	stringValues []string       // values, --set-string
	fileValues   []string       // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	fields       []*configField // all of config fields
	//fs           *pflag.FlagSet

	// instance data
	data   map[string]interface{}
	env    map[string]interface{}
	path   []string
	parsed bool
	//samples []*ConfigFields
}

type ConfigFields struct {
	fs     *pflag.FlagSet
	path   string
	sample interface{}
	opts   []ConfigFieldsOption
}

func newConfiger() *configer {
	return &configer{
		ConfigerOptions: newConfigerOptions(),
		data:            map[string]interface{}{},
		env:             map[string]interface{}{},
		path:            []string{},
	}
}

func (p *configer) Parse(opts ...ConfigerOption) (ParsedConfiger, error) {
	for _, opt := range opts {
		opt(p.ConfigerOptions)
	}

	if err := p.ConfigerOptions.Validate(); err != nil {
		return nil, err
	}

	if err := p.parse(); err != nil {
		return nil, err
	}

	return p, nil
}

//func (p *configer) FlagSet() *pflag.FlagSet {
//	return p.fs
//}

func (p *configer) PrintFlags(out io.Writer) {
	fmt.Fprintf(out, "configer FLAG:\n")
	for _, value := range p.valueFiles {
		fmt.Fprintf(out, "  --values=%s\n", value)
	}
	for _, value := range p.values {
		fmt.Fprintf(out, "  --set=%s\n", value)
	}
	for _, value := range p.stringValues {
		fmt.Fprintf(out, "  --set-string=%s\n", value)
	}
	for _, value := range p.fileValues {
		fmt.Fprintf(out, "  --set-file=%s\n", value)
	}
}

func (p *configer) parse() (err error) {
	if p.parsed {
		return nil
	}

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
		klog.V(1).InfoS("config load", "filePath", filePath)
	}

	// User specified a value via --set
	for _, value := range p.values {
		if err := strvals.ParseInto(value, base); err != nil {
			return fmt.Errorf("failed parsing --set data: %s", err)
		}
		klog.V(1).InfoS("config load", "value", value)
	}

	// User specified a value via --set-string
	for _, value := range p.stringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return fmt.Errorf("failed parsing --set-string data: %s", err)
		}
		klog.V(1).InfoS("config load", "filepath(string)", value)
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
		klog.V(1).InfoS("config load", "set-file", value)
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
		if v := p.getFlagValue(f); v != nil {
			klog.V(10).InfoS("flag", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToValues(joinPath(append(p.path, f.configPath)...), v))
		}
	}

	return into
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

func (p *configer) Set(path string, v interface{}) error {
	if path == "" {
		b, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(b, &p.data)
	}

	ps := strings.Split(path, ".")
	src := map[string]interface{}{ps[len(ps)-1]: v}

	for i := len(ps) - 2; i >= 0; i-- {
		src = map[string]interface{}{ps[i]: src}
	}

	p.data = mergeValues(p.data, src)

	return nil
}

func (p *configer) GetConfiger(path string) ParsedConfiger {
	if p == nil || !p.parsed {
		return nil
	}

	data, _ := p.GetRaw(path).(map[string]interface{})

	// noneed deepCopy
	out := new(configer)
	*out = *p

	out.path = append(clonePath(p.path), parsePath(path)...)
	out.data = data

	return out
}

func (p *configer) GetRaw(path string) interface{} {
	if path == "" {
		return Values(p.data)
	}

	v, err := Values(p.data).PathValue(path)
	if err != nil {
		klog.V(5).InfoS("get pathValue err, ignored", "path", path, "v", v, "err", err)
		return nil
	}
	return v
}

func (p *configer) GetString(path string) string {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return ""
	}

	return cast.ToString(v)
}

func (p *configer) GetBool(path string) (bool, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return false, err
	}

	return cast.ToBool(v), nil
}

func (p *configer) GetBoolDef(path string, def bool) bool {
	v, err := p.GetBool(path)
	if err != nil {
		return def
	}
	return v
}

func (p *configer) GetFloat64(path string) (float64, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return 0, err
	}

	return cast.ToFloat64(v), nil
}

func (p *configer) GetFloat64Def(path string, def float64) float64 {
	v, err := p.GetFloat64(path)
	if err != nil {
		return def
	}

	return v
}

func (p *configer) GetInt64(path string) (int64, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return cast.ToInt64(v), nil
}

func (p *configer) GetInt64Def(path string, def int64) int64 {
	v, err := p.GetInt64(path)
	if err != nil {
		return def
	}
	return v
}

func (p *configer) GetInt(path string) (int, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return cast.ToInt(v), nil
}

func (p *configer) GetIntDef(path string, def int) int {
	v, err := p.GetInt(path)
	if err != nil {
		return def
	}
	return v
}

type validator interface {
	Validate() error
}

func (p *configer) IsSet(path string) bool {
	_, err := Values(p.data).PathValue(path)
	return err == nil
}

func (p *configer) Read(path string, into interface{}) error {
	if into == nil {
		return nil
	}

	if v := p.GetRaw(path); v != nil {
		data, err := yaml.Marshal(v)
		//klog.V(5).InfoS("marshal", "v", v, "data", string(data), "err", err)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(data, into)
		if err != nil {
			klog.V(5).InfoS("unmarshal", "data", string(data), "err", err)
			if klog.V(5).Enabled() {
				panic(err)
			}
			return err
		}
	}

	if v, ok := into.(validator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}

	if klog.V(10).Enabled() {
		b, _ := yaml.Marshal(into)
		klog.Infof("Read \n[%s]\n%s", path, string(b))
	}
	return nil
}

func (p *configer) String() string {
	buf, err := yaml.Marshal(p.data)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

func (p *configer) Document() string {
	buf, err := yaml.Marshal(p.data)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

func (p *configer) getEnv(key string) (string, bool) {
	val, ok := os.LookupEnv(key)
	return val, ok && (p.allowEmptyEnv || val != "")
}

// AddFlags: add configer flags to *pflag.FlagSet, like -f, --set, --set-string, --set-file
func (p *configer) AddFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&p.valueFiles, "values", "f", p.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&p.values, "set", p.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.stringValues, "set-string", p.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.fileValues, "set-file", p.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

// ValueFiles: return files list, which set by --set-file
func (p *configer) ValueFiles() []string {
	return append(p.valueFiles, p.filesOverride...)
}

// Var: set config fields to yaml configfile reader and pflags.FlagSet from sample
// defualt priority sample > tagsGetter > tags
func (p *configer) Var(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	if p == nil {
		return errors.New("configer pointer is nil")
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

	if err := p._var(parsePath(path), fs, rv, rt, o); err != nil {
		return err
	}

	return nil
}

func indirectValue(rv reflect.Value, rt reflect.Type) (reflect.Value, reflect.Type) {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rt.Elem()))
		}
		rv = rv.Elem()
		rt = rv.Type()
	}
	return rv, rt
}

func (p *configer) _var(path []string, fs *pflag.FlagSet, _rv reflect.Value, _rt reflect.Type, opt *configFieldsOptions) error {
	if len(path) > p.maxDepth {
		return fmt.Errorf("path.depth is larger than the maximum allowed depth of %d", p.maxDepth)
	}

	for i := 0; i < _rv.NumField(); i++ {
		sf := _rt.Field(i)
		rv := _rv.Field(i)
		rt := rv.Type()

		if !rv.CanSet() {
			continue
		}

		rv, rt = indirectValue(rv, rt)

		tag := opt.getTagOpts(sf, path)
		if tag.skip {
			continue
		}
		//klog.InfoS("getTagOpts", "flag", tag.Flag, "def", tag.Default)

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
			field = newConfigField(value, fs, ps, tag, fs.BoolVar, fs.BoolVarP, cast.ToBool(def))
		case *string:
			field = newConfigField(value, fs, ps, tag, fs.StringVar, fs.StringVarP, cast.ToString(def))
		case *int:
			field = newConfigField(value, fs, ps, tag, fs.IntVar, fs.IntVarP, cast.ToInt(def))
		case *int8:
			field = newConfigField(value, fs, ps, tag, fs.Int8Var, fs.Int8VarP, cast.ToInt8(def))
		case *int16:
			field = newConfigField(value, fs, ps, tag, fs.Int16Var, fs.Int16VarP, cast.ToInt16(def))
		case *int32:
			field = newConfigField(value, fs, ps, tag, fs.Int32Var, fs.Int32VarP, cast.ToInt32(def))
		case *int64:
			field = newConfigField(value, fs, ps, tag, fs.Int64Var, fs.Int64VarP, cast.ToInt64(def))
		case *uint:
			field = newConfigField(value, fs, ps, tag, fs.UintVar, fs.UintVarP, cast.ToUint(def))
		case *uint8:
			field = newConfigField(value, fs, ps, tag, fs.Uint8Var, fs.Uint8VarP, cast.ToUint8(def))
		case *uint16:
			field = newConfigField(value, fs, ps, tag, fs.Uint16Var, fs.Uint16VarP, cast.ToUint16(def))
		case *uint32:
			field = newConfigField(value, fs, ps, tag, fs.Uint32Var, fs.Uint32VarP, cast.ToUint32(def))
		case *uint64:
			field = newConfigField(value, fs, ps, tag, fs.Uint64Var, fs.Uint64VarP, cast.ToUint64(def))
		case *float32:
			field = newConfigField(value, fs, ps, tag, fs.Float32Var, fs.Float32VarP, cast.ToFloat32(def))
		case *float64:
			field = newConfigField(value, fs, ps, tag, fs.Float64Var, fs.Float64VarP, cast.ToFloat64(def))
		case *time.Duration:
			field = newConfigField(value, fs, ps, tag, fs.DurationVar, fs.DurationVarP, cast.ToDuration(def))
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

			// iterate struct{}
			if rt.Kind() == reflect.Struct {
				if err := p._var(curPath, fs, rv, rt, opt); err != nil {
					return err
				}
				continue
			}

			// set field.default
			field = newConfigField(value, fs, ps, tag, nil, nil, cast.ToStringMapString(def))
		}
		p.fields = append(p.fields, field)
	}
	return nil
}

func (p *configer) getFlagValue(f *configField) interface{} {
	if f.flag != "" && f.fs.Changed(f.flag) {
		return reflect.ValueOf(f.flagValue).Elem().Interface()
	}

	return nil
}

func (p *configer) GetDefault(path string) (interface{}, bool) {
	return "", false
}

func (p *configer) GetDescription(path string) (string, bool) {
	return "", false
}
