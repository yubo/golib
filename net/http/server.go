/*
 * Copyright 2022 yubo. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"sort"
	"sync"
)

type methodType struct {
	ServiceMethod string      `json:"method"` // serviceName.methodName
	Request       interface{} `json:"request"`
	Response      interface{} `json:"response"`
	serviceName   string
	mname         string
	rcvr          reflect.Value // receiver of methods for the service
	method        reflect.Method
	reqType       reflect.Type
	respType      reflect.Type
}

type Request struct {
	Method  string          `json:"method"` // format: "Service.Method"
	Request json.RawMessage `json:"request"`
}

type Server struct {
	methods map[string]*methodType
	list    []*methodType
	once    sync.Once
}

func NewServer() *Server {
	return &Server{
		methods: make(map[string]*methodType),
	}
}

func (server *Server) Register(rcvr any) error {
	return server.register(rcvr, "", false)
}

func (server *Server) RegisterName(name string, rcvr any) error {
	return server.register(rcvr, name, true)
}

func (server *Server) register(rcvr interface{}, name string, useName bool) error {
	rt := reflect.TypeOf(rcvr)
	rv := reflect.ValueOf(rcvr)
	sname := reflect.Indirect(rv).Type().Name()
	if useName {
		sname = name
	}

	if rt.Kind() != reflect.Pointer {
		return fmt.Errorf("server.Register: must be a pointer %s", rt)
	}
	if sname == "" {
		s := "server.Register: no service name for type " + rt.String()
		log.Print(s)
		return errors.New(s)
	}

	if rt.NumMethod() == 0 {
		rt = reflect.PointerTo(rt)
	}

	for i := 0; i < rt.NumMethod(); i++ {
		rm := rt.Method(i)

		if !rm.IsExported() {
			continue
		}

		method, err := newMethodType(sname, rcvr, rm)
		if err != nil {
			return err
		}

		if method != nil {
			if _, ok := server.methods[method.ServiceMethod]; ok {
				return fmt.Errorf("servcieMethod %s is exists", method.ServiceMethod)
			}

			server.methods[method.ServiceMethod] = method
			server.list = append(server.list, method)
		}
	}

	return nil

}

func (server *Server) listHandle(w http.ResponseWriter, r *http.Request) {
	server.once.Do(func() {
		sort.Slice(server.list, func(i, j int) bool { return server.list[i].ServiceMethod < server.list[j].ServiceMethod })
	})
	server.writeJson(w, server.list)
}

func newMethodType(name string, in interface{}, rm reflect.Method) (*methodType, error) {
	method := &methodType{
		serviceName:   name,
		rcvr:          reflect.ValueOf(in),
		mname:         rm.Name,
		ServiceMethod: fmt.Sprintf("%s.%s", name, rm.Name),
		method:        rm,
	}

	mtype := rm.Type
	numIn := mtype.NumIn()
	numOut := mtype.NumOut()

	if numIn != 2 && numIn != 3 {
		return nil, fmt.Errorf("server.Register: method %q has %d input parameters; needs exactly 2|3", method.ServiceMethod, mtype.NumIn())
	}

	if ctx := mtype.In(1).String(); ctx != "context.Context" {
		return nil, fmt.Errorf("expected func (p %s) %s(context.Context, any) got %s", method.serviceName, method.mname, ctx)
	}

	if numIn == 3 {
		method.reqType = mtype.In(2)
		//if method.req = mtype.In(2); method.req.Kind() != reflect.Ptr {
		//	return nil, fmt.Errorf("%s(ctx context.Context, req any); req must be a ptr", method.ServiceMethod)
		//}
		method.Request = newElem(method.reqType)
	}

	if numOut != 1 && numOut != 2 {
		return nil, fmt.Errorf("server.Register: method %q has %d input parameters; needs exactly 1|2", method.ServiceMethod, mtype.NumIn())
	}

	if numOut == 1 {
		if err := mtype.Out(0).String(); err != "error" {
			return nil, fmt.Errorf("expected func %s(...) (err error)", method.ServiceMethod)
		}
	}

	if numOut == 2 {
		method.respType = mtype.Out(0)
		if err := mtype.Out(1).String(); err != "error" {
			return nil, fmt.Errorf("expected func %s(...) (resp %s, err error)", method.respType.String(), method.ServiceMethod)
		}
		method.Response = newElem(method.respType)
	}

	return method, nil

}

func (server *Server) decodeRequest(r *http.Request) (*Request, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	req := &Request{}
	if err := json.Unmarshal(body, req); err != nil {
		return nil, err
	}

	return req, nil
}

func (p *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request, err := p.decodeRequest(r)
	if err != nil {
		p.writeError(w, err)
		return
	}

	if request.Method == "list" {
		p.listHandle(w, r)
		return
	}

	mtype, ok := p.methods[request.Method]
	if !ok {
		p.writeError(w, fmt.Errorf("action %s not found", request.Method))
		return
	}

	in := []reflect.Value{mtype.rcvr, reflect.ValueOf(r.Context())}

	if mtype.reqType != nil {
		var reqv reflect.Value
		argIsValue := false // if true, need to indirect before calling.
		if mtype.reqType.Kind() == reflect.Pointer {
			reqv = reflect.New(mtype.reqType.Elem())
		} else {
			reqv = reflect.New(mtype.reqType)
			argIsValue = true
		}

		if err := json.Unmarshal(request.Request, reqv.Interface()); err != nil {
			p.writeError(w, err)
			return
		}
		if argIsValue {
			reqv = reqv.Elem()
		}
		in = append(in, reqv)
	}

	ret := mtype.method.Func.Call(in)

	if mtype.respType != nil {
		if err, _ := ret[1].Interface().(error); err != nil {
			p.writeError(w, err)
			return
		}

		p.writeJson(w, toInterface(ret[0]))
		return
	}

	if err, _ := ret[0].Interface().(error); err != nil {
		p.writeError(w, err)
		return
	}

	p.writeJson(w, "ok")
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func (s *Server) writeJson(w http.ResponseWriter, resp interface{}) {
	if b, ok := resp.([]byte); ok {
		w.Header().Set("Content-Type", "text/plan")
		w.Write(b)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func toInterface(v reflect.Value) interface{} {
	if v.CanInterface() {
		return v.Interface()
	}
	return nil
}

func newElem(rt reflect.Type) interface{} {
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	return reflect.New(rt).Interface()
}
