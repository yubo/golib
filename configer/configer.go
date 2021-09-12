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
	"github.com/yubo/golib/util/strvals"
	"github.com/yubo/golib/util/template"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

var (
	GlobalOptions = newOptions()
)

func SetOptions(allowEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	GlobalOptions.SetOptions(allowEnv, allowEmptyEnv, maxDepth, fs)
}

type Configer struct {
	*Options

	data     map[string]interface{}
	path     []string
	prepared bool
}

// must called after pflag parse
func New(opts ...Option) (*Configer, error) {
	options := GlobalOptions.DeepCopy()
	for _, opt := range opts {
		opt(options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	conf := &Configer{
		data:    map[string]interface{}{},
		Options: options,
	}

	if err := conf.Prepare(); err != nil {
		return nil, err
	}

	return conf, nil
}

func (p *Configer) PrintFlags() {
	printf := klog.V(1).Infof
	for _, value := range p.valueFiles {
		printf("FLAG: --values=%s\n", value)
	}
	for _, value := range p.values {
		printf("FLAG: --set=%s\n", value)
	}
	for _, value := range p.stringValues {
		printf("FLAG: --set-string=%s\n", value)
	}
	for _, value := range p.fileValues {
		printf("FLAG: --set-file=%s\n", value)
	}
}

func (p *Configer) Prepare() (err error) {
	if p.prepared {
		return nil
	}

	base := map[string]interface{}{}

	// init base from flag default
	p.mergeDefaultValues(base)

	// base with path
	for path, b := range p.pathsBase {
		if base, err = yaml2ValuesWithPath(base, path, []byte(b)); err != nil {
			return err
		}
	}

	// configFile & valueFile --values
	for _, filePath := range p.valueFiles {
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

	for path, b := range p.pathsOverride {
		if base, err = yaml2ValuesWithPath(base, path, []byte(b)); err != nil {
			return err
		}
	}

	p.data = base
	p.prepared = true
	return nil
}

func (p *Configer) ValueFiles() []string {
	if p == nil || p.Options == nil {
		return nil
	}
	return p.valueFiles
}

func (p *Configer) GetConfiger(path string) *Configer {
	if data, ok := p.GetRaw(path).(map[string]interface{}); ok {
		return &Configer{
			Options: p.Options,
			path:    append(clonePath(p.path), parsePath(path)...),
			data:    data,
		}
	}

	return &Configer{
		Options: p.Options,
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
	return err == nil
}

func (p *Configer) Read(path string, into interface{}) error {
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

func (p *Configer) String() string {
	buf, err := yaml.Marshal(p.data)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

func (p *Options) getEnv(key string) (string, bool) {
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
