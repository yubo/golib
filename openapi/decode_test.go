package openapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emicklei/go-restful"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/util"
)

func TestDecode(t *testing.T) {
	header0 := make(http.Header)
	header1 := make(http.Header)
	header1.Set("headerValue", "HeaderValue")

	cases := []struct {
		url    string
		body   string
		header http.Header
		want   *Sample
	}{{
		"",
		"{}",
		header0,
		&Sample{},
	}, {
		"",
		"{}",
		header1,
		&Sample{
			HeaderValue: util.String("HeaderValue"),
		},
	}, {
		"?queryValue=QueryValue",
		"{}",
		header0,
		&Sample{
			QueryValue: util.String("QueryValue"),
		},
	}, {
		"",
		`{"dataValueString" : "DataValueString"}`,
		header0,
		&Sample{
			DataValueString: util.String("DataValueString"),
		},
	}, {
		"",
		`{
			"dataValueStruct": {"structValue": "StructValue"}
		}`,
		header0,
		&Sample{
			DataValueStruct: &SampleStruct{
				StructValue: util.String("StructValue"),
			},
		},
	}, {
		"?queryValue=QueryValue",
		`{
			"dataValueString" : "DataValueString" ,
			"dataValueStruct": {"structValue": "StructValue"}
		}`,
		header1,
		&Sample{
			HeaderValue:     util.String("HeaderValue"),
			QueryValue:      util.String("QueryValue"),
			DataValueString: util.String("DataValueString"),
			DataValueStruct: &SampleStruct{
				StructValue: util.String("StructValue"),
			},
		},
	}}

	for i, c := range cases {
		httpRequest, _ := http.NewRequest("GET", c.url, strings.NewReader(c.body))
		httpRequest.Header = c.header
		httpRequest.Header.Set("Content-Type", "application/json")
		request := restful.NewRequest(httpRequest)

		got := &Sample{}
		err := json.Unmarshal([]byte(c.body), got)
		if err != nil {
			t.Fatalf("case-%d ReadEntity failed %v", i, err)
		}
		if err := ReadEntity(request, got); err != nil {
			t.Fatalf("case-%d ReadEntity failed %v", i, err)
		}

		require.Equalf(t, c.want, got, "case-%d", i)
	}
}
