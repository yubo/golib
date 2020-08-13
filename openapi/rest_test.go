package openapi

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

func tearDown() {
	restful.DefaultContainer = restful.NewContainer()
}

func dbgFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	klog.V(8).Infof("tracing filter entering")
	if klog.V(8).Enabled() {
		klog.Infof("[req] HTTP %v %v", req.Request.Method, req.SelectedRoutePath())

		body, _ := ioutil.ReadAll(req.Request.Body)
		if len(body) > 0 {
			req.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
		klog.Info("[req] " + Req2curl(req.Request, body, nil, nil))
	}
	chain.ProcessFilter(req, resp)
	if klog.V(8).Enabled() {
		err := resp.Error()
		if resp.StatusCode() == http.StatusFound {
			klog.Infof("[resp] %v %v %d %s", req.Request.Method, req.SelectedRoutePath(), resp.StatusCode(), resp.Header().Get("location"))
		} else {
			klog.Infof("[resp] %v %v %d %v", req.Request.Method, req.SelectedRoutePath(), resp.StatusCode(), err)
		}
		if klog.V(10).Enabled() && err != nil {
			klog.Infof("[resp] %s", status.GetDetail(err))
		}
	}
}

func TestHttpParam(t *testing.T) {
	type getHost struct {
		Dir   *string `param:"path" json:"-"`
		Name  *string `param:"path" json:"-"`
		Query *string `param:"query" json:"-"`
		Ip    *string `param:"data" json:"ip"`
		Dns   *string `param:"data" json:"dns"`
	}

	cases := []getHost{{
		Dir:   util.String("dir"),
		Name:  util.String("name"),
		Query: util.String("query"),
		Ip:    util.String("1.1.1.1"),
		Dns:   util.String("jbo-ep-dev.fw"),
	}, {
		Dir:  util.String("dir"),
		Name: util.String("name"),
		Ip:   util.String(""),
		Dns:  util.String(""),
	}, {
		Dir:  util.String("dir"),
		Name: util.String("name"),
		Ip:   nil,
		Dns:  nil,
	}, {
		Dir:  util.String("dir"),
		Name: util.String("name"),
	}}

	for i, c := range cases {
		tearDown()
		ws := new(restful.WebService).Path("").Consumes("MIME_JSON")
		ws.Route(ws.POST("/api/v1/dirs/{dir}/hosts/{name}").Consumes(MIME_JSON).Filter(dbgFilter).
			To(func(req *restful.Request, resp *restful.Response) {
				out := getHost{}
				err := ReadEntity(req, &out)
				require.Emptyf(t, err, "case-%d", i)
				require.Equalf(t, c, out, "case-%d", i)
			}))
		restful.DefaultContainer.Add(ws)

		// write
		opt := &RequestOption{
			Method: "GET",
			Url:    "http://example.com/api/v1/dirs/{dir}/hosts/{name}",
			Input:  &c,
		}
		req, err := NewRequest(opt)
		if err != nil {
			t.Fatal(err)
		}
		require.Emptyf(t, err, "case-%d", i)

		httpWriter := httptest.NewRecorder()
		restful.DefaultContainer.DoNotRecover(false)
		restful.DefaultContainer.Dispatch(httpWriter, req.Request)
	}
}
