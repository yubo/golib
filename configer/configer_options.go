package configer

import (
	"reflect"
	"strings"

	"github.com/spf13/cast"
	"k8s.io/klog/v2"
)

type ConfigerOptions struct {
	filesOverride  []string               //  will append to valueFiles
	defaultValues  map[string]interface{} // WithDefault()
	overrideValues map[string]interface{} // WithOverride()
	maxDepth       int
	enableEnv      bool
	allowEmptyEnv  bool
	err            error
	//enableFlag     bool
	//flagSet       *pflag.FlagSet
}

func (p *ConfigerOptions) Validate() error {
	if p.err != nil {
		return p.err
	}

	return nil
}

func newConfigerOptions() *ConfigerOptions {
	return &ConfigerOptions{
		//enableFlag:     true,
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

// ############# config fields

func newConfigFieldsOptions(c *configer) *configFieldsOptions {
	return &configFieldsOptions{configer: c}
}

// for addConfigs
type configFieldsOptions struct {
	*configer
	tagsGetter    func() map[string]*FieldTag
	prefixPath    string
	defaultValues map[string]interface{}
	tags          map[string]*FieldTag
}

func (p *configFieldsOptions) getTagOpts(sf reflect.StructField, paths []string) *FieldTag {
	tag := GetFieldTag(sf)

	if p == nil || p.tags == nil {
		return tag
	}

	path := strings.TrimPrefix(joinPath(append(paths, tag.json)...), p.prefixPath+".")
	if o := p.tags[path]; o != nil {
		if len(o.Flag) > 0 {
			tag.Flag = o.Flag
		}
		if len(o.Description) > 0 {
			tag.Description = o.Description
		}
		if len(o.Default) > 0 {
			tag.Default = o.Default
		}
		if len(o.Env) > 0 {
			tag.Env = o.Env
		}
	}

	return tag
}

// env > value from registered config > structField tag
func (p *configFieldsOptions) getDefaultValue(path string, tag *FieldTag, opts *ConfigerOptions) string {
	// overrideValues
	if v, err := Values(opts.overrideValues).PathValue(path); err == nil {
		if def := cast.ToString(v); len(def) > 0 {
			tag.Default = def
			tag.Description += " (override)"
			return def
		}
	}

	// env
	if p.enableEnv && tag.Env != "" {
		if def, ok := p.getEnv(tag.Env); ok {
			if len(def) > 0 {
				tag.Default = def
				p.env = mergePathValue(p.env, path, def)
				return def
			}
		}
	}

	klog.V(10).InfoS("default values", "values", Values(p.defaultValues).String())
	// defaultValues
	if v, _ := Values(p.defaultValues).PathValue(path); v != nil {
		if def := ToString(v); len(def) > 0 {
			tag.Default = def
			return def
		}
	}

	return tag.Default
}

type ConfigFieldsOption func(*configFieldsOptions)

// WithTags just for AddConfigs
func WithTags(getter func() map[string]*FieldTag) ConfigFieldsOption {
	return func(o *configFieldsOptions) {
		o.tagsGetter = getter
	}
}
