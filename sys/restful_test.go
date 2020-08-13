package sys

import (
	"net/http"
	"testing"

	restful "github.com/emicklei/go-restful"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/openapi"
)

func TestScanReqIntoStruct(t *testing.T) {
	type Bar struct {
		X int `param:"x"`
		Y int `param:"y"`
	}
	type foo struct {
		Bar `param:",inline"`
		B   bool     `param:"b"`
		I   int32    `param:"i"`
		S   string   `param:"s"`
		SS  []string `param:"ss"`
	}
	cases := []struct {
		url  string
		want foo
		got  foo
	}{
		{
			"?x=111&y=222&b=true&i=1&s=hello+world&ss=a&ss=b",
			foo{Bar{111, 222}, true, 1, "hello world", []string{"a", "b"}},
			foo{},
		},
	}
	for i, c := range cases {
		httpRequest, _ := http.NewRequest("GET", "/test"+c.url, nil)
		req := restful.NewRequest(httpRequest)

		err := openapi.ReadEntity(req, &c.got)
		require.Nil(t, err, c.url)
		require.Equal(t, c.want, c.got, i)
	}

}
