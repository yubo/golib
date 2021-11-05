package configer

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
)

// RegisterConfigFields set config fields to yaml configfile reader and pflags.FlagSet from sample
func RegisterConfigFields(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	return DefaultFactory.RegisterConfigFields(fs, path, sample, opts...)
}

// addConfigs: add flags and env from sample's tags
// defualt priority sample > tagsGetter > tags
func (p *configer) RegisterConfigFields(fs *pflag.FlagSet, path string, sample interface{}, opts ...ConfigFieldsOption) error {
	if p == nil {
		return errors.New("configer pointer is nil")
	}
	o := newConfigFieldsOptions(p)

	for _, opt := range opts {
		opt(o)
	}
	o.prefixPath = path

	if v, err := objToValues(sample); err != nil {
		return err
	} else {
		o.defaultValues = pathValueToValues(path, v)
	}

	rv := reflect.Indirect(reflect.ValueOf(sample))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("Addflag: sample must be a struct, got %v/%v", rv.Kind(), rt)
	}

	return p.addConfigs(parsePath(path), fs, rt, o)
}

func (p *configer) addConfigs(path []string, fs *pflag.FlagSet, rt reflect.Type, opt *configFieldsOptions) error {
	if len(path) > p.maxDepth {
		return fmt.Errorf("path.depth is larger than the maximum allowed depth of %d", p.maxDepth)
	}

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		isUnexported := sf.PkgPath != ""
		if sf.Anonymous {
			t := sf.Type
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			if isUnexported && t.Kind() != reflect.Struct {
				// Ignore embedded fields of unexported non-struct types.
				continue
			}
		} else if isUnexported {
			// Ignore unexported non-embedded fields.
			continue
		}

		tag := opt.getTagOpts(sf, path)
		if tag.skip {
			continue
		}

		ft := sf.Type
		if ft.Kind() == reflect.Ptr {
			// Follow pointer.
			ft = ft.Elem()
		}

		curPath := make([]string, len(path))
		copy(curPath, path)

		if len(tag.json) > 0 {
			curPath = append(curPath, tag.json)
		}

		if len(tag.Flag) == 0 && ft.Kind() == reflect.Struct {
			if err := p.addConfigs(curPath, fs, ft, opt); err != nil {
				return err
			}
			continue
		}

		ps := joinPath(curPath...)
		def := opt.getDefaultValue(ps, tag, p.ConfigerOptions)
		var field *configField

		switch sample := reflect.New(ft).Interface().(type) {
		case pflag.Value:
			field = newConfigFieldByValue(fs, ps, tag, sample, def)
		case *net.IP:
			var df net.IP
			if def != "" {
				df = net.ParseIP(def)
			}
			field = newConfigField(fs, ps, tag, fs.IP, fs.IPP, df)
		case *bool:
			field = newConfigField(fs, ps, tag, fs.Bool, fs.BoolP, cast.ToBool(def))
		case *string:
			field = newConfigField(fs, ps, tag, fs.String, fs.StringP, cast.ToString(def))
		case *int32, *int16, *int8, *int:
			field = newConfigField(fs, ps, tag, fs.Int, fs.IntP, cast.ToInt(def))
		case *int64:
			field = newConfigField(fs, ps, tag, fs.Int64, fs.Int64P, cast.ToInt64(def))
		case *uint:
			field = newConfigField(fs, ps, tag, fs.Uint, fs.UintP, cast.ToUint(def))
		case *uint8:
			field = newConfigField(fs, ps, tag, fs.Uint8, fs.Uint8P, cast.ToUint8(def))
		case *uint16:
			field = newConfigField(fs, ps, tag, fs.Uint8, fs.Uint8P, cast.ToUint16(def))
		case *uint32:
			field = newConfigField(fs, ps, tag, fs.Uint32, fs.Uint32P, cast.ToUint32(def))
		case *uint64:
			field = newConfigField(fs, ps, tag, fs.Uint64, fs.Uint64P, cast.ToUint64(def))
		case *float32, *float64:
			field = newConfigField(fs, ps, tag, fs.Float64, fs.Float64P, cast.ToFloat64(def))
		case *time.Duration:
			field = newConfigField(fs, ps, tag, fs.Duration, fs.DurationP, cast.ToDuration(def))
		case *[]string:
			field = newConfigField(fs, ps, tag, fs.StringArray, fs.StringArrayP, cast.ToStringSlice(def))
		case *[]int:
			field = newConfigField(fs, ps, tag, fs.IntSlice, fs.IntSliceP, cast.ToIntSlice(def))
		case *map[string]string:
			field = newConfigField(fs, ps, tag, fs.StringToString, fs.StringToStringP, cast.ToStringMapString(def))
		default:
			panic(fmt.Sprintf("add config unsupported type %s path %s kind %s", ft.String(), ps, ft.Kind()))
		}
		p.fields = append(p.fields, field)
	}
	return nil
}

