package configer

import (
	"errors"
	"path/filepath"

	"github.com/yubo/golib/template"
)

type options struct {
	baseFile     string
	base         []byte
	valueFiles   []string // files, -f/--values
	values       []string // values, --set
	stringValues []string // values, --set-string
	fileValues   []string // values from file, --set-file
}

func (p *options) Validate() (err error) {
	if len(p.base) > 0 && p.baseFile != "" {
		return errors.New("config base & baseFile can't be set at the same time")
	}
	if p.baseFile != "" {
		if p.baseFile, err = filepath.Abs(p.baseFile); err != nil {
			return
		}
	}

	if p.baseFile != "" {
		if p.base, err = template.ReadFileWithInclude(p.baseFile); err != nil {
			return
		}

	}

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

func WithBaseFile(baseFile string) Option {
	return newFuncOption(func(o *options) {
		o.baseFile = baseFile
	})
}

func WithBaseBytes(base []byte) Option {
	return newFuncOption(func(o *options) {
		o.base = base
	})
}

func WithValueFiles(valueFiles ...string) Option {
	return newFuncOption(func(o *options) {
		o.valueFiles = append(o.valueFiles, valueFiles...)

	})
}

func WithValueFile(valueFile string) Option {
	return newFuncOption(func(o *options) {
		o.valueFiles = append(o.valueFiles, valueFile)

	})
}

func WithValues(values ...string) Option {
	return newFuncOption(func(o *options) {
		o.values = append(o.values, values...)

	})
}

func WithStringValues(values ...string) Option {
	return newFuncOption(func(o *options) {
		o.stringValues = append(o.stringValues, values...)

	})
}

func WithFileValues(values ...string) Option {
	return newFuncOption(func(o *options) {
		o.fileValues = append(o.fileValues, values...)

	})
}
