package openapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/util"
)

func TestEncode(t *testing.T) {
	header0 := make(http.Header)
	header1 := make(http.Header)
	header1.Set("headerValue", "HeaderValue")

	cases := []struct {
		url        string
		in         *Sample
		wantUrl    string
		wantHeader http.Header
	}{{
		"",
		&Sample{},
		"",
		header0,
	}, {
		"http://example.com/users/{name}",
		&Sample{PathValue: util.String("tom")},
		"http://example.com/users/tom",
		header0,
	}, {
		"",
		&Sample{HeaderValue: util.String("HeaderValue")},
		"",
		header1,
	}, {
		"",
		&Sample{QueryValue: util.String("QueryValue")},
		"?queryValue=QueryValue",
		header0,
	}, {
		"",
		&Sample{DataValueString: util.String("DataValueString")},
		"",
		header0,
	}, {
		"",
		&Sample{DataValueStruct: &SampleStruct{
			StructValue: util.String("StructValue"),
		}},
		"",
		header0,
	}, {
		"http://example.com/users/{name}",
		&Sample{
			PathValue:       util.String("tom"),
			HeaderValue:     util.String("HeaderValue"),
			QueryValue:      util.String("QueryValue"),
			DataValueString: util.String("DataValueString"),
			DataValueStruct: &SampleStruct{
				StructValue: util.String("StructValue"),
			},
		},
		"http://example.com/users/tom?queryValue=QueryValue",
		header1,
	}}

	for i, c := range cases {
		url, data, header, err := NewEncoder().Encode(c.url, c.in)
		if err != nil {
			t.Fatalf("cases-%d Encode failed %#v", i, err)
		}
		require.Equalf(t, c.wantUrl, url, "cases-%d", i)
		require.Equalf(t, data, data, "cases-%d", i)
		require.Equalf(t, c.wantHeader, header, "cases-%d", i)
	}
}

func TestInvokePathVariable(t *testing.T) {
	data := map[string]string{
		"user-name":   "tom",
		"user-id":     "16",
		"api-version": "1",
		"empty":       "",
	}

	cases := []struct {
		in   string
		want string
	}{
		{"{user-name}", "tom"},
		{"/{user-name}", "/tom"},
		{"{user-name}/", "tom/"},
		{"/{empty}/a", "//a"},
		{"/{user-name}/{user-id}/", "/tom/16/"},
		{"http://example.com/api/v{api-version}/user/{user-id}",
			"http://example.com/api/v1/user/16"},
	}

	for i, c := range cases {
		got, err := invokePathVariable(c.in, data)
		require.Emptyf(t, err, "case-%d", i)
		require.Equalf(t, c.want, got, "case-%d", i)
	}
}
