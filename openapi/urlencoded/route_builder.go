package urlencoded

import (
	"reflect"

	restful "github.com/emicklei/go-restful"
	"github.com/yubo/golib/util"
)

func RouteBuilderReads(b *restful.RouteBuilder, rv reflect.Value, rf reflect.StructField, rt reflect.Type) error {
	if rv.Kind() != reflect.Struct {
		return buildParam(b, rv, rf, rt)
	}

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		panic("schema: interface must be a struct, got " + rt.String())
	}

	for i := 0; i < rt.NumField(); i++ {
		fv := rv.Field(i)
		ff := rt.Field(i)
		ft := ff.Type
		if err := buildParam(b, fv, ff, ft); err != nil {
			panic(err)
		}
	}
	return nil
}

func buildParam(b *restful.RouteBuilder, rv reflect.Value, rf reflect.StructField, rt reflect.Type) error {
	//rt := rt.Field(i)
	if !rv.CanInterface() {
		return nil
	}

	name, format, skip, inline := getTags(rf)
	if skip {
		return nil
	}

	util.PrepareValue(rv, rt)
	if inline {
		if err := RouteBuilderReads(b, rv, rf, rt); err != nil {
			panic(err)
		}
		return nil
	}

	parameter := restful.FormParameter(name, rf.Tag.Get("description"))

	switch rv.Kind() {
	case reflect.String:
		parameter.DataType("string")
	case reflect.Bool:
		parameter.DataType("bool")
	case reflect.Uint, reflect.Int, reflect.Int32, reflect.Int64:
		parameter.DataType("integer")
	case reflect.Slice:
		if typeName := rv.Type().Elem().Name(); typeName != "string" {
			panic("unsupported param " + rf.Name)
		}
	default:
		panic("unsupported type " + rt.String())
	}

	if format != "" {
		parameter.DataFormat(format)
	}

	b.Param(parameter)

	return nil

}
