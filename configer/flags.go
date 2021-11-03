package configer

import (
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func (p *Options) addFlagsVar(fs *pflag.FlagSet, in interface{}, depth int) {
	if depth > DefaultOptions.maxDepth {
		panic(fmt.Sprintf("path.depth is larger than the maximum allowed depth of %d", DefaultOptions.maxDepth))
	}

	if k := reflect.ValueOf(in).Kind(); k != reflect.Ptr {
		panic(fmt.Sprintf("sample must be a ptr, got %s", k))
	}

	rv := reflect.ValueOf(in).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		fv := rv.Field(i)
		ft := sf.Type

		opt := p.getTagOpts(sf, nil)
		if len(opt.Flag) == 0 {
			continue
		}

		if ft.Kind() == reflect.Struct {
			prepareValue(fv, ft)
			if fv.Kind() == reflect.Ptr {
				fv = fv.Elem()
				ft = fv.Type()
			}
			p.addFlagsVar(fs, fv.Addr().Interface(), depth+1)
			continue
		}

		addflagvar(fs, fv, ft, opt)
	}
}

func prepareValue(rv reflect.Value, rt reflect.Type) {
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		rv.Set(reflect.New(rt.Elem()))
	}
}

func addflagvar(fs *pflag.FlagSet, rv reflect.Value, rt reflect.Type, opt *TagOpts) {
	if !rv.CanSet() {
		panic(fmt.Sprintf("field %s (%s) can not be set", opt.name, rv.Kind()))
	}

	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rt.Elem()))
		}
		rv = rv.Elem()
	}

	def := opt.Default
	switch p := rv.Addr().Interface().(type) {
	case *bool:
		addFlagField(fs, p, opt, fs.BoolVar, fs.BoolVarP, cast.ToBool(def))
	case *string:
		addFlagField(fs, p, opt, fs.StringVar, fs.StringVarP, cast.ToString(def))
	case *int32, *int16, *int8, *int:
		addFlagField(fs, p, opt, fs.IntVar, fs.IntVarP, cast.ToInt(def))
	case *int64:
		addFlagField(fs, p, opt, fs.Int64Var, fs.Int64VarP, cast.ToInt64(def))
	case *uint:
		addFlagField(fs, p, opt, fs.UintVar, fs.UintVarP, cast.ToUint(def))
	case *uint8:
		addFlagField(fs, p, opt, fs.Uint8Var, fs.Uint8VarP, cast.ToUint8(def))
	case *uint16:
		addFlagField(fs, p, opt, fs.Uint8Var, fs.Uint8VarP, cast.ToUint16(def))
	case *uint32:
		addFlagField(fs, p, opt, fs.Uint32Var, fs.Uint32VarP, cast.ToUint32(def))
	case *uint64:
		addFlagField(fs, p, opt, fs.Uint64Var, fs.Uint64VarP, cast.ToUint64(def))
	case *float32, *float64:
		addFlagField(fs, p, opt, fs.Float64Var, fs.Float64VarP, cast.ToFloat64(def))
	case *time.Duration:
		addFlagField(fs, p, opt, fs.DurationVar, fs.DurationVarP, cast.ToDuration(def))
	case *[]string:
		addFlagField(fs, p, opt, fs.StringArrayVar, fs.StringArrayVarP, cast.ToStringSlice(def))
	case *[]int:
		addFlagField(fs, p, opt, fs.IntSliceVar, fs.IntSliceVarP, cast.ToIntSlice(def))
	case *map[string]string:
		addFlagField(fs, p, opt, fs.StringToStringVar, fs.StringToStringVarP, cast.ToStringMapString(def))
	default:
		klog.V(6).InfoS("add config unsupported", "type", rt.String(), "kind", rt.Kind())
	}

	return
}

func addFlagField(fs *pflag.FlagSet, p interface{}, opt *TagOpts, varFn, varPFn, def interface{}) {
	switch len(opt.Flag) {
	case 0:
		// nothing
	case 1:
		reflect.ValueOf(varFn).Call([]reflect.Value{
			reflect.ValueOf(p),
			reflect.ValueOf(opt.Flag[0]),
			reflect.ValueOf(def),
			reflect.ValueOf(opt.Description),
		})
	case 2:
		reflect.ValueOf(varPFn).Call([]reflect.Value{
			reflect.ValueOf(p),
			reflect.ValueOf(opt.Flag[0]),
			reflect.ValueOf(opt.Flag[1]),
			reflect.ValueOf(def),
			reflect.ValueOf(opt.Description),
		})
	default:
		panic("invalid flag value")
	}
}
