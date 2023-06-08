package api

import (
	"net/http"
	"net/url"
	"reflect"

	"github.com/emicklei/go-restful/v3"
)

func NewParameters() *Parameters {
	return &Parameters{
		Header: make(http.Header),
		Query:  make(url.Values),
		Path:   make(map[string]string),
	}
}

// ParameterCodec defines methods for serializing and deserializing API objects to url.Values and
// performing any necessary conversion. Unlike the normal Codec, query parameters are not self describing
// and the desired version must be specified.
type ParameterCodec interface {
	// DecodeParameters takes the given url.Values in the specified group version and decodes them
	// into the provided object, or returns an error.
	DecodeParameters(parameter *Parameters, into interface{}) error
	// EncodeParameters encodes the provided object as query parameters or returns an error.
	EncodeParameters(obj interface{}) (*Parameters, error)

	RouteBuilderParameters(rb *restful.RouteBuilder, obj interface{})

	ValidateParamType(rt reflect.Type) error
}

type Parameters struct {
	Header http.Header
	Query  url.Values
	Path   map[string]string
}
