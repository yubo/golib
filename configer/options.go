package configer

import (
	"sigs.k8s.io/yaml"
)

type options struct {
	valueFiles []string
	pathsBase  map[string]string // data in yaml format with path
	cb         func(o Options)
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

func WithCallback(cb func(Options)) Option {
	return newFuncOption(func(o *options) {
		o.cb = cb
	})
}

type Options interface {
	AppendValueFile(file string)
}

func (p *options) AppendValueFile(file string) {
	p.valueFiles = append(p.valueFiles, file)
}
