/*
Copyright 2015 The Kubernetes Authors.

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

package protobuf

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/yubo/golib/api"
	"github.com/yubo/golib/runtime"
	"github.com/yubo/golib/runtime/serializer"
	"github.com/yubo/golib/util/framer"
	"k8s.io/klog/v2"
)

var (
	ContentTypeProtobuf = "application/x-protobuf" // Accept or Content-Type used in Consumes() and/or Produces()
	// protoEncodingPrefix serves as a magic number for an encoded protobuf message on this serializer. All
	// proto messages serialized by this schema will be preceded by the bytes 0x6b 0x38 0x73, with the fourth
	// byte being reserved for the encoding style. The only encoding style defined is 0x00, which means that
	// the rest of the byte stream is a message of type k8s.io.kubernetes.pkg.runtime.Unknown (proto2).
	//
	// See k8s.io/apimachinery/pkg/runtime/generated.proto for details of the runtime.Unknown message.
	//
	// This encoding scheme is experimental, and is subject to change at any time.
	protoEncodingPrefix = []byte{0x6b, 0x38, 0x73, 0x00}
)

func WithSerializer(options *serializer.CodecFactoryOptions) {
	protoSerializer := NewSerializer()
	options.Serializers = append(options.Serializers, serializer.SerializerType{
		AcceptContentTypes: []string{ContentTypeProtobuf},
		ContentType:        ContentTypeProtobuf,
		FileExtensions:     []string{"pb"},
		Serializer:         protoSerializer,
		Framer:             LengthDelimitedFramer,
		StreamSerializer:   protoSerializer,
	})
}

type errNotMarshalable struct {
	t reflect.Type
}

func (e errNotMarshalable) Error() string {
	return fmt.Sprintf("object %v does not implement the protobuf marshalling interface and cannot be encoded to a protobuf message", e.t)
}

func (e errNotMarshalable) Status() api.Status {
	return api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusNotAcceptable,
		Reason:  api.StatusReason("NotAcceptable"),
		Message: e.Error(),
	}
}

// IsNotMarshalable checks the type of error, returns a boolean true if error is not nil and not marshalable false otherwise
func IsNotMarshalable(err error) bool {
	_, ok := err.(errNotMarshalable)
	return err != nil && ok
}

// NewSerializer creates a Protobuf serializer that handles encoding versioned objects into the proper wire form. If a typer
// is passed, the encoded object will have group, version, and kind fields set. If typer is nil, the objects will be written
// as-is (any type info passed with the object will be used).
func NewSerializer() *Serializer {
	return &Serializer{
		prefix: protoEncodingPrefix,
	}
}

// Serializer handles encoding versioned objects into the proper wire form
type Serializer struct {
	prefix []byte
}

var _ runtime.Serializer = &Serializer{}

const serializerIdentifier runtime.Identifier = "protobuf"

// Decode attempts to convert the provided data into a protobuf message, extract the stored schema kind, apply the provided default
// gvk, and then load that data into an object matching the desired schema kind or the provided into. If into is *runtime.Unknown,
// the raw data will be extracted and no decoding will be performed. If into is not registered with the typer, then the object will
// be straight decoded using normal protobuf unmarshalling (the MarshalTo interface). If into is provided and the original data is
// not fully qualified with kind/version/group, the type of the into will be used to alter the returned gvk. On success or most
// errors, the method will return the calculated schema kind.
func (s *Serializer) Decode(originalData []byte, into runtime.Object) (runtime.Object, error) {
	prefixLen := len(s.prefix)
	switch {
	case len(originalData) == 0:
		// TODO: treat like decoding {} from JSON with defaulting
		return nil, fmt.Errorf("empty data")
	case len(originalData) < prefixLen || !bytes.Equal(s.prefix, originalData[:prefixLen]):
		return nil, fmt.Errorf("provided data does not appear to be a protobuf message, expected prefix %v", s.prefix)
	case len(originalData) == prefixLen:
		// TODO: treat like decoding {} from JSON with defaulting
		return nil, fmt.Errorf("empty body")
	}

	data := originalData[prefixLen:]

	return unmarshalToObject(into, data)
}

// EncodeWithAllocator writes an object to the provided writer.
// In addition, it allows for providing a memory allocator for efficient memory usage during object serialization.
func (s *Serializer) EncodeWithAllocator(obj runtime.Object, w io.Writer, memAlloc runtime.MemoryAllocator) error {
	return s.encode(obj, w, memAlloc)
}

// Encode serializes the provided object to the given writer.
func (s *Serializer) Encode(obj runtime.Object, w io.Writer) error {
	return s.encode(obj, w, &runtime.SimpleAllocator{})
}

func (s *Serializer) encode(obj runtime.Object, w io.Writer, memAlloc runtime.MemoryAllocator) error {
	if co, ok := obj.(runtime.CacheableObject); ok {
		return co.CacheEncode(s.Identifier(), func(obj runtime.Object, w io.Writer) error { return s.doEncode(obj, w, memAlloc) }, w)
	}
	return s.doEncode(obj, w, memAlloc)
}

func (s *Serializer) doEncode(obj runtime.Object, w io.Writer, memAlloc runtime.MemoryAllocator) error {
	if memAlloc == nil {
		klog.Error("a mandatory memory allocator wasn't provided, this might have a negative impact on performance, check invocations of EncodeWithAllocator method, falling back on runtime.SimpleAllocator")
		memAlloc = &runtime.SimpleAllocator{}
	}
	prefixSize := uint64(len(s.prefix))

	switch t := obj.(type) {
	case bufferedMarshaller:
		// this path performs a single allocation during write only when the Allocator wasn't provided
		// it also requires the caller to implement the more efficient Size and MarshalToSizedBuffer methods
		encodedSize := uint64(t.Size())
		data := memAlloc.Allocate(prefixSize + encodedSize)

		i, err := t.MarshalTo(data[prefixSize:])
		if err != nil {
			return err
		}

		copy(data, s.prefix)

		_, err = w.Write(data[:prefixSize+uint64(i)])
		return err

	case proto.Marshaler:
		// this path performs extra allocations
		data, err := t.Marshal()
		if err != nil {
			return err
		}
		_, err = w.Write(append(s.prefix, data...))
		return err

	default:
		// TODO: marshal with a different content type and serializer (JSON for third party objects)
		return errNotMarshalable{reflect.TypeOf(obj)}
	}
}

// Identifier implements runtime.Encoder interface.
func (s *Serializer) Identifier() runtime.Identifier {
	return serializerIdentifier
}

// RecognizesData implements the RecognizingDecoder interface.
func (s *Serializer) RecognizesData(data []byte) (bool, bool, error) {
	return bytes.HasPrefix(data, s.prefix), false, nil
}

// copyKindDefaults defaults dst to the value in src if dst does not have a value set.
//func copyKindDefaults(dst, src *schema.GroupVersionKind) {
//	if src == nil {
//		return
//	}
//	// apply kind and version defaulting from provided default
//	if len(dst.Kind) == 0 {
//		dst.Kind = src.Kind
//	}
//	if len(dst.Version) == 0 && len(src.Version) > 0 {
//		dst.Group = src.Group
//		dst.Version = src.Version
//	}
//}

// bufferedMarshaller describes a more efficient marshalling interface that can avoid allocating multiple
// byte buffers by pre-calculating the size of the final buffer needed.
type bufferedMarshaller interface {
	proto.Sizer
	runtime.ProtobufMarshaller
}

// Like bufferedMarshaller, but is able to marshal backwards, which is more efficient since it doesn't call Size() as frequently.
type bufferedReverseMarshaller interface {
	proto.Sizer
	runtime.ProtobufReverseMarshaller
}

// NewRawSerializer creates a Protobuf serializer that handles encoding versioned objects into the proper wire form. If typer
// is not nil, the object has the group, version, and kind fields set. This serializer does not provide type information for the
// encoded object, and thus is not self describing (callers must know what type is being described in order to decode).
//
// This encoding scheme is experimental, and is subject to change at any time.
func NewRawSerializer() *RawSerializer {
	return &RawSerializer{}
}

// RawSerializer encodes and decodes objects without adding a runtime.Unknown wrapper (objects are encoded without identifying
// type).
type RawSerializer struct{}

var _ runtime.Serializer = &RawSerializer{}

const rawSerializerIdentifier runtime.Identifier = "raw-protobuf"

// Decode attempts to convert the provided data into a protobuf message, extract the stored schema kind, apply the provided default
// gvk, and then load that data into an object matching the desired schema kind or the provided into. If into is *runtime.Unknown,
// the raw data will be extracted and no decoding will be performed. If into is not registered with the typer, then the object will
// be straight decoded using normal protobuf unmarshalling (the MarshalTo interface). If into is provided and the original data is
// not fully qualified with kind/version/group, the type of the into will be used to alter the returned gvk. On success or most
// errors, the method will return the calculated schema kind.
func (s *RawSerializer) Decode(originalData []byte, into runtime.Object) (runtime.Object, error) {
	if into == nil {
		return nil, fmt.Errorf("this serializer requires an object to decode into: %#v", s)
	}

	if len(originalData) == 0 {
		// TODO: treat like decoding {} from JSON with defaulting
		return nil, fmt.Errorf("empty data")
	}
	data := originalData

	//actual := &schema.GroupVersionKind{}
	//copyKindDefaults(actual, gvk)

	return unmarshalToObject(into, data)
}

// unmarshalToObject is the common code between decode in the raw and normal serializer.
func unmarshalToObject(obj runtime.Object, data []byte) (runtime.Object, error) {
	// use the target if necessary
	pb, ok := obj.(proto.Message)
	if !ok {
		return nil, errNotMarshalable{reflect.TypeOf(obj)}
	}
	if err := proto.Unmarshal(data, pb); err != nil {
		return nil, err
	}
	return obj, nil
}

// Encode serializes the provided object to the given writer. Overrides is ignored.
func (s *RawSerializer) Encode(obj runtime.Object, w io.Writer) error {
	return s.encode(obj, w, &runtime.SimpleAllocator{})
}

// EncodeWithAllocator writes an object to the provided writer.
// In addition, it allows for providing a memory allocator for efficient memory usage during object serialization.
func (s *RawSerializer) EncodeWithAllocator(obj runtime.Object, w io.Writer, memAlloc runtime.MemoryAllocator) error {
	return s.encode(obj, w, memAlloc)
}

func (s *RawSerializer) encode(obj runtime.Object, w io.Writer, memAlloc runtime.MemoryAllocator) error {
	if co, ok := obj.(runtime.CacheableObject); ok {
		return co.CacheEncode(s.Identifier(), func(obj runtime.Object, w io.Writer) error { return s.doEncode(obj, w, memAlloc) }, w)
	}
	return s.doEncode(obj, w, memAlloc)
}

func (s *RawSerializer) doEncode(obj runtime.Object, w io.Writer, memAlloc runtime.MemoryAllocator) error {
	if memAlloc == nil {
		klog.Error("a mandatory memory allocator wasn't provided, this might have a negative impact on performance, check invocations of EncodeWithAllocator method, falling back on runtime.SimpleAllocator")
		memAlloc = &runtime.SimpleAllocator{}
	}
	switch t := obj.(type) {
	case bufferedReverseMarshaller:
		// this path performs a single allocation during write only when the Allocator wasn't provided
		// it also requires the caller to implement the more efficient Size and MarshalToSizedBuffer methods
		encodedSize := uint64(t.Size())
		data := memAlloc.Allocate(encodedSize)

		n, err := t.MarshalToSizedBuffer(data)
		if err != nil {
			return err
		}
		_, err = w.Write(data[:n])
		return err

	case bufferedMarshaller:
		// this path performs a single allocation during write only when the Allocator wasn't provided
		// it also requires the caller to implement the more efficient Size and MarshalTo methods
		encodedSize := uint64(t.Size())
		data := memAlloc.Allocate(encodedSize)

		n, err := t.MarshalTo(data)
		if err != nil {
			return err
		}
		_, err = w.Write(data[:n])
		return err

	case proto.Marshaler:
		// this path performs extra allocations
		data, err := t.Marshal()
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err

	default:
		return errNotMarshalable{reflect.TypeOf(obj)}
	}
}

// Identifier implements runtime.Encoder interface.
func (s *RawSerializer) Identifier() runtime.Identifier {
	return rawSerializerIdentifier
}

// LengthDelimitedFramer is exported variable of type lengthDelimitedFramer
var LengthDelimitedFramer = lengthDelimitedFramer{}

// Provides length delimited frame reader and writer methods
type lengthDelimitedFramer struct{}

// NewFrameWriter implements stream framing for this serializer
func (lengthDelimitedFramer) NewFrameWriter(w io.Writer) io.Writer {
	return framer.NewLengthDelimitedFrameWriter(w)
}

// NewFrameReader implements stream framing for this serializer
func (lengthDelimitedFramer) NewFrameReader(r io.ReadCloser) io.ReadCloser {
	return framer.NewLengthDelimitedFrameReader(r)
}
