package api

import (
	"net/http"
	"net/url"
)

// ParameterCodec defines methods for serializing and deserializing API objects to url.Values and
// performing any necessary conversion. Unlike the normal Codec, query parameters are not self describing
// and the desired version must be specified.
type ParameterCodec interface {
	// DecodeParameters takes the given url.Values in the specified group version and decodes them
	// into the provided object, or returns an error.
	DecodeParameters(parameters *Parameters, into interface{}) error
	// EncodeParameters encodes the provided object as query parameters or returns an error.
	EncodeParameters(obj interface{}) (*Parameters, error)
}

type Parameters struct {
	Header http.Header
	Query  url.Values
	Path   map[string]string
}
