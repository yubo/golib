/*
Copyright 2019 The Kubernetes Authors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/runtime/serializer/protobuf/testdata"
	"github.com/yubo/golib/util"
)

func TestSerializer(t *testing.T) {
	var want, got testdata.User

	writer := &bytes.Buffer{}
	serializer := NewSerializer()

	want.Name = util.String("name")
	want.Age = util.Int32(16)

	err := serializer.Encode(&want, writer)
	assert.NoError(t, err)

	_, err = serializer.Decode(writer.Bytes(), &got)
	assert.NoError(t, err)
	assert.EqualValues(t, &want, &got)
}

//
//func TestCacheableObject(t *testing.T) {
//	encoders := []runtime.Encoder{
//		NewSerializer(),
//		NewRawSerializer(),
//	}
//
//	for _, encoder := range encoders {
//		runtimetesting.CacheableObjectTest(t, encoder)
//	}
//}
//
//func TestRawSerializerEncodeWithAllocator(t *testing.T) {
//}
//
//type testAllocator struct {
//	buf           []byte
//	allocateCount int
//}
//
//func (ta *testAllocator) Allocate(n uint64) []byte {
//	ta.buf = make([]byte, n)
//	ta.allocateCount++
//	return ta.buf
//}
