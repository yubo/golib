/*
Copyright 2016 The Kubernetes Authors.

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

package flag

import (
	"encoding/json"
	"errors"
	"flag"
	"strings"
)

// +k8s:deepcopy-gen=true
// NamedCertKey is a flag value parsing "certfile,keyfile" and "certfile,keyfile:name,name,name".
type NamedCertKey struct {
	Names    []string
	CertFile string
	KeyFile  string
}

func (in *NamedCertKey) DeepCopyInto(out *NamedCertKey) {
	*out = *in

	if in.Names != nil {
		in, out := &in.Names, &out.Names
		*out = make([]string, len(*in))
		copy(*out, *in)
	}

	return
}

func (in *NamedCertKey) DeepCopy() *NamedCertKey {
	if in == nil {
		return nil
	}
	out := new(NamedCertKey)
	in.DeepCopyInto(out)
	return out
}

func (a *NamedCertKey) UnmarshalJSON(b []byte) error {
	if len(b) == 4 && string(b) == "null" {
		return nil
	}

	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}

	return a.Set(str)
}

func (a NamedCertKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

var _ flag.Value = &NamedCertKey{}

func (nkc *NamedCertKey) String() string {
	s := nkc.CertFile + "," + nkc.KeyFile
	if len(nkc.Names) > 0 {
		s = s + ":" + strings.Join(nkc.Names, ",")
	}
	return s
}

func (nkc *NamedCertKey) Set(value string) error {
	cs := strings.SplitN(value, ":", 2)
	var keycert string
	if len(cs) == 2 {
		var names string
		keycert, names = strings.TrimSpace(cs[0]), strings.TrimSpace(cs[1])
		if names == "" {
			return errors.New("empty names list is not allowed")
		}
		nkc.Names = nil
		for _, name := range strings.Split(names, ",") {
			nkc.Names = append(nkc.Names, strings.TrimSpace(name))
		}
	} else {
		nkc.Names = nil
		keycert = strings.TrimSpace(cs[0])
	}
	cs = strings.Split(keycert, ",")
	if len(cs) != 2 {
		return errors.New("expected comma separated certificate and key file paths")
	}
	nkc.CertFile = strings.TrimSpace(cs[0])
	nkc.KeyFile = strings.TrimSpace(cs[1])
	return nil
}

func (*NamedCertKey) Type() string {
	return "namedCertKey"
}

// NamedCertKeyArray is a flag value parsing NamedCertKeys, each passed with its own
// flag instance (in contrast to comma separated slices).
type NamedCertKeyArray struct {
	Value   *[]NamedCertKey
	changed bool
}

func (a *NamedCertKeyArray) UnmarshalJSON(b []byte) error {
	if len(b) == 4 && string(b) == "null" {
		return nil
	}

	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}

	return a.SetDefault(str)
}

func (a NamedCertKeyArray) MarshalJSON() ([]byte, error) {
	if a.IsZero() {
		// Encode unset/nil objects as JSON's "null".
		return []byte("null"), nil
	}

	return json.Marshal(a.String())
}

// IsZero returns true if the value is nil
func (a *NamedCertKeyArray) IsZero() bool {
	if a == nil || a.Value == nil || len(*a.Value) == 0 {
		return true
	}
	return false
}

var _ flag.Value = &NamedCertKeyArray{}

// NewNamedKeyCertArray creates a new NamedCertKeyArray with the internal value
// pointing to p.
func NewNamedCertKeyArray(p *[]NamedCertKey) *NamedCertKeyArray {
	return &NamedCertKeyArray{
		Value: p,
	}
}

func (a *NamedCertKeyArray) SetDefault(val string) error {
	if err := a.Set(val); err != nil {
		return err
	}
	a.changed = false
	return nil
}

func (a *NamedCertKeyArray) Set(val string) error {
	for _, v := range strings.Split(val, ";") {
		if err := a.set(v); err != nil {
			return err
		}
	}

	return nil
}

func (a *NamedCertKeyArray) set(val string) error {
	nkc := NamedCertKey{}
	err := nkc.Set(val)
	if err != nil {
		return err
	}
	if !a.changed {
		if a.Value == nil {
			a.Value = &[]NamedCertKey{nkc}
		} else {
			*a.Value = []NamedCertKey{nkc}
		}
		a.changed = true
	} else {
		*a.Value = append(*a.Value, nkc)
	}
	return nil
}

func (a *NamedCertKeyArray) Type() string {
	return "namedCertKey"
}

func (a *NamedCertKeyArray) String() string {
	if a.Value == nil {
		return ""
	}

	nkcs := make([]string, 0, len(*a.Value))
	for i := range *a.Value {
		nkcs = append(nkcs, (*a.Value)[i].String())
	}
	return strings.Join(nkcs, ";")
}
