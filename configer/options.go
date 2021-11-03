package configer

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

// default value priority: env > sample > comstom tags > fieldstruct tags

type Options struct {
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
	defaultValues map[string]interface{} // from sample
}

func NewOptions() *Options {
	return &Options{
		enableFlag:    true,
		enableEnv:     true,
		allowEmptyEnv: false,
		maxDepth:      5,
	}
}

func (s *Options) set(enableEnv, allowEmptyEnv bool, maxDepth int, fs *pflag.FlagSet) {
	s.enableEnv = enableEnv
	s.maxDepth = maxDepth
	s.allowEmptyEnv = allowEmptyEnv

	if fs != nil {
		s.enableFlag = true
		s.flagSet = fs
	}
}

func (s *Options) addFlags(f *pflag.FlagSet) {
	f.StringSliceVarP(&s.valueFiles, "values", "f", s.valueFiles, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&s.values, "set", s.values, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.stringValues, "set-string", s.stringValues, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.fileValues, "set-file", s.fileValues, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
}

func (in *Options) deepCopy() (out *Options) {
	if in == nil {
		return nil
	}

	out = new(Options)
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

func (p *Options) validate() (err error) {
	return nil
}

func (p *Options) addConfigs(path []string, fs *pflag.FlagSet, rt reflect.Type) error {
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

		opt := p.getTagOpts(sf, path)
		if opt.skip {
			continue
		}

		ft := sf.Type
		if ft.Kind() == reflect.Ptr {
			// Follow pointer.
			ft = ft.Elem()
		}

		curPath := make([]string, len(path))
		copy(curPath, path)

		if len(opt.json) > 0 {
			curPath = append(curPath, opt.json)
		}

		if len(opt.Flag) == 0 && ft.Kind() == reflect.Struct {
			if err := p.addConfigs(curPath, fs, ft); err != nil {
				return err
			}
			continue
		}

		ps := joinPath(curPath...)
		def := getDefaultValue(ps, opt, p)

		switch sample := reflect.New(ft).Interface().(type) {
		case pflag.Value:
			addConfigFieldByValue(fs, ps, opt, sample, def)
		case *net.IP:
			var df net.IP
			if def != "" {
				df = net.ParseIP(def)
			}
			addConfigField(fs, ps, opt, fs.IP, fs.IPP, df)
		case *bool:
			addConfigField(fs, ps, opt, fs.Bool, fs.BoolP, cast.ToBool(def))
		case *string:
			addConfigField(fs, ps, opt, fs.String, fs.StringP, cast.ToString(def))
		case *int32, *int16, *int8, *int:
			addConfigField(fs, ps, opt, fs.Int, fs.IntP, cast.ToInt(def))
		case *int64:
			addConfigField(fs, ps, opt, fs.Int64, fs.Int64P, cast.ToInt64(def))
		case *uint:
			addConfigField(fs, ps, opt, fs.Uint, fs.UintP, cast.ToUint(def))
		case *uint8:
			addConfigField(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint8(def))
		case *uint16:
			addConfigField(fs, ps, opt, fs.Uint8, fs.Uint8P, cast.ToUint16(def))
		case *uint32:
			addConfigField(fs, ps, opt, fs.Uint32, fs.Uint32P, cast.ToUint32(def))
		case *uint64:
			addConfigField(fs, ps, opt, fs.Uint64, fs.Uint64P, cast.ToUint64(def))
		case *float32, *float64:
			addConfigField(fs, ps, opt, fs.Float64, fs.Float64P, cast.ToFloat64(def))
		case *time.Duration:
			addConfigField(fs, ps, opt, fs.Duration, fs.DurationP, cast.ToDuration(def))
		case *[]string:
			addConfigField(fs, ps, opt, fs.StringArray, fs.StringArrayP, cast.ToStringSlice(def))
		case *[]int:
			addConfigField(fs, ps, opt, fs.IntSlice, fs.IntSliceP, cast.ToIntSlice(def))
		case *map[string]string:
			addConfigField(fs, ps, opt, fs.StringToString, fs.StringToStringP, cast.ToStringMapString(def))
		default:
			klog.InfoS("add config unsupported", "type", ft.String(), "path", ps, "kind", ft.Kind())
		}
	}
	return nil
}

func (p *Options) getTagOpts(sf reflect.StructField, paths []string) *TagOpts {
	opts := getTagOpts(sf, p)

	if p.tags != nil {
		path := strings.TrimPrefix(joinPath(append(paths, opts.json)...), p.prefixPath+".")
		if o := p.tags[path]; o != nil {
			if len(o.Flag) > 0 {
				opts.Flag = o.Flag
			}
			if len(o.Description) > 0 {
				opts.Description = o.Description
			}
			if len(o.Default) > 0 {
				opts.Default = o.Default
			}
			if len(o.Env) > 0 {
				opts.Env = o.Env
			}
		}
	}

	return opts
}

type Option func(*Options)

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
	return func(o *Options) {
		if o.pathsBase == nil {
			o.pathsBase = map[string]string{path: yamlData}
		} else {
			o.pathsBase[path] = yamlData
		}
	}
}

func WithOverrideYaml(path, yamlData string) Option {
	return func(o *Options) {
		if o.pathsOverride == nil {
			o.pathsOverride = map[string]string{path: yamlData}
		} else {
			o.pathsOverride[path] = yamlData
		}
	}
}

func WithValueFile(valueFiles ...string) Option {
	return func(o *Options) {
		o.valueFiles = append(o.valueFiles, valueFiles...)
	}
}

func WithTags(tags map[string]*TagOpts) Option {
	return func(o *Options) {
		o.tags = tags
	}
}
