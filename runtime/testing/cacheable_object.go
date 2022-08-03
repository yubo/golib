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

package testing

import (
	"github.com/yubo/golib/runtime"
)

type noncacheableTestObject struct{}

// MarshalJSON implements json.Marshaler interface.
func (*noncacheableTestObject) MarshalJSON() ([]byte, error) {
	return []byte("\"json-result\""), nil
}

// Marshal implements proto.Marshaler interface.
func (*noncacheableTestObject) Marshal() ([]byte, error) {
	return []byte("\"proto-result\""), nil
}

// DeepCopyObject implements runtime.Object interface.
func (*noncacheableTestObject) DeepCopyObject() runtime.Object {
	panic("DeepCopy unimplemented for noncacheableTestObject")
}
