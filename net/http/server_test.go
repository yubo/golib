package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"
)

type hw struct{}

func (h *hw) Hello(ctx context.Context) ([]byte, error) {
	return []byte("hello world"), nil
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
	resp, err := svc.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	b, _ = httputil.DumpResponse(resp, true)
	t.Logf("resp: %s", string(b))

	if g, w := resp.StatusCode, http.StatusOK; g != w {
		t.Fatalf("Status code mismatch: got %d, want %d", g, w)
	}

}
