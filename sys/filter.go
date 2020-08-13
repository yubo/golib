package sys

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/openapi/api"
	"github.com/yubo/golib/status"
	"k8s.io/klog/v2"
)

type debugReadCloser struct {
	io.Reader
}

func (p *debugReadCloser) Read(in []byte) (int, error) {
	return p.Reader.Read(in)
}

func (p *debugReadCloser) Close() error { return nil }

func DebugNopCloser(r io.Reader) io.ReadCloser {
	return &debugReadCloser{r}
}

// klog level >= 8
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

type LogEntity interface {
	Log() (action, target string, data interface{})
}

func (p *Module) LogFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	chain.ProcessFilter(req, resp)

	in, ok := api.ReqEntityFrom(req)
	if !ok {
		return
	}

	entity, ok := in.(LogEntity)
	if !ok {
		return
	}

	action, target, data := entity.Log()
	if err := p.Log5(req, &action, &target, data, resp.Error()); err != nil {
		klog.Error(err)
	}
	return
}
