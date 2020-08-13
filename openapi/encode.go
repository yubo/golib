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
		if err = p.scan(src); err != nil {
			panic(err)
		}
		if p.data == nil && len(p.data2) > 0 {
			p.data = p.data2
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
func (p *Encoder) scan(sample interface{}) error {
	rv := reflect.Indirect(reflect.ValueOf(sample))
	rt := rv.Type()

	if rv.Kind() != reflect.Struct || rt.String() == "time.Time" {
		return errors.New(fmt.Sprintf("rest-encode: input must be a struct, got %v/%v", rv.Kind(), rt))
	}

	for i := 0; i < rt.NumField(); i++ {
		fv := rv.Field(i)
		ff := rt.Field(i)
		ft := ff.Type

		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				continue
			}
			// fv = fv.Elem()
		}

		if !fv.CanInterface() {
			continue
		}

		opt := getTagOpt(ff)
		if opt.skip {
			continue
		}

		if opt.inbody {
			p.data = fv.Interface()
			return nil
		}

		if opt.inline {
			if err := p.scan(fv.Interface()); err != nil {
				return err
			}
			continue
		}

		if opt.typ == DataType {
			p.data2[opt.name] = fv.Interface()
			continue
		}

		data, err := util.GetValue(fv, ft)
		if err != nil {
			return err
		}

		if opt.typ == PathType {
			p.path[opt.name] = data[0]
		} else if opt.typ == QueryType {
			p.param[opt.name] = data
		} else if opt.typ == HeaderType {
			p.header.Set(opt.name, data[0])
		} else {
			return errors.New("invalid kind: " + opt.typ + " " + ff.Type.String())
		}
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
