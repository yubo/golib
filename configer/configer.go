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

var (
	DefaultConfiger = newConfiger()
)

// for testing
func Reset() {
	DefaultConfiger = newConfiger()
}

type Configer struct {
	data     map[string]interface{}
	path     []string
	prepared bool

	// options
	pathsBase     map[string]string // data in yaml format with path
	pathsOverride map[string]string // data in yaml format with path
	valueFiles    []string          // files, -f/--values
	values        []string          // values, --set
	stringValues  []string          // values, --set-string
	fileValues    []string          // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	enableFlag    bool
	enableEnv     bool
	maxDepth      int
	allowEmptyEnv bool
	flagSet       *pflag.FlagSet
	params        []*param // all of config fields
	tags          map[string]*TagOpts
	prefixPath    string
	defaultValues map[string]interface{} // from sample
}

func newConfiger() *Configer {
	return &Configer{
		enableFlag:    true,
		enableEnv:     true,
		allowEmptyEnv: false,
		maxDepth:      5,
	}
}

func NewConfiger(opts ...Option) (*Configer, error) {
	return DefaultConfiger.NewConfiger(opts...)
}

func (p *Configer) NewConfiger(opts ...Option) (*Configer, error) {
	cfg := DefaultConfiger.clone()

	if len(opts) == 0 && p.prepared {
		return cfg, nil
	}

	for _, opt := range opts {
		opt(cfg)
	}
	cfg.data = map[string]interface{}{}
	cfg.path = []string{}
	cfg.prepared = false

	if err := cfg.Prepare(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// clone copy all field except data, path, params
func (in *Configer) clone() (out *Configer) {
	if in == nil {
		return nil
	}

	out = new(Configer)
	*out = *in

	if in.pathsBase != nil {
		in, out := &in.pathsBase, &out.pathsBase
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}

	if in.valueFiles != nil {
		in, out := &in.valueFiles, &out.valueFiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}

	if in.values != nil {
		in, out := &in.values, &out.values
		*out = make([]string, len(*in))
		copy(*out, *in)
	}

	if in.fileValues != nil {
		in, out := &in.fileValues, &out.fileValues
		*out = make([]string, len(*in))
		copy(*out, *in)
	}

	// skip in.params
	return
}

func (p *Configer) PrintFlags(out io.Writer) {
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
	if p == nil {
		return nil
	}
	return p.valueFiles
}

func (p *Configer) GetConfiger(path string) *Configer {
	if p == nil || !p.prepared {
		return nil
	}

	data, ok := p.GetRaw(path).(map[string]interface{})
	if !ok {
		data = map[string]interface{}{}
	}

	cfg := p.clone()
	cfg.path = append(clonePath(p.path), parsePath(path)...)
	cfg.data = data
	return cfg
}

func (p *Configer) Set(path string, v interface{}) error {
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

	return cast.ToString(v)
}

func (p *Configer) GetBool(path string) (bool, error) {
	v, err := Values(p.data).PathValue(path)
	if err != nil {
		return false, err
	}

	return cast.ToBool(v), nil
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

	return cast.ToFloat64(v), nil
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

	return cast.ToInt64(v), nil
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

	return cast.ToInt(v), nil
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

func (p *Configer) getEnv(key string) (string, bool) {
	val, ok := os.LookupEnv(key)
	return val, ok && (p.allowEmptyEnv || val != "")
}
