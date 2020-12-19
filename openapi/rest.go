package openapi

import (
	"fmt"
	"io"
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/status"
	"k8s.io/klog/v2"
)

const (
	// Accept or Content-Type used in Consumes() and/or Produces()
	MIME_JSON        = "application/json"
	MIME_XML         = "application/xml"
	MIME_TXT         = "text/plain"
	MIME_URL_ENCODED = "application/x-www-form-urlencoded"
	MIME_OCTET       = "application/octet-stream" // If Content-Type is not present in request, use the default

	PathType   = "path"
	QueryType  = "query"
	HeaderType = "header"
	DataType   = "data"

	MaxFormSize = int64(1<<63 - 1)
)

func RespWriter(resp *restful.Response, data interface{}, err error) {
	if err != nil {
		code := http.StatusBadRequest
		if s, ok := status.FromError(err); ok {
			code = status.HTTPStatusFromCode(s.Code())
		}
		resp.WriteError(code, err)

		klog.V(3).Infof("response %d %s", code, err.Error())
	} else {
		resp.WriteEntity(data)
	}
}

// wrapper data and error
func RespWriterErrInBody(resp *restful.Response, data interface{}, err error) {
	var eMsg string
	code := 200

	if err != nil {
		eMsg = err.Error()
		code = http.StatusBadRequest

		if s, ok := status.FromError(err); ok {
			code = status.HTTPStatusFromCode(s.Code())
		}
		if klog.V(3).Enabled() {
			klog.ErrorDepth(1, fmt.Sprintf("httpReturn %d %s", code, eMsg))
		}
	}

	resp.WriteEntity(map[string]interface{}{
		"data": data,
		"err":  eMsg,
		"code": code,
	})
}

type Tx interface {
	Tx() bool
	Commit() error
	Rollback() error
}

//  Deprecated
func HttpWriteData(resp *restful.Response, data interface{}, err error, tx ...Tx) {
	var eMsg string
	code := 200

	if len(tx) > 0 && tx != nil {
		txClose(tx[0], err)
	}

	if err != nil {
		eMsg = err.Error()
		code = http.StatusBadRequest

		if s, ok := status.FromError(err); ok {
			code = status.HTTPStatusFromCode(s.Code())
		}
		if klog.V(3).Enabled() {
			klog.ErrorDepth(1, fmt.Sprintf("httpReturn %d %s", code, eMsg))
		}
	}

	resp.WriteEntity(map[string]interface{}{
		"data": data,
		"err":  eMsg,
		"code": code,
	})
}

func HttpWriteList(resp *restful.Response, total int64, list interface{}, err error) {
	var eMsg string
	code := 200

	if err != nil {
		eMsg = err.Error()
		code = http.StatusBadRequest

		if s, ok := status.FromError(err); ok {
			code = status.HTTPStatusFromCode(s.Code())
		}
	} else if list == nil {
		list = []string{}
	}

	resp.WriteEntity(map[string]interface{}{
		"data": map[string]interface{}{
			"total": total,
			"list":  list,
		},
		"err":  eMsg,
		"code": code,
	})
}

func txClose(tx Tx, err error) error {
	if tx == nil || !tx.Tx() {
		return nil
	}

	if err == nil {
		return tx.Commit()
	}
	return tx.Rollback()
}

func HttpRedirect(w http.ResponseWriter, url string) {
	w.Header().Add("location", url)
	w.WriteHeader(http.StatusFound)
}

func HttpRedirectErr(resp *restful.Response, url string, err error) {
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	resp.ResponseWriter.Header().Add("location", url)
	resp.ResponseWriter.WriteHeader(http.StatusFound)
}

func HttpWriteEntity(resp *restful.Response, in interface{}, err error) {
	if err != nil {
		HttpWriteErr(resp, err)
		return
	}

	if s, ok := in.(string); ok {
		resp.Write([]byte(s))
		return
	}

	resp.WriteEntity(in)
}

func HttpWrite(resp *restful.Response, data []byte, err error) {
	if err != nil {
		HttpWriteErr(resp, err)
		return
	}

	resp.Write(data)
}

func HttpWriteErr(resp *restful.Response, err error) {
	if err == nil {
		return
	}

	s, ok := status.FromError(err)
	if !ok {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if code := status.HTTPStatusFromCode(s.Code()); code != 200 {
		resp.WriteError(code, err)
	}

}

func HttpRespPrint(out io.Writer, resp *http.Response, body []byte) {
	if out == nil || resp == nil {
		return
	}

	fmt.Fprintf(out, "[resp]\ncode: %d\n", resp.StatusCode)
	fmt.Fprintf(out, "header:\n")

	for k, v := range resp.Header {
		for _, v1 := range v {
			fmt.Fprintf(out, "  %s: %s\n", k, v1)
		}
	}

	if len(body) > 0 {
		fmt.Fprintf(out, "body:\n%s\n", string(body))
	}
}
