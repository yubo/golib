package http

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/status"
	"k8s.io/klog/v2"
)

func DbgFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	klog.V(8).Info("tracing filter entering")
	klog.Infof("[req] HTTP %s %s", req.Request.Method, req.SelectedRoutePath())

	body, _ := ioutil.ReadAll(req.Request.Body)
	if len(body) > 0 {
		req.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
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
