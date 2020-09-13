package openapi

import (
	"testing"

	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

type SampleStruct struct {
	StructValue *string `json:"structValue"`
	foo         *string
	bar         string
}

type Sample struct {
	PathValue       *string       `param:"path" name:"name"`
	HeaderValue     *string       `param:"header" name:"headerValue"`
	QueryValue      *string       `param:"query" name:"queryValue"`
	DataValueString *string       `param:"data" name:"dataValueString"`
	DataValueStruct *SampleStruct `param:"data" name:"dataValueStruct"`
	foo             *string
	bar             string
}

func init() {
	var level klog.Level
	level.Set("20")
}

func TestRequest(t *testing.T) {
	opt := &RequestOption{
		Url:    "http://example.com/users/{name}",
		Method: "GET",
		Input: &Sample{
			PathValue:       util.String("tom"),
			HeaderValue:     util.String("HeaderValue"),
			QueryValue:      util.String("QueryValue"),
			DataValueString: util.String("DataValueString"),
			DataValueStruct: &SampleStruct{
				StructValue: util.String("StructValue"),
			},
		},
	}

	if _, err := NewRequest(opt); err != nil {
		t.Fatal(err)
	}
}

/*
func TestJson(t *testing.T) {
	json.Unmarshal()
	json.Marshal()
}
*/
