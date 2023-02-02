/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package versioning

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/yubo/golib/runtime"
	"k8s.io/klog/v2"
)

// NewDefaultingCodecForScheme is a convenience method for callers that are using a scheme.
func NewDefaultingCodecForScheme(encoder runtime.Encoder, decoder runtime.Decoder) runtime.Codec {
	return NewCodec(encoder, decoder, nil)
}

// NewCodec takes objects in their internal versions and converts them to external versions before
// serializing them. It assumes the serializer provided to it only deals with external versions.
// This class is also a serializer, but is generally used with a specific version.
func NewCodec(encoder runtime.Encoder, decoder runtime.Decoder, defaulter runtime.ObjectDefaulter) runtime.Codec {
	internal := &codec{
		encoder:    encoder,
		decoder:    decoder,
		defaulter:  defaulter,
		identifier: identifier(encoder),
	}
	return internal
}

type codec struct {
	encoder    runtime.Encoder
	decoder    runtime.Decoder
	defaulter  runtime.ObjectDefaulter
	identifier runtime.Identifier
}

var _ runtime.Encoder = &codec{}

var identifiersMap sync.Map

type codecIdentifier struct {
	Encoder string `json:"encoder,omitempty"`
	Name    string `json:"name,omitempty"`
}

// identifier computes Identifier of Encoder based on codec parameters.
func identifier(encoder runtime.Encoder) runtime.Identifier {
	result := codecIdentifier{
		Name: "versioning",
	}

	if encoder != nil {
		result.Encoder = string(encoder.Identifier())
	}
	if id, ok := identifiersMap.Load(result); ok {
		return id.(runtime.Identifier)
	}
	identifier, err := json.Marshal(result)
	if err != nil {
		klog.Fatalf("Failed marshaling identifier for codec: %v", err)
	}
	identifiersMap.Store(result, runtime.Identifier(identifier))
	return runtime.Identifier(identifier)
}

// Decode attempts a decode of the object, then tries to convert it to the internal version. If into is provided and the decoding is
// successful, the returned runtime.Object will be the value passed as into. Note that this may bypass conversion if you pass an
// into that matches the serialized version.
func (c *codec) Decode(data []byte, into runtime.Object) (runtime.Object, error) {
	out, err := c.decoder.Decode(data, into)
	if err != nil {
		return nil, err
	}
	obj := into

	if d, ok := obj.(runtime.NestedObjectDecoder); ok {
		if err := d.DecodeNestedObjects(c.decoder); err != nil {
			return nil, err
		}
	}

	// perform defaulting if requested
	if c.defaulter != nil {
		c.defaulter.Default(obj)
	}

	return out, err
}

// Encode ensures the provided object is output in the appropriate group and version, invoking
// conversion if necessary. Unversioned objects (according to the ObjectTyper) are output as is.
func (c *codec) Encode(obj runtime.Object, w io.Writer) error {
	//switch obj := obj.(type) {
	//case *runtime.Unknown:
	//	return c.encoder.Encode(obj, w)
	//}

	if e, ok := obj.(runtime.NestedObjectEncoder); ok {
		if err := e.EncodeNestedObjects(c.encoder); err != nil {
			return err
		}
	}

	// Conversion is responsible for setting the proper group, version, and kind onto the outgoing object
	return c.encoder.Encode(obj, w)
}

// Identifier implements runtime.Encoder interface.
func (c *codec) Identifier() runtime.Identifier {
	return c.identifier
}
