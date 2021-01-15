package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
	"k8s.io/klog/v2"
)

const (
	reqEntityKey = "req-entity"
)

type Validator interface {
	Validate() error
}

// dst: must be ptr
func ReadEntity(req *restful.Request, dst interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("openapi.ReadEntity %s", r)
		}
	}()

	req.SetAttribute(reqEntityKey, dst)
	err = NewDecoder().Decode(req, dst)
	return
}

func ReqEntityFrom(r *restful.Request) (interface{}, bool) {
	entity := r.Attribute(reqEntityKey)
	return entity, entity != nil
}

// http.Request -> struct
type Decoder struct {
	r      *restful.Request
	header http.Header
	query  map[string][]string
	path   map[string]string
	data   bool
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

// Request -> struct
func (p *Decoder) Decode(r *restful.Request, dst interface{}) error {
	p.r = r
	p.header = r.Request.Header
	p.path = r.PathParameters()
	p.query = r.Request.URL.Query()

	rv := reflect.ValueOf(dst)
	rt := rv.Type()

	if rv.Kind() != reflect.Ptr {
		return status.Errorf(codes.InvalidArgument, "needs a pointer, got %s %s",
			rt.Kind().String(), rv.Kind().String())
	}

	if rv.IsNil() {
		return status.Errorf(codes.InvalidArgument, "invalid pointer(nil)")
	}

	rv = rv.Elem()
	rt = rv.Type()

	if rv.Kind() == reflect.Slice {
		return r.ReadEntity(dst)
	}

	if rv.Kind() != reflect.Struct || rv.Kind() == reflect.Slice || rt.String() == "time.Time" {
		return status.Errorf(codes.InvalidArgument,
			"schema: interface must be a pointer to struct")
	}

	fields := cachedTypeFields(rt)
	if body := fields.body; body != nil {
		ptr, err := getBodyPtr(rv, body.index)
		if err != nil {
			return err
		}
		if data, ok := r.Attribute(RshDataKey).([]byte); ok && len(data) > 0 {
			//klog.V(3).Infof(">>>> %s", string(data))
			if err := json.Unmarshal(data, ptr); err != nil {
				klog.V(5).Infof("rsh data json.Unmarshal() error %s", err)
				return err
			}
			//klog.V(3).Infof(">>> %s", dst)
		} else {
			if err := r.ReadEntity(ptr); err != nil {
				klog.V(5).Infof("restful.ReadEntity() error %s", err)
				return err
			}
		}
	}

	// p.decode will set p.data
	if err := p.decode(rv, fields); err != nil {
		return err
	}

	// postcheck
	if v, ok := dst.(Validator); ok {
		return v.Validate()
	}
	return nil
}

func (p *Decoder) decode(rv reflect.Value, fields structFields) error {
	klog.V(5).Info("entering openapi.decode()")

	for _, f := range fields.list {
		subv, err := getSubv(rv, f.index, true)
		if err != nil {
			return err
		}
		if err := p.setValue(&f, subv); err != nil {
			return err
		}
	}
	return nil
}

func (p *Decoder) setValue(f *field, v reflect.Value) error {
	var data []string
	var value string
	var ok bool

	key := f.name
	if key == "" {
		key = f.key
	}

	switch f.paramType {
	case PathType:
		if value, ok = p.path[key]; !ok {
			if f.required {
				return status.Errorf(codes.InvalidArgument, "%s must be set", key)
			}
			return nil
		}
		data = []string{value}
	case HeaderType:
		if value = p.header.Get(key); value == "" {
			if f.required {
				return status.Errorf(codes.InvalidArgument, "%s must be set", key)
			}
			return nil
		}
		data = []string{value}
	case QueryType:
		if data, ok = p.query[key]; !ok {
			if f.required {
				return status.Errorf(codes.InvalidArgument, "%s must be set", key)
			}
			return nil
		}
	default:
		panicType(f.typ, "invalid opt type")
	}

	if err := util.SetValue(v, data); err != nil {
		return err
	}

	return nil
}
