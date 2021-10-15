package configer

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

type options struct {
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
	defualtValues map[string]interface{} // for AddConfigs()
}

func newOptions() *options {
	return &options{
		enableFlag:    true,
		enableEnv:     true,
		allowEmptyEnv: false,
		maxDepth:      5,
	}
}

func (s *options) set(enableEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	s.enableEnv = enableEnv
	s.maxDepth = maxDepth
	s.allowEmptyEnv = allowEmptyEnv

	if fs != nil {
		s.enableFlag = true
		s.flagSet = fs
	}
}

func (s *options) addFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&s.valueFiles, "values", "f", s.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&s.values, "set", s.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.stringValues, "set-string", s.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.fileValues, "set-file", s.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

func (in *options) deepCopy() (out *options) {
	if in == nil {
		return nil
	}

	out = new(options)
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

func (p *options) validate() (err error) {
	return nil
}

type Option func(*options)

// with config object
func WithConfig(path string, config interface{}) Option {
	b, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}

	return WithDefaultYaml(path, string(b))
}

// with config yaml
func WithDefaultYaml(path, yamlData string) Option {
	return func(o *options) {
		if o.pathsBase == nil {
			o.pathsBase = map[string]string{path: yamlData}
		} else {
			o.pathsBase[path] = yamlData
		}
	}
}

func WithOverrideYaml(path, yamlData string) Option {
	return func(o *options) {
		if o.pathsOverride == nil {
			o.pathsOverride = map[string]string{path: yamlData}
		} else {
			o.pathsOverride[path] = yamlData
		}
	}
}

func WithValueFile(valueFiles ...string) Option {
	return func(o *options) {
		o.valueFiles = append(o.valueFiles, valueFiles...)
	}
}

func WithTags(tags map[string]*TagOpts) Option {
	return func(o *options) {
		o.tags = tags
	}
}
