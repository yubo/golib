package configer

import (
	"github.com/spf13/pflag"
)

type options struct {
	valueFiles    []string
	pathsBase     map[string]string // data in yaml format with path
	enableFlag    bool
	enableEnv     bool
	maxDepth      int
	allowEmptyEnv bool
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

func WithFlag(fs *pflag.FlagSet, allowEnv, allowEmptyEnv bool, maxDepth int) Option {
	return newFuncOption(func(o *options) {
		if maxDepth == 0 {
			maxDepth = 5
		}
		if fs != nil {
			o.enableFlag = true
		}
		o.enableEnv = allowEnv
		o.maxDepth = maxDepth
		o.allowEmptyEnv = allowEmptyEnv
		o.fs = fs
	})
}