type configField struct {
	envName      string      // env name
	flag         string      // flag
	shothand     string      // flag shothand
	configPath   string      // config path
	flagValue    interface{} // flag's value
	defaultValue interface{} // field's default value
}

func (p *configer) getFlagValue(f *configField) interface{} {
	if f.flag == "" {
		return nil
	}

	if p.flagSet.Changed(f.flag) {
		return reflect.ValueOf(f.flagValue).Elem().Interface()
	}

	return nil
}

type defaultSetter interface {
	SetDefault(string) error
}

func newConfigFieldByValue(fs *pflag.FlagSet, path string, tag *FieldTag, value pflag.Value, defValue string) *configField {
	rt := reflect.Indirect(reflect.ValueOf(value)).Type()
	def := reflect.New(rt).Interface().(pflag.Value)

	// set value
	if defValue != "" {
		if d, ok := def.(defaultSetter); ok {
			d.SetDefault(defValue)
			value.(defaultSetter).SetDefault(defValue)
		} else {
			// the changed flag may be affected
			def.Set(defValue)
			value.Set(defValue)
		}
	}

	field := &configField{
		configPath: path,
		envName:    tag.Env,
		flagValue:  value,
	}

	if tag.Default != "" {
		field.defaultValue = def
	}

	switch len(tag.Flag) {
	case 0:
		return field
	// nothing
	case 1:
		field.flag = tag.Flag[0]
		fs.Var(value, tag.Flag[0], tag.Description)
	case 2:
		field.flag = tag.Flag[0]
		field.shothand = tag.Flag[1]
		fs.VarP(value, tag.Flag[0], tag.Flag[1], tag.Description)
	default:
		panic("invalid flag value")
	}

	if len(field.flag) > 0 && len(tag.Deprecated) > 0 {
		fs.MarkDeprecated(field.flag, tag.Deprecated)
		fs.Lookup(field.flag).Hidden = false
	}

	return field
}

func newConfigField(fs *pflag.FlagSet, path string, tag *FieldTag, varFn, varPFn, def interface{}) *configField {
	field := &configField{
		configPath: path,
		envName:    tag.Env,
	}

	if tag.Default != "" {
		field.defaultValue = def
	}

	// add flag
	switch len(tag.Flag) {
	case 0:
		// nothing
		return field
	case 1:
		field.flag = tag.Flag[0]
		ret := reflect.ValueOf(varFn).Call([]reflect.Value{
			reflect.ValueOf(tag.Flag[0]),
			reflect.ValueOf(def),
			reflect.ValueOf(tag.Description),
		})
		field.flagValue = ret[0].Interface()
	case 2:
		field.flag = tag.Flag[0]
		field.shothand = tag.Flag[1]
		ret := reflect.ValueOf(varPFn).Call([]reflect.Value{
			reflect.ValueOf(tag.Flag[0]),
			reflect.ValueOf(tag.Flag[1]),
			reflect.ValueOf(def),
			reflect.ValueOf(tag.Description),
		})
		field.flagValue = ret[0].Interface()
	default:
		panic("invalid flag value")
	}

	if len(field.flag) > 0 && len(tag.Deprecated) > 0 {
		fs.MarkDeprecated(field.flag, tag.Deprecated)
		fs.Lookup(field.flag).Hidden = false
	}

	return field
}

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
				tag.Description += " (Read Only)"
				return def
			}
		}
	}

	// env
	if p.enableEnv && tag.Env != "" {
		if def, ok := p.getEnv(tag.Env); ok {
			if len(def) > 0 {
				tag.Default = def
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
