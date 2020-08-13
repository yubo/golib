package tracing

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/status"
	"k8s.io/klog/v2"
)

type debugReadCloser struct {
	io.Reader
}

func (p *debugReadCloser) Read(in []byte) (int, error) {
	n, err := p.Reader.Read(in)
	klog.V(5).Infof("read() %d, %v", n, err)
	return n, err
}

func (p *debugReadCloser) Close() error { return nil }

func DebugNopCloser(r io.Reader) io.ReadCloser {
	return &debugReadCloser{r}
}

func DbgFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	klog.Info("tracing filter entering")
	klog.Infof("[req] HTTP %s %s", req.Request.Method, req.SelectedRoutePath())

	body, _ := ioutil.ReadAll(req.Request.Body)
	if len(body) > 0 {
		req.Request.Body = DebugNopCloser(bytes.NewBuffer(body))
	}
	klog.Info("[req] " + openapi.Req2curl(req.Request, body, nil, nil))

	chain.ProcessFilter(req, resp)

	err := resp.Error()
	if resp.StatusCode() == http.StatusFound {
		klog.Infof("[resp] %s %s %d %s", req.Request.Method, req.SelectedRoutePath(), resp.StatusCode(), resp.Header().Get("location"))
	} else {
		klog.Infof("[resp] %s %s %d %v", req.Request.Method, req.SelectedRoutePath(), resp.StatusCode(), err)
	}
	if err != nil {
		klog.Infof("[resp] %s", status.GetDetail(err))
	}
}

// Filter will record all REST API request info and response code
func Filter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	klog.Infof("filter entering")
	gTracer := opentracing.GlobalTracer()
	if gTracer == nil {
		chain.ProcessFilter(req, resp)
		return
	}

	spanName := fmt.Sprintf("HTTP %s %s",
		req.Request.Method, req.SelectedRoutePath())
	span := opentracing.StartSpan(spanName)
	defer span.Finish()

	tracer := GetTracerWithSpan(span)

	body, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		tracer.Errorf("error reading body: %v", err)
		resp.WriteErrorString(http.StatusBadRequest, "cannot read body")
		return
	}

	if body != nil {
		tracer.V(5).LogKV("request body", string(body))
	}

	req.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	reqID := GetRequestIDWithSpan(span)
	resp.ResponseWriter.Header().Add("Request-ID", reqID)

	ctx := opentracing.ContextWithSpan(req.Request.Context(), span)
	nr := req.Request.WithContext(ctx)
	req.Request = nr

	chain.ProcessFilter(req, resp)

	if err := resp.Error(); err != nil {
		tracer.Error(err)
	}

	ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode()))
}
