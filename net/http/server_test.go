package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hw struct{}

func (h *hw) Hello(ctx context.Context) (string, error) {
	return "hello world", nil
}

func (h *hw) Add(ctx context.Context, req *Request) (*Response, error) {
	return "hello world", nil
}

func TestNewHandlerFunc(t *testing.T) {
	server := NewServer()
	server.Register(new(hw))

	svc := httptest.NewServer(server)
	defer svc.Close()

	b, _ := json.Marshal(Request{Method: "hw.Hello"})
	req, err := http.NewRequest("POST", svc.URL, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = io.ReadAll(res.Body)
	t.Logf("resp: %s", b)
	res.Body.Close()

	if g, w := res.StatusCode, http.StatusOK; g != w {
		t.Fatalf("Status code mismatch: got %d, want %d", g, w)
	}

}
