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
	"io/ioutil"
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
	Register(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error
	Parse(opts ...ConfigerOption) (ParsedConfiger, error)
	AddFlags(fs *pflag.FlagSet)
	AddRegisteredFlags(fs *pflag.FlagSet)
	ValueFiles() []string
	Envs() []string
	Flags() []string
}

type ParsedConfiger interface {
	ValueFiles() []string
	Envs() []string
	Flags() []string

	FlagSet() *pflag.FlagSet
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

func AddFlags(fs *pflag.FlagSet) {
	DefaultConfiger.AddFlags(fs)
}
func AddRegisteredFlags(fs *pflag.FlagSet) {
	DefaultConfiger.AddRegisteredFlags(fs)
}
func ValueFiles() []string {
	return DefaultConfiger.ValueFiles()
}
func Envs() []string {
	return DefaultConfiger.Envs()
}
func Flags() []string {
	return DefaultConfiger.Flags()
}

func AddFlagsVar(fs *pflag.FlagSet, sample interface{}, opts ...ConfigFieldsOption) error {
	return DefaultConfiger.Register(fs, "", sample, opts...)
}

// Register set config fields to yaml configfile reader and pflags.FlagSet from sample
func Register(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	return DefaultConfiger.Register(fs, path, sample, opts...)
}

type configer struct {
	*ConfigerOptions

	// factory options
	valueFiles   []string       // files, -f/--values
	values       []string       // values, --set
	stringValues []string       // values, --set-string
	fileValues   []string       // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	fields       []*configField // all of config fields
	fs           *pflag.FlagSet

	// instance data
	data    map[string]interface{}
	env     map[string]interface{}
	path    []string
	parsed  bool
	samples []*ConfigFields
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

func (p *configer) FlagSet() *pflag.FlagSet {
	return p.fs
}

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
			bytes, err := ioutil.ReadFile(string(rs))
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

func (p *configer) Flags() (names []string) {
	//if !p.enableFlag {
	//	return
	//}
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

func (p *configer) getEnv(key string) (string, bool) {
	val, ok := os.LookupEnv(key)
	return val, ok && (p.allowEmptyEnv || val != "")
}

func (p *configer) AddFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&p.valueFiles, "values", "f", p.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&p.values, "set", p.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.stringValues, "set-string", p.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.fileValues, "set-file", p.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

func (p *configer) ValueFiles() []string {
	return append(p.valueFiles, p.filesOverride...)
}

// AddRegisteredFlags: add registered flags that from RegisterConfigFields to pflag.FlagSet
func (p *configer) AddRegisteredFlags(fs *pflag.FlagSet) {
	p.fs = fs
	for _, v := range p.samples {
		o := newConfigFieldsOptions(p)
		for _, opt := range v.opts {
			opt(o)
		}
		o.prefixPath = v.path
		if o.tagsGetter != nil {
			o.tags = o.tagsGetter()
		}

		if values, err := objToValues(v.sample); err != nil {
			panic(err)
		} else {
			o.defaultValues = pathValueToValues(v.path, values)
		}

		rv := reflect.Indirect(reflect.ValueOf(v.sample))
		rt := rv.Type()

		if rv.Kind() != reflect.Struct {
			panic(fmt.Errorf("Addflag: sample must be a struct, got %v/%v", rv.Kind(), rt))
		}

		if err := p.addConfigs(parsePath(v.path), v.fs, rt, o); err != nil {
			panic(err)
		}
	}
}

// addConfigs: add flags and env from sample's tags
// defualt priority sample > tagsGetter > tags
func (p *configer) Register(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	if p == nil {
		return errors.New("configer pointer is nil")
	}

	p.samples = append(p.samples, &ConfigFields{
		fs:     fs,
		path:   path,
		sample: sample,
		opts:   opts,
	})

	return nil
}

func (p *configer) addConfigs(path []string, fs *pflag.FlagSet, rt reflect.Type, opt *configFieldsOptions) error {
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

		tag := opt.getTagOpts(sf, path)
		if tag.skip {
			continue
		}

		ft := sf.Type
		if ft.Kind() == reflect.Ptr {
			// Follow pointer.
			ft = ft.Elem()
		}

		curPath := make([]string, len(path))
		copy(curPath, path)

		if len(tag.json) > 0 {
			curPath = append(curPath, tag.json)
		}

		// env
		ps := joinPath(curPath...)

		def := opt.getDefaultValue(ps, tag, p.ConfigerOptions)
		if strings.HasPrefix(ps, "authentication") {
			klog.InfoS("config fields", "ps", ps, "def", def, "flag", tag.Flag)
		}

		var field *configField

		switch sample := reflect.New(ft).Interface().(type) {
		case pflag.Value:
			field = newConfigFieldByValue(fs, ps, tag, sample, def)
		case *net.IP:
			var df net.IP
			if def != "" {
				df = net.ParseIP(def)
			}
			field = newConfigField(fs, ps, tag, fs.IP, fs.IPP, df)
		case *bool:
			field = newConfigField(fs, ps, tag, fs.Bool, fs.BoolP, cast.ToBool(def))
		case *string:
			field = newConfigField(fs, ps, tag, fs.String, fs.StringP, cast.ToString(def))
			if strings.HasPrefix(ps, "authentication") {
				klog.InfoS("config fields", "ps", ps, "def", def, "flag", tag.Flag)
			}
		case *int32, *int16, *int8, *int:
			field = newConfigField(fs, ps, tag, fs.Int, fs.IntP, cast.ToInt(def))
		case *int64:
			field = newConfigField(fs, ps, tag, fs.Int64, fs.Int64P, cast.ToInt64(def))
		case *uint:
			field = newConfigField(fs, ps, tag, fs.Uint, fs.UintP, cast.ToUint(def))
		case *uint8:
			field = newConfigField(fs, ps, tag, fs.Uint8, fs.Uint8P, cast.ToUint8(def))
		case *uint16:
			field = newConfigField(fs, ps, tag, fs.Uint8, fs.Uint8P, cast.ToUint16(def))
		case *uint32:
			field = newConfigField(fs, ps, tag, fs.Uint32, fs.Uint32P, cast.ToUint32(def))
		case *uint64:
			field = newConfigField(fs, ps, tag, fs.Uint64, fs.Uint64P, cast.ToUint64(def))
		case *float32, *float64:
			field = newConfigField(fs, ps, tag, fs.Float64, fs.Float64P, cast.ToFloat64(def))
		case *time.Duration:
			field = newConfigField(fs, ps, tag, fs.Duration, fs.DurationP, cast.ToDuration(def))
		case *[]string:
			field = newConfigField(fs, ps, tag, fs.StringArray, fs.StringArrayP, cast.ToStringSlice(def))
		case *[]int:
			field = newConfigField(fs, ps, tag, fs.IntSlice, fs.IntSliceP, cast.ToIntSlice(def))
		case *[]float64:
			field = newConfigField(fs, ps, tag, fs.Float64Slice, fs.Float64SliceP, ToFloat64Slice(def))
		case *map[string]string:
			field = newConfigField(fs, ps, tag, fs.StringToString, fs.StringToStringP, cast.ToStringMapString(def))
		default:
			if len(tag.Flag) > 0 {
				panic(fmt.Sprintf("add config unsupported type %s path %s kind %s", ft.String(), ps, ft.Kind()))
			}

			// iterate struct{}
			if ft.Kind() == reflect.Struct {
				if err := p.addConfigs(curPath, fs, ft, opt); err != nil {
					return err
				}
				continue
			}

			// set field.default
			field = newConfigField(fs, ps, tag, nil, nil, cast.ToStringMapString(def))
		}
		p.fields = append(p.fields, field)
	}
	return nil
}

func (p *configer) getFlagValue(f *configField) interface{} {
	if f.flag != "" && p.fs.Changed(f.flag) {
		return reflect.ValueOf(f.flagValue).Elem().Interface()
	}

	return nil
}
