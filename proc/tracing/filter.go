package tracing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"k8s.io/klog"
)

// TraceFilter will record all REST API request info and response code
func (p *Module) filter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	sp, err := p.startSpanWithHttp(req, resp)
	if err != nil {
		resp.WriteError(http.StatusBadRequest, err)
	}

	defer func() {
		code := resp.StatusCode()
		// panicErr := recover()
		// didPanic := panicErr != nil

		if code == 0 {
			code = 200
		}
		if code > 0 {
			ext.HTTPStatusCode.Set(sp, uint16(code))
		}
		if code >= http.StatusInternalServerError {
			ext.Error.Set(sp, true)
		}

		if err := resp.Error(); err != nil {
			sp.LogFields(log.Error(err))
		}

		sp.Finish()

		// if didPanic {
		// 	panic(panicErr)
		// }
	}()

	chain.ProcessFilter(req, resp)
}

func (p *Module) startSpanWithHttp(req *restful.Request, resp *restful.Response) (opentracing.Span, error) {
	r := req.Request
	tr := opentracing.GlobalTracer()

	opts := []opentracing.StartSpanOption{}
	ctx, _ := tr.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if ctx != nil {
		opts = append(opts, ext.RPCServerOption(ctx))
	}

	sp := tr.StartSpan(fmt.Sprintf("HTTP %s %s", req.Request.Method, req.SelectedRoutePath()), opts...)
	ext.HTTPUrl.Set(sp, r.URL.String())

	r = r.WithContext(opentracing.ContextWithSpan(r.Context(), sp))

	fields := []log.Field{}
	if p.HttpBody && r.Method != "GET" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			sp.LogFields(log.String("evnet", "readAll http.body"), log.Error(err))
			sp.Finish()
			return nil, fmt.Errorf("connot read body")
		}

		if body != nil {
			fields = append(fields, log.String("http body", string(body)))
		}

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}

	if p.HttpHeader {
		for k, v := range r.Header {
			for _, v1 := range v {
				fields = append(fields, log.String("http head "+k, v1))
			}
		}
	}

	if len(fields) > 0 {
		sp.LogFields(fields...)
	}

	req.Request = r

	if p.RespTraceId {
		carrier := opentracing.HTTPHeadersCarrier(resp.Header())
		if err := tr.Inject(sp.Context(), opentracing.HTTPHeaders, carrier); err != nil {
			klog.Errorf("tracer inject err %s", err)
		}
	}

	return sp, nil
}
