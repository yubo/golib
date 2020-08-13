package openapi

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi/api"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
	"k8s.io/klog/v2"
)

type Validator interface {
	Validate() error
}

// dst: must be ptr
func ReadEntity(req *restful.Request, dst interface{}) error {
	req.SetAttribute(ReqEntityKey, dst)
	return NewDecoder().Decode(req, dst)
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

	// p.decode will set p.data
	if err := p.decode(rv, rt); err != nil {
		return err
	}

	if p.data {
		if r.Request.Method == "GET" {
			data, ok := r.Attribute(api.RshDataKey).([]byte)
			if ok && len(data) > 0 {
				//klog.V(3).Infof(">>>> %s", string(data))
				if err := json.Unmarshal(data, dst); err != nil {
					klog.V(5).Infof("rsh data json.Unmarshal() error %s", err)
					return err
				}
				//klog.V(3).Infof(">>> %s", dst)
			}
		} else {
			if err := r.ReadEntity(dst); err != nil {
				klog.V(5).Infof("restful.ReadEntity() error %s", err)
				return err
			}
		}
	}

	// postcheck
	if v, ok := dst.(Validator); ok {
		return v.Validate()
	}
	return nil
}

func (p *Decoder) decode(rv reflect.Value, rt reflect.Type) error {
	klog.V(5).Info("entering openapi.decode()")

	if rv.Kind() != reflect.Struct || rv.Kind() == reflect.Slice || rt.String() == "time.Time" {
		return status.Errorf(codes.InvalidArgument, "schema: interface must be a pointer to struct")
	}

	for i := 0; i < rt.NumField(); i++ {
		// Notify: ignore ptr, use indirect elem
		fv := rv.Field(i)
		ff := rt.Field(i)
		ft := ff.Type

		opt := getTagOpt(ff)

		if !fv.CanSet() {
			klog.V(5).Infof("can't addr name %s, continue", opt.name)
			continue
		}

		if opt.inbody {
			util.PrepareValue(fv, ft)
			if fv.Kind() == reflect.Ptr {
				fv = fv.Elem()
				ft = fv.Type()
			}
			return p.r.ReadEntity(fv.Addr().Interface())
		}

		if opt.inline {
			util.PrepareValue(fv, ft)
			if fv.Kind() == reflect.Ptr {
				fv = fv.Elem()
				ft = fv.Type()
			}
			if err := p.decode(fv, ft); err != nil {
				return err
			}
			continue
		}

		if opt.skip {
			continue
		}

		if opt.typ == "data" {
			p.data = true
			continue
		}

		{
			var data []string
			var value string
			var ok bool

			switch opt.typ {
			case PathType:
				if value, ok = p.path[opt.name]; !ok {
					continue
				}
				data = []string{value}
			case HeaderType:
				if value = p.header.Get(opt.name); value == "" {
					continue
				}
				data = []string{value}
			case QueryType:
				if data, ok = p.query[opt.name]; !ok {
					continue
				}
			default:
				panic("invalid opt type " + opt.typ)
			}

			if err := util.SetValue(fv, ft, data); err != nil {
				return err
			}
		}
	}
	return nil
}
