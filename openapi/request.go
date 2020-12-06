package openapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/yubo/golib/openapi/urlencoded"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

func HttpRequest(in *RequestOptions) (*http.Request, *http.Response, error) {
	req, err := NewRequest(in)
	if err != nil {
		return nil, nil, err
	}

	resp, err := req.Do()

	return req.Request, resp, err
}

type Request struct {
	*RequestOptions
	Request        *http.Request
	url            string
	bodyObject     interface{} // the object in http.body
	bodyContent    []byte
	bodyReader     io.Reader
	bodyCloser     io.Closer
	responseWriter io.Writer
	responseCloser io.Closer
}

func NewRequest(in *RequestOptions, opts ...RequestOption) (req *Request, err error) {
	for _, opt := range opts {
		opt.apply(in)
	}

	req = &Request{RequestOptions: in}

	if err = req.prepare(); err != nil {
		return nil, err
	}

	req.Request, err = http.NewRequest(req.Method, req.url, req.bodyReader)
	if err != nil {
		return nil, err
	}

	req.Request.Header = req.header

	klog.V(10).Infof("req %s", req)
	return req, nil
}

func (p *Request) prepare() error {
	if p.Mime == "" {
		p.Mime = MIME_JSON
	}

	if err := p.prepareInput(); err != nil {
		return err
	}

	if err := p.prepareBody(); err != nil {
		return err
	}

	if p.ApiKey != nil {
		p.header.Set("X-API-Key", *p.ApiKey)
	}

	if p.Bearer != nil {
		p.header.Set("Authorization", "Bearer "+*p.Bearer)
	}

	if p.User != nil && p.Pwd != nil {
		p.header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(*p.User+":"+*p.Pwd)))
	}

	if p.header.Get("Accept") == "" {
		p.header.Set("Accept", "*/*")
	}

	if p.Client.Transport == nil {
		var err error
		if p.Client.Transport, err = p.Transport(); err != nil {
			return err
		}
	}

	if filePath := strings.TrimSpace(util.StringValue(p.OutputFile)); filePath != "" {
		if filePath == "-" {
			p.responseWriter = os.Stdout
		} else {
			fd, err := os.OpenFile(*p.OutputFile, os.O_RDWR|os.O_CREATE, 0644)
			if err != nil {
				return err
			}
			p.responseWriter = fd
			p.responseCloser = fd
		}
	}

	return nil
}

func (p Request) String() string {
	return util.Prettify(p.RequestOptions)
}

func (p *Request) prepareInput() (err error) {
	p.url, p.bodyObject, p.header, err = (&Encoder{
		path:   map[string]string{},
		param:  map[string][]string{},
		header: p.header,
		data2:  map[string]interface{}{},
	}).Encode(p.Url, p.Input)
	return err
}

func (p *Request) prepareBody() error {
	if filePath := strings.TrimSpace(util.StringValue(p.InputFile)); filePath != "" {
		if filePath == "-" {
			b, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			p.InputContent = b
		} else {
			info, err := os.Stat(*p.InputFile)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return fmt.Errorf("%s is dir", *p.InputFile)
			}

			fd, err := os.Open(*p.InputFile)
			if err != nil {
				return err
			}
			p.bodyReader = fd
			p.bodyCloser = fd
			p.header.Set("Content-Length", fmt.Sprintf("%d", info.Size()))

			return nil
		}
	}

	if len(p.InputContent) > 0 {
		p.header.Set("Content-Type", p.Mime)
		p.bodyContent = p.InputContent
		p.bodyReader = bytes.NewReader(p.bodyContent)
		p.header.Set("Content-Length", fmt.Sprintf("%d", len(p.bodyContent)))
		return nil
	}

	if p.bodyObject != nil {
		var err error
		switch p.Mime {
		case MIME_JSON:
			if p.bodyContent, err = json.Marshal(p.bodyObject); err != nil {
				return err
			}
		case MIME_XML:
			if p.bodyContent, err = xml.Marshal(p.bodyObject); err != nil {
				return err
			}
		case MIME_URL_ENCODED:
			if p.bodyContent, err = urlencoded.Marshal(p.bodyObject); err != nil {
				return err
			}
		default:
			return errors.New("http request header Content-Type invalid " + p.Mime)
		}

		if p.Method != "GET" {
			p.header.Set("Content-Type", p.Mime)
			p.bodyReader = bytes.NewReader(p.bodyContent)
			p.header.Set("Content-Length", fmt.Sprintf("%d", len(p.bodyContent)))
		}
		return nil
	}

	return nil
}

func (p *Request) Content() []byte {
	return p.bodyContent
}

func (p *Request) HeaderSet(key, value string) {
	p.header.Set(key, value)
}

func (p *Request) Do() (resp *http.Response, err error) {
	var respBody []byte
	r := p.Request

	defer func() {
		if !klog.V(5).Enabled() {
			return
		}

		body := p.bodyContent
		if len(body) > 1024 {
			body = body[:1024]
		}
		klog.Infof("[req] %s\n", Req2curl(r, body, p.InputFile, p.OutputFile))

		buf := &bytes.Buffer{}
		HttpRespPrint(buf, resp, respBody)
		if buf.Len() > 0 {
			klog.Infof(buf.String())
		}
	}()

	// ctx & tracer
	if sp := opentracing.SpanFromContext(p.Ctx); sp != nil {
		p.Client.Transport = &nethttp.Transport{}

		r = r.WithContext(p.Ctx)
		var ht *nethttp.Tracer
		r, ht = nethttp.TraceRequest(sp.Tracer(), r)
		defer ht.Finish()
	}

	if resp, err = p.Client.Do(r); err != nil {
		return
	}

	defer func() {
		if p.bodyCloser != nil {
			p.bodyCloser.Close()
		}
		if p.responseCloser != nil {
			p.responseCloser.Close()
		}
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		respBody, _ = ioutil.ReadAll(resp.Body)
		err = fmt.Errorf("%d: %s", resp.StatusCode, respBody)
		return
	}

	if p.responseWriter != nil {
		_, err = io.Copy(p.responseWriter, resp.Body)
		return
	}

	if out, ok := p.Output.(io.Writer); ok {
		_, err = io.Copy(out, resp.Body)
		return
	}

	if respBody, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	if p.Output == nil {
		return
	}

	switch mime := resp.Header.Get("Content-Type"); mime {
	case MIME_XML:
		err = xml.Unmarshal(respBody, p.Output)
	case MIME_URL_ENCODED:
		err = urlencoded.Unmarshal(respBody, p.Output)
	case MIME_JSON:
		err = json.Unmarshal(respBody, p.Output)
	default:
		err = json.Unmarshal(respBody, p.Output)
	}

	return
}

func (p *Request) Curl() string {
	return Req2curl(p.Request, p.bodyContent, p.InputFile, p.OutputFile)
}
