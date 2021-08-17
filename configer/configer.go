// the configer is not thread safe,
// make sure not use it after call process.Start()
package configer

// def < env < config < valueFile < value < flag

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/pflag"
	cliflag "github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/util/strvals"
	"github.com/yubo/golib/util/template"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

var (
	Setting = newSetting()
)

func newSetting() *setting {
	return &setting{
		enableFlag: true,
		enableEnv:  true,
		maxDepth:   5,
	}
}

type setting struct {
	valueFiles    []string // files, -f/--values
	values        []string // values, --set
	stringValues  []string // values, --set-string
	fileValues    []string // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	namedFlagSets cliflag.NamedFlagSets

	enableFlag    bool
	enableEnv     bool
	maxDepth      int
	allowEmptyEnv bool
	flagSet       *pflag.FlagSet

	//flags  []*param              // add to flags
	params []*param // all of config fields
}

func SetOptions(allowEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	Setting.SetOptions(allowEnv, allowEmptyEnv, maxDepth, fs)
}

func (s *setting) SetOptions(enableEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	s.enableEnv = enableEnv
	s.maxDepth = maxDepth
	s.allowEmptyEnv = allowEmptyEnv

	if fs != nil {
		s.enableFlag = true
		s.flagSet = fs
	}
}

func (s *setting) AddFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&s.valueFiles, "values", "f", s.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&s.values, "set", s.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.stringValues, "set-string", s.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.fileValues, "set-file", s.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

type Configer struct {
	*setting
	*options

	data     map[string]interface{}
	path     []string
	prepared bool
}

func New(optsIn ...Option) (*Configer, error) {
	opts := &options{}
	for _, opt := range optsIn {
		opt.apply(opts)
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	conf := &Configer{
		data:    map[string]interface{}{},
		options: opts,
		setting: Setting,
	}

	if err := conf.Prepare(); err != nil {
		return nil, err
	}

	return conf, nil
}

func (p *Configer) PrintFlags() {
	printf := klog.V(1).Infof
	for _, value := range append(p.options.valueFiles, p.setting.valueFiles...) {
		printf("FLAG: --values=%s\n", value)
	}
	for _, value := range p.setting.values {
		printf("FLAG: --set=%s\n", value)
	}
	for _, value := range p.setting.stringValues {
		printf("FLAG: --set-string=%s\n", value)
	}
	for _, value := range p.setting.fileValues {
		printf("FLAG: --set-file=%s\n", value)
	}
}

func (p *Configer) Prepare() (err error) {
	if p.prepared {
		return nil
	}

	base := map[string]interface{}{}

	// cb
	if p.cb != nil {
		p.cb(p.options)
	}

	// init base from flag default
	p.mergeDefaultValues(base)

	// base with path
	for path, b := range p.pathsBase {
		if base, err = yaml2ValuesWithPath(base, path, []byte(b)); err != nil {
			return err
		}
	}

	// configFile & valueFile --values
	for _, filePath := range append(p.options.valueFiles, p.setting.valueFiles...) {
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
	for _, value := range p.setting.values {
		if err := strvals.ParseInto(value, base); err != nil {
			return fmt.Errorf("failed parsing --set data: %s", err)
		}
	}

	// User specified a value via --set-string
	for _, value := range p.setting.stringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return fmt.Errorf("failed parsing --set-string data: %s", err)
		}
	}

	// User specified a value via --set-file
	for _, value := range p.setting.fileValues {
		reader := func(rs []rune) (interface{}, error) {
			bytes, err := ioutil.ReadFile(string(rs))
			return string(bytes), err
		}
		if err := strvals.ParseIntoFile(value, base, reader); err != nil {
			return fmt.Errorf("failed parsing --set-file data: %s", err)
		}
	}

	// override
	p.mergeEnvValues(base)
	p.mergeFlagValues(base)

	p.data = base
	p.prepared = true
	return nil
}

func (p *Configer) ValueFiles() []string {
	return p.setting.valueFiles
}

func (p *Configer) GetConfiger(path string) *Configer {
	if data, ok := p.GetRaw(path).(map[string]interface{}); ok {
		return &Configer{
			options: p.options,
			path:    append(clonePath(p.path), parsePath(path)...),
			data:    data,
		}
	}

	return &Configer{
		options: p.options,
		path:    append(clonePath(p.path), parsePath(path)...),
		data:    map[string]interface{}{},
	}
}

func (p *Configer) Set(path string, v interface{}) error {
	if path == "" {
		b, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(b, p.data)
	}

	ps := strings.Split(path, ".")
	src := map[string]interface{}{ps[len(ps)-1]: v}

	for i := len(ps) - 2; i >= 0; i-- {
		src = map[string]interface{}{ps[i]: src}
	}

	p.data = mergeValues(p.data, src)

	return nil
}

func (p *Configer) GetRaw(path string) interface{} {
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

func (p *Configer) GetString(path string) string {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func (p *Configer) GetBool(path string) (bool, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return false, err
	}

	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%v is not bool", path)
	}
	return b, nil
}

func (p *Configer) GetBoolDef(path string, def bool) bool {
	v, err := p.GetBool(path)
	if err != nil {
		return def
	}
	return v
}

func (p *Configer) GetFloat64(path string) (float64, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return 0, err
	}

	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("%v is not number", path)
	}

	return f, nil
}

func (p *Configer) GetFloat64Def(path string, def float64) float64 {
	v, err := p.GetFloat64(path)
	if err != nil {
		return def
	}

	return v
}

func (p *Configer) GetInt64(path string) (int64, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return int64(v), nil
}

func (p *Configer) GetInt64Def(path string, def int64) int64 {
	v, err := p.GetInt64(path)
	if err != nil {
		return def
	}
	return v
}

func (p *Configer) GetInt(path string) (int, error) {
	v, err := p.GetFloat64(path)
	if err != nil {
		return 0, err
	}

	return int(v), nil
}

func (p *Configer) GetIntDef(path string, def int) int {
	v, err := p.GetInt(path)
	if err != nil {
		return def
	}
	return v
}

type validator interface {
	Validate() error
}

func (p *Configer) IsSet(path string) bool {
	_, err := Values(p.data).PathValue(path)
	return err != nil
}

func (p *Configer) Read(path string, into interface{}, optsIn ...Option) error {
	if into == nil {
		return nil
	}

	opts := &options{}
	for _, opt := range optsIn {
		opt.apply(opts)
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

func (p *Configer) String() string {
	buf, err := yaml.Marshal(p.data)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

func (p *Configer) getEnv(key string) (string, bool) {
	val, ok := os.LookupEnv(key)
	return val, ok && (p.allowEmptyEnv || val != "")
}

// merge path.bytes -> into
func yaml2ValuesWithPath(into map[string]interface{}, path string, data []byte) (map[string]interface{}, error) {
	currentMap := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &currentMap); err != nil {
		return into, err
	}

	if len(path) > 0 {
		ps := strings.Split(path, ".")
		for i := len(ps) - 1; i >= 0; i-- {
			currentMap = map[string]interface{}{ps[i]: currentMap}
		}
	}

	into = mergeValues(into, currentMap)
	return into, nil
}

// Merges source and into map, preferring values from the source map ( src > into)
func mergeValues(into map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, ok := into[k]; !ok {
			into[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			into[k] = v
			continue
		}
		intoMap, isMap := into[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			into[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		into[k] = mergeValues(intoMap, nextMap)
	}
	return into
}

func clonePath(path []string) []string {
	ret := make([]string, len(path))
	copy(ret, path)
	return ret
}
