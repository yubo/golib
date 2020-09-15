package openapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

// struct -> http.Reqeust's param
type Encoder struct {
	path   map[string]string
	param  map[string][]string
	header http.Header
	url    string
	data   interface{}            // store for struct direct param:",inbody"
	data2  map[string]interface{} // store for param:"data"
}

func NewEncoder() *Encoder {
	return &Encoder{
		path:   map[string]string{},
		param:  map[string][]string{},
		header: make(http.Header),
		data2:  map[string]interface{}{},
	}
}

func (p *Encoder) Encode(url1 string, src interface{}) (url2 string, data interface{}, header http.Header, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("openapi.Encode %s", r)
		}
	}()

	header = p.header
	data = p.data

	// precheck
	if v, ok := src.(Validator); ok {
		if err = v.Validate(); err != nil {
			klog.V(1).Infof("%s.Validate() err: %s",
				reflect.TypeOf(src), err)
			return
		}
	}

	if src != nil {
		rv := reflect.Indirect(reflect.ValueOf(src))
		rt := rv.Type()

		if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
			err = errors.New(fmt.Sprintf("rest-encode: input must be a struct, got %v/%v", rv.Kind(), rt))
			return
		}

		fields := cachedTypeFields(rt)

		if fields.hasData && p.data == nil {
			p.data = src
		}

		if err = p.scan(rv, fields); err != nil {
			panic(err)
		}
	}

	if url2, err = invokePathVariable(url1, p.path); err != nil {
		return
	}

	var u *url.URL
	if u, err = url.Parse(url2); err != nil {
		return
	}

	v := u.Query()
	for k1, v1 := range p.param {
		for _, v2 := range v1 {
			v.Add(k1, v2)
		}
	}
	u.RawQuery = v.Encode()
	url2 = u.String()

	return url2, p.data, p.header, nil
}

// struct -> request's path, query, header, data
func (p *Encoder) scan(rv reflect.Value, fields structFields) error {
	klog.V(5).Info("entering openapi.scan()")

	for i, f := range fields.list {
		klog.V(11).Infof("%s[%d] %s", rv.Type(), i, f)
		subv, err := getSubv(rv, f.index, false)
		if subv.IsNil() || err != nil {
			if f.required {
				return fmt.Errorf("%v must be set", f.key)
			}
			continue
		}
		if f.paramType == DataType {
			continue
		}
		if err := p.setValue(&f, subv); err != nil {
			klog.V(11).Infof("f %v subv %v", f, subv)
			return err
		}
	}

	return nil
}

func (p *Encoder) setValue(f *field, v reflect.Value) error {
	data, err := util.GetValue(v)
	if err != nil {
		return err
	}

	key := f.name
	if key == "" {
		key = f.key
	}

	switch f.paramType {
	case PathType:
		p.path[key] = data[0]
	case QueryType:
		p.param[key] = data
	case HeaderType:
		p.header.Set(key, data[0])
	default:
		return fmt.Errorf("invalid kind: %s %s", f.paramType, f.key)
	}
	return nil

}

func invokePathVariable(rawurl string, data map[string]string) (string, error) {
	var buf strings.Builder
	var begin int

	match := false
	for i, c := range []byte(rawurl) {
		if !match {
			if c == '{' {
				match = true
				begin = i
			} else {
				buf.WriteByte(c)
			}
			continue
		}

		if c == '}' {
			k := rawurl[begin+1 : i]
			if v, ok := data[k]; ok {
				buf.WriteString(url.PathEscape(v))
			} else {
				return "", fmt.Errorf("param {%s} not found in data (%s)", k, util.JsonStr(data, true))
			}
			match = false
		}
	}

	if match {
		return "", fmt.Errorf("param %s is not ended", rawurl[begin:])
	}

	return buf.String(), nil
}
