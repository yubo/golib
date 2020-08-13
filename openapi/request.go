package openapi

import (
	"bytes"
	"crypto/tls"
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

	"github.com/yubo/golib/openapi/urlencoded"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

func HttpRequest(in *RequestOption) (*http.Request, *http.Response, error) {
	req, err := NewRequest(in)
	if err != nil {
		return nil, nil, err
	}

	resp, err := req.Do()

	return req.Request, resp, err
}

type RequestOption struct {
	http.Client
	Url          string // https://example.com/api/v{version}/{model}/{object}?type=vm
	Method       string
	User         *string
	Pwd          *string
	Bearer       *string
	ApiKey       *string
	OtpCode      *string
	InputFile    *string // Priority is higher than Content
	InputContent []byte  // Priority is higher than input
	Input        interface{}
	OutputFile   *string // Priority is higher than output
	Output       interface{}
	Mime         string
	Header       map[string]string
}

func (p RequestOption) String() string {
	return util.Prettify(p)
}

type Request struct {
	*RequestOption
	Request      *http.Request
	url          string
	header       http.Header
	input        interface{}
	inputContent []byte
	inputReader  io.Reader
	inputCloser  io.Closer
	outputWriter io.Writer
	outputCloser io.Closer
}

func (p Request) String() string {
	return util.Prettify(p.RequestOption)
}

func NewRequest(in *RequestOption) (req *Request, err error) {
	req = &Request{RequestOption: in}

	// klog.V(10).Infof("newreqeust %s", in)

	if req.Mime == "" {
		req.Mime = MIME_JSON
	}

	if req.url, req.input, req.header, err = NewEncoder().Encode(req.Url, req.Input); err != nil {
		return nil, err
	}

	if util.StringValue(req.ApiKey) != "" {
		req.header.Set("X-API-Key", *req.ApiKey)
		req.Bearer = nil
	}

	if util.StringValue(req.User) != "" && util.StringValue(req.Pwd) != "" {
		req.header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(*req.User+":"+*req.Pwd)))
	}

	if req.Bearer != nil {
		req.header.Set("Authorization", "Bearer "+*req.Bearer)

		if req.OtpCode != nil {
			req.header.Set("X-Otp-Code", *req.OtpCode)
		}
	}

	for k, v := range in.Header {
		req.header.Set(k, v)
	}

	req.header.Set("Accept", "*/*")

	if err := req.prepareBody(); err != nil {
		return nil, err
	}

	if req.OutputFile != nil {
		fd, err := os.OpenFile(*req.OutputFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, err
		}
		req.outputWriter = fd
		req.outputCloser = fd
	}

	// http client
	if strings.HasPrefix(req.Url, "https:") || strings.HasPrefix(req.Url, "wss:") {
		req.Client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	req.Request, err = http.NewRequest(req.Method, req.url, req.inputReader)
	if err != nil {
		return nil, err
	}

	req.Request.Header = req.header

	// klog.V(10).Infof("req %s", req)
	return req, nil
}

func (p *Request) prepareBody() error {
	if p.InputFile != nil {
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
		p.inputReader = fd
		p.inputCloser = fd
		p.header.Set("Content-Length", fmt.Sprintf("%d", info.Size()))

		return nil
	}

	if len(p.InputContent) > 0 {
		p.header.Set("Content-Type", p.Mime)
		p.inputContent = p.InputContent
		p.inputReader = bytes.NewReader(p.inputContent)
		p.header.Set("Content-Length", fmt.Sprintf("%d", len(p.inputContent)))
		return nil
	}

	if p.input != nil {
		var err error
		switch p.Mime {
		case MIME_JSON:
			if p.inputContent, err = json.Marshal(p.input); err != nil {
				return err
			}
		case MIME_XML:
			if p.inputContent, err = xml.Marshal(p.input); err != nil {
				return err
			}
		case MIME_URL_ENCODED:
			if p.inputContent, err = urlencoded.Marshal(p.input); err != nil {
				return err
			}
		default:
			return errors.New("http request header Content-Type invalid " + p.Mime)
		}

		if p.Method != "GET" {
			p.header.Set("Content-Type", p.Mime)
			p.inputReader = bytes.NewReader(p.inputContent)
			p.header.Set("Content-Length", fmt.Sprintf("%d", len(p.inputContent)))
		}
		return nil
	}

	return nil
}

func (p *Request) Content() []byte {
	return p.inputContent
}

func (p *Request) HeaderSet(key, value string) {
	p.header.Set(key, value)
}

func (p *Request) Do() (resp *http.Response, err error) {
	var (
		respBody []byte
	)

	defer func() {
		if !klog.V(5).Enabled() {
			return
		}

		body := p.inputContent
		if len(body) > 1024 {
			body = body[:1024]
		}
		klog.Infof("[req] %s\n", Req2curl(p.Request,
			body, p.InputFile, p.OutputFile))

		buf := &bytes.Buffer{}
		HttpRespPrint(buf, resp, respBody)
		if buf.Len() > 0 {
			klog.Infof(buf.String())
		}
	}()

	if resp, err = p.Client.Do(p.Request); err != nil {
		return
	}

	defer func() {
		if p.inputCloser != nil {
			p.inputCloser.Close()
		}
		if p.outputCloser != nil {
			p.outputCloser.Close()
		}
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		respBody, _ = ioutil.ReadAll(resp.Body)
		err = fmt.Errorf("%d: %s", resp.StatusCode, respBody)
		return
	}

	if p.outputWriter != nil {
		_, err = io.Copy(p.outputWriter, resp.Body)
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
	return Req2curl(p.Request, p.inputContent, p.InputFile, p.OutputFile)
}
