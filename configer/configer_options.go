package configer

import (
	"github.com/spf13/pflag"
)

type ConfigerOptions struct {
	filesOverride  []string               //  will append to valueFiles
	defaultValues  map[string]interface{} // WithDefault()
	overrideValues map[string]interface{} // WithOverride()
	enableFlag     bool
	flagSet        *pflag.FlagSet
	maxDepth       int
	enableEnv      bool
	allowEmptyEnv  bool
	err            error
}

func (p *ConfigerOptions) Validate() error {
	if p.err != nil {
		return p.err
	}

	return nil
}

func newConfigerOptions() *ConfigerOptions {
	return &ConfigerOptions{
		enableFlag:     true,
		enableEnv:      true,
		allowEmptyEnv:  false,
		maxDepth:       5,
		defaultValues:  map[string]interface{}{},
		overrideValues: map[string]interface{}{},
	}
}

type ConfigerOption func(*ConfigerOptions)

// with config object
func WithDefault(path string, sample interface{}) ConfigerOption {
	return func(c *ConfigerOptions) {
		c.defaultValues, c.err = mergePathObj(c.defaultValues, path, sample)
	}
}

// with config yaml
func WithDefaultYaml(path, yaml string) ConfigerOption {
	return func(c *ConfigerOptions) {
		c.defaultValues, c.err = mergePathYaml(c.defaultValues, path, []byte(yaml))
	}
}

func WithOverride(path string, sample interface{}) ConfigerOption {
	return func(c *ConfigerOptions) {
		c.overrideValues, c.err = mergePathObj(c.overrideValues, path, sample)
	}
}

func WithOverrideYaml(path, yaml string) ConfigerOption {
	return func(c *ConfigerOptions) {
		c.overrideValues, c.err = mergePathYaml(c.overrideValues, path, []byte(yaml))
	}
}

// WithValueFile priority greater than --values
func WithValueFile(valueFiles ...string) ConfigerOption {
	return func(c *ConfigerOptions) {
		c.filesOverride = append(c.filesOverride, valueFiles...)
	}
}

func WithEnv(allowEnv, allowEmptyEnv bool) ConfigerOption {
	return func(p *ConfigerOptions) {
		p.enableEnv = allowEnv
		p.allowEmptyEnv = allowEmptyEnv
	}
}

func WithMaxDepth(maxDepth int) ConfigerOption {
	return func(p *ConfigerOptions) {
		p.maxDepth = maxDepth
	}
}

func WithFlagSet(fs *pflag.FlagSet) ConfigerOption {
	return func(p *ConfigerOptions) {
		if fs == nil {
			p.enableFlag = false
			return
		}

		p.enableFlag = true
		p.flagSet = fs
	}
}
