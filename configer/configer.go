// the configer is not thread safe,
// make sure not use it after call process.Start()
package configer

// def < env < config < valueFile < value < flag

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"github.com/yubo/golib/util/strvals"
	"github.com/yubo/golib/util/template"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type Factory interface {
	RegisterConfigFields(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error
	NewConfiger(opts ...ConfigerOption) (Configer, error)
	SetOptions(allowEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet)
	AddFlags(f *pflag.FlagSet)
	ValueFiles() []string
	Envs() []string
	Flags() []string
}

type Configer interface {
	ValueFiles() []string
	Envs() []string
	Flags() []string

	Set(path string, v interface{}) error
	GetConfiger(path string) Configer
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
	DefaultFactory = NewFactory()
)

func NewFactory() Factory {
	return newConfiger()
}
func NewConfiger(opts ...ConfigerOption) (Configer, error) {
	return DefaultFactory.NewConfiger(opts...)
}
func SetOptions(allowEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	DefaultFactory.SetOptions(allowEnv, allowEmptyEnv, maxDepth, fs)
}
func AddFlags(f *pflag.FlagSet) {
	DefaultFactory.AddFlags(f)
}
func ValueFiles() []string {
	return DefaultFactory.ValueFiles()
}
func Envs() []string {
	return DefaultFactory.Envs()
}
func Flags() []string {
	return DefaultFactory.Flags()
}

type configer struct {
	// factory options
	valueFiles    []string // files, -f/--values
	values        []string // values, --set
	stringValues  []string // values, --set-string
	fileValues    []string // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	enableFlag    bool
	maxDepth      int
	enableEnv     bool
	allowEmptyEnv bool
	flagSet       *pflag.FlagSet
	fields        []*configField // all of config fields

	ConfigerOptions

	// data options
	data   map[string]interface{}
	path   []string
	parsed bool
}

func newConfiger() *configer {
	return &configer{
		enableFlag:    true,
		enableEnv:     true,
		allowEmptyEnv: false,
		maxDepth:      5,
		data:          map[string]interface{}{},
		path:          []string{},
	}
}

func (p *configer) NewConfiger(opts ...ConfigerOption) (Configer, error) {
	for _, opt := range opts {
		opt(&p.ConfigerOptions)
	}

	if err := p.parse(); err != nil {
		return nil, err
	}

	return p, nil
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

	// init base from flag default
	p.mergeDefaultValues(base)

	// base with path
	for path, b := range p.ConfigerOptions.PathsBase {
		if base, err = yaml2ValuesWithPath(base, path, []byte(b)); err != nil {
			return err
		}
	}

	// configFile & valueFile --values
	for _, filePath := range append(p.valueFiles, p.ConfigerOptions.FilesOverride...) {
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

	// override
	//p.mergeEnvValues(base)
	p.mergeFlagValues(base)

	for path, b := range p.ConfigerOptions.PathsOverride {
		if base, err = yaml2ValuesWithPath(base, path, []byte(b)); err != nil {
			return err
		}
	}

	p.data = base
	p.parsed = true
	return nil
}
func (p *configer) mergeDefaultValues(into map[string]interface{}) {
	for _, f := range p.fields {
		if v := f.defaultValue; v != nil {
			//klog.V(7).InfoS("def", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToTable(joinPath(append(p.path, f.configPath)...), v))
		}
	}
}

// merge flag value into ${into}
func (p *configer) mergeFlagValues(into map[string]interface{}) {
	if !p.enableFlag {
		return
	}
	for _, f := range p.fields {
		if v := p.getFlagValue(f); v != nil {
			//klog.V(7).InfoS("flag", "path", joinPath(append(p.path, f.configPath)...), "value", v)
			mergeValues(into, pathValueToTable(joinPath(append(p.path, f.configPath)...), v))
		}
	}
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
	if !p.enableFlag {
		return
	}
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

func (p *configer) GetConfiger(path string) Configer {
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

// default value priority: env > sample > comstom tags > fieldstruct tags
func (p *configer) SetOptions(allowEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	p.enableEnv = allowEnv
	p.maxDepth = maxDepth
	p.allowEmptyEnv = allowEmptyEnv

	if fs != nil {
		p.enableFlag = true
		p.flagSet = fs
	}

}

func (p *configer) AddFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&p.valueFiles, "values", "f", p.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&p.values, "set", p.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.stringValues, "set-string", p.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.fileValues, "set-file", p.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")

}

func (p *configer) ValueFiles() []string {
	return append(p.valueFiles, p.FilesOverride...)
}

type ConfigerOptions struct {
	FilesOverride []string // same as valueFiles
	PathsBase     map[string]string
	PathsOverride map[string]string
}

type ConfigerOption func(*ConfigerOptions)

// with config object
func WithConfig(path string, config interface{}) ConfigerOption {
	b, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}

	return WithDefaultYaml(path, string(b))
}

// with config yaml
func WithDefaultYaml(path, yamlData string) ConfigerOption {
	return func(c *ConfigerOptions) {
		if c.PathsBase == nil {
			c.PathsBase = map[string]string{path: yamlData}
		} else {
			c.PathsBase[path] = yamlData
		}
	}
}

func WithOverrideYaml(path, yamlData string) ConfigerOption {
	return func(c *ConfigerOptions) {
		if c.PathsOverride == nil {
			c.PathsOverride = map[string]string{path: yamlData}
		} else {
			c.PathsOverride[path] = yamlData
		}
	}
}

// WithValueFile priority greater than --values
func WithValueFile(valueFiles ...string) ConfigerOption {
	return func(c *ConfigerOptions) {
		c.FilesOverride = append(c.FilesOverride, valueFiles...)
	}
}
