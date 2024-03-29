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

package json_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/api"
	"github.com/yubo/golib/runtime"
	"github.com/yubo/golib/runtime/serializer/json"
)

type testDecodable struct {
	api.TypeMeta `json:",inline"`
	Other        string
	Value        int           `json:"value"`
	Spec         DecodableSpec `json:"spec"`
	Interface    interface{}   `json:"interface"`
}

// DecodableSpec has 15 fields. json-iterator treats struct with more than 10
// fields differently from struct that has less than 10 fields.
type DecodableSpec struct {
	A int `json:"A"`
	B int `json:"B"`
	C int `json:"C"`
	D int `json:"D"`
	E int `json:"E"`
	F int `json:"F"`
	G int `json:"G"`
	H int `json:"h"`
	I int `json:"i"`
	J int `json:"j"`
	K int `json:"k"`
	L int `json:"l"`
	M int `json:"m"`
	N int `json:"n"`
	O int `json:"o"`
}

func TestDecode(t *testing.T) {
	testCases := []struct {
		yaml   bool
		pretty bool
		strict bool

		data []byte
		into runtime.Object

		errFn          func(error) bool
		expectedObject runtime.Object
	}{
		{
			data:           []byte(`{}`),
			into:           &testDecodable{},
			expectedObject: &testDecodable{},
		},
		// accept runtime.Unknown as into and bypass creator
		{
			data: []byte(`{}`),
			into: &runtime.Unknown{},

			//expectedGVK: &schema.GroupVersionKind{},
			expectedObject: &runtime.Unknown{
				Raw:         []byte(`{}`),
				ContentType: runtime.ContentTypeJSON,
			},
		},
		{
			data: []byte(`{"test":"object"}`),
			into: &runtime.Unknown{},

			expectedObject: &runtime.Unknown{
				Raw:         []byte(`{"test":"object"}`),
				ContentType: runtime.ContentTypeJSON,
			},
		},
		{
			data: []byte(`{"test":"object"}`),
			into: &runtime.Unknown{},
			expectedObject: &runtime.Unknown{
				Raw:         []byte(`{"test":"object"}`),
				ContentType: runtime.ContentTypeJSON,
			},
		},

		// unregistered objects can be decoded into directly
		{
			data: []byte(`{"value":1,"Other":"test"}`),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Other: "test",
				Value: 1,
			},
		},
		// registered types get defaulted by the into object kind
		{
			data: []byte(`{"value":1,"Other":"test"}`),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Other: "test",
				Value: 1,
			},
		},
		// Unmarshalling is case-sensitive
		{
			// "VaLue" should have been "value"
			data: []byte(`{"VaLue":1,"Other":"test"}`),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Other: "test",
			},
		},
		// Unmarshalling is case-sensitive for big struct.
		{
			// "b" should have been "B", "I" should have been "i"
			data: []byte(`{"spec": {"A": 1, "b": 2, "h": 3, "I": 4}}`),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Spec: DecodableSpec{A: 1, H: 3},
			},
		},
		// Strict JSON decode into unregistered objects directly.
		{
			data: []byte(`{"value":1,"Other":"test"}`),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Other: "test",
				Value: 1,
			},
			strict: true,
		},
		// Strict YAML decode into unregistered objects directly.
		{
			data: []byte("value: 1\n" +
				"Other: test\n"),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Other: "test",
				Value: 1,
			},
			yaml:   true,
			strict: true,
		},
		// Valid strict JSON decode without GVK.
		{
			data: []byte(`{"value":1234}`),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Value: 1234,
			},
			strict: true,
		},
		{
			data: []byte("value: 1234\n"),
			into: &testDecodable{},
			expectedObject: &testDecodable{
				Value: 1234,
			},
			yaml:   true,
			strict: true,
		},
	}

	for i, test := range testCases {
		var s runtime.Serializer
		if test.yaml {
			s = json.NewSerializerWithOptions(json.SerializerOptions{Yaml: test.yaml, Pretty: false, Strict: test.strict})
		} else {
			s = json.NewSerializerWithOptions(json.SerializerOptions{Yaml: test.yaml, Pretty: test.pretty, Strict: test.strict})
		}
		_, err := s.Decode([]byte(test.data), test.into)
		obj := test.into
		switch {
		case err == nil && test.errFn != nil:
			t.Errorf("%d: failed: %v", i, err)
			continue
		case err != nil && test.errFn == nil:
			t.Errorf("%d: failed: %v", i, err)
			continue
		case err != nil:
			if !test.errFn(err) {
				t.Errorf("%d: failed: %v", i, err)
			}
			if obj != nil {
				t.Errorf("%d: should have returned nil object", i)
			}
			continue
		}

		if test.into != nil && test.into != obj {
			t.Errorf("%d: expected into to be returned: %v", i, obj)
			continue
		}

		assert.Equal(t, test.expectedObject, obj)
	}
}

type mockCreater struct {
	apiVersion string
	kind       string
	err        error
	obj        runtime.Object
}

func (c *mockCreater) New() (runtime.Object, error) {
	return c.obj, c.err
}

type mockTyper struct {
	err error
}

func (t *mockTyper) Recognizes() bool {
	return false
}
