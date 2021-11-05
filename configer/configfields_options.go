package configer

import (
	"reflect"
	"strings"

	"github.com/spf13/cast"
)

func newConfigFieldsOptions(c *configer) *configFieldsOptions {
	return &configFieldsOptions{configer: c}
}

// for addConfigs
type configFieldsOptions struct {
	*configer
	tags          map[string]*FieldTag
	prefixPath    string
	defaultValues map[string]interface{}
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
		if !isZero(v) {
			if def := cast.ToString(v); len(def) > 0 {
				tag.Default = def
				tag.Description += " (override)"
				return def
			}
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

	// defaultValues
	if v, err := Values(opts.defaultValues).PathValue(path); err == nil {
		if !isZero(v) {
			if def := cast.ToString(v); len(def) > 0 {
				tag.Default = def
				return def
			}
		}
	}

	if v, err := Values(p.defaultValues).PathValue(path); err == nil {
		if !isZero(v) {
			if def := cast.ToString(v); len(def) > 0 {
				tag.Default = def
				return def
			}
		}
	}

	return tag.Default
}

type ConfigFieldsOption func(*configFieldsOptions)

// WithTags just for AddConfigs
func WithTags(tags map[string]*FieldTag) ConfigFieldsOption {
	return func(o *configFieldsOptions) {
		o.tags = tags
	}
}
