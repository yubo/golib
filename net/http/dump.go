package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type ResponseWriterRecorder struct {
	statusCode int
	buf        *bytes.Buffer
	w          io.Writer
	resp       http.ResponseWriter
	req        *http.Request
}

func (r *ResponseWriterRecorder) Header() http.Header {
	return r.resp.Header()
}

func (r *ResponseWriterRecorder) Write(b []byte) (int, error) {
	return r.w.Write(b)
}

func (r *ResponseWriterRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.resp.WriteHeader(statusCode)
}

func (r *ResponseWriterRecorder) Dump(body bool) ([]byte, error) {
	w := &bytes.Buffer{}

	text := http.StatusText(r.statusCode)
	if text == "" {
		text = "status code " + strconv.Itoa(r.statusCode)
	}

	if _, err := fmt.Fprintf(w, "HTTP/%d.%d %03d %s\r\n", r.req.ProtoMajor, r.req.ProtoMinor, r.statusCode, text); err != nil {
		return nil, err
	}
	if err := r.resp.Header().Write(w); err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n", r.buf.Len()); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return nil, err
	}

	if body {
		if _, err := io.Copy(w, bytes.NewReader(r.buf.Bytes())); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, "\r\n"); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

func NewResponseWriterRecorder(w http.ResponseWriter, req *http.Request) *ResponseWriterRecorder {
	buf := &bytes.Buffer{}

	return &ResponseWriterRecorder{
		resp: w,
		buf:  buf,
		w:    io.MultiWriter(w, buf),
		req:  req,
	}
}
