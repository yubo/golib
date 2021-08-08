package configer

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

type options struct {
	valueFiles    []string
	pathsBase     map[string]string // data in yaml format with path
	enableFlag    bool
	enableEnv     bool
	maxDepth      int
	allowEmptyEnv bool
	cb            func(o Options)
	fs            *pflag.FlagSet
}

func (p *options) Validate() (err error) {
	return nil
}

type Option interface {
	apply(*options)
}

type funcOption struct {
	f func(*options)
}

func (p *funcOption) apply(opt *options) {
	p.f(opt)
}

func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

func WithConfig(path string, config interface{}) Option {
	b, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}

	return WithDefaultYaml(path, string(b))
}

func WithDefaultYaml(path, yamlData string) Option {
	return newFuncOption(func(o *options) {
		if o.pathsBase == nil {
			o.pathsBase = map[string]string{path: yamlData}
		} else {
			o.pathsBase[path] = yamlData
		}
	})
}

func WithValueFile(valueFiles ...string) Option {
	return newFuncOption(func(o *options) {
		o.valueFiles = append(o.valueFiles, valueFiles...)
	})
}

func WithFlagOptions(allowEnv, allowEmptyEnv bool, maxDepth int) Option {
	return newFuncOption(func(o *options) {
		o.enableEnv = allowEnv
		o.maxDepth = maxDepth
		o.allowEmptyEnv = allowEmptyEnv
	})
}

func WithCallback(cb func(Options)) Option {
	return newFuncOption(func(o *options) {
		o.cb = cb
	})
}

func WithFlag(fs *pflag.FlagSet) Option {
	return newFuncOption(func(o *options) {
		if fs != nil {
			o.enableFlag = true
		}
		if o.maxDepth == 0 {
			o.maxDepth = 5
		}
		o.fs = fs
	})
}

type Options interface {
	AppendValueFile(file string)
}

func (p *options) AppendValueFile(file string) {
	p.valueFiles = append(p.valueFiles, file)
}
