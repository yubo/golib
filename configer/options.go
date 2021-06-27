package configer

import (
	"path/filepath"

	"github.com/spf13/pflag"
)

type options struct {
	pathsBase     map[string]string // data in yaml format with path
	valueFiles    []string          // files, -f/--values
	values        []string          // values, --set
	stringValues  []string          // values, --set-string
	fileValues    []string          // values from file, --set-file=rsaPubData=/etc/ssh/ssh_host_rsa_key.pub
	enableFlag    bool
	enableEnv     bool
	maxDepth      int
	allowEmptyEnv bool
	fs            *pflag.FlagSet
}

func (o *options) addFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&o.valueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&o.values, "set", o.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&o.stringValues, "set-string", o.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&o.fileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

func (p *options) Validate() (err error) {
	for i, file := range p.valueFiles {
		if p.valueFiles[i], err = filepath.Abs(file); err != nil {
			return
		}
	}

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

// key1=val1,key2=val2
func WithValues(values ...string) Option {
	return newFuncOption(func(o *options) {
		o.values = append(o.values, values...)

	})
}

// key1=val1,key2=val2
func WithStringValues(values ...string) Option {
	return newFuncOption(func(o *options) {
		o.stringValues = append(o.stringValues, values...)

	})
}

// key1=path1,key2=path2
func WithFileValues(values ...string) Option {
	return newFuncOption(func(o *options) {
		o.fileValues = append(o.fileValues, values...)
	})
}

func WithFlag(fs *pflag.FlagSet, allowEnv, allowEmptyEnv bool, maxDepth int) Option {
	return newFuncOption(func(o *options) {
		if maxDepth == 0 {
			maxDepth = 5
		}
		o.enableFlag = true
		o.enableEnv = allowEnv
		o.maxDepth = maxDepth
		o.allowEmptyEnv = allowEmptyEnv
		o.fs = fs
	})
}
