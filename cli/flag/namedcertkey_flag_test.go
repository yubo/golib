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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/configer"
)

func TestNamedCertKeyArrayConfig(t *testing.T) {
	type Foo struct {
		Certs NamedCertKeyArray `json:"certs" flag:"tls-sni-cert-key" default:"foo.crt,foo.key:abc; bar.crt,bar.key"`
	}
	tests := []struct {
		args        []string
		fileContent string
		def         *[]NamedCertKey
		expected    *[]NamedCertKey
	}{
		{
			expected: &[]NamedCertKey{{
				Names:    []string{"abc"},
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
			}, {
				KeyFile:  "bar.key",
				CertFile: "bar.crt",
			}},
		},
		{
			def: &[]NamedCertKey{{
				KeyFile:  "bar.key",
				CertFile: "bar.crt",
			}},
			expected: &[]NamedCertKey{{
				KeyFile:  "bar.key",
				CertFile: "bar.crt",
			}},
		},
		{
			args: []string{"foo.crt,foo.key"},
			expected: &[]NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
			}},
		},
	}

	for i, test := range tests {
		configer.Reset()
		fs := pflag.NewFlagSet("testNamedCertKeyArray", pflag.ContinueOnError)
		configer.SetOptions(true, false, 5, fs)

		{
			err := configer.RegisterConfigFields(fs, "",
				&Foo{Certs: NamedCertKeyArray{value: test.def}})
			assert.NoError(t, err, i)
		}

		args := []string{}
		for _, a := range test.args {
			args = append(args, fmt.Sprintf("--tls-sni-cert-key=%s", a))
		}
		err := fs.Parse(args)
		assert.NoError(t, err, i)

		cf, err := configer.NewConfiger(configer.WithDefaultYaml("", test.fileContent))
		assert.NoError(t, err, i)

		// debug
		t.Logf("%v", cf.GetRaw(""))

		got := Foo{}
		cf.Read("", &got)
		assert.Equal(t, test.expected, got.Certs.value, i)
	}
}

func TestNamedCertKeyArrayFlag(t *testing.T) {
	tests := []struct {
		args       []string
		def        []NamedCertKey
		expected   []NamedCertKey
		parseError string
	}{
		{
			args:     []string{},
			expected: nil,
		},
		{
			args: []string{"foo.crt,foo.key"},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
			}},
		},
		{
			args: []string{"  foo.crt , foo.key    "},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
			}},
		},
		{
			args: []string{"foo.crt,foo.key:abc"},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
				Names:    []string{"abc"},
			}},
		},
		{
			args: []string{"foo.crt,foo.key: abc  "},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
				Names:    []string{"abc"},
			}},
		},
		{
			args:       []string{"foo.crt,foo.key:"},
			parseError: "empty names list is not allowed",
		},
		{
			args:       []string{""},
			parseError: "expected comma separated certificate and key file paths",
		},
		{
			args:       []string{"   "},
			parseError: "expected comma separated certificate and key file paths",
		},
		{
			args:       []string{"a,b,c"},
			parseError: "expected comma separated certificate and key file paths",
		},
		{
			args: []string{"foo.crt,foo.key:abc,def,ghi"},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
				Names:    []string{"abc", "def", "ghi"},
			}},
		},
		{
			args: []string{"foo.crt,foo.key:*.*.*"},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
				Names:    []string{"*.*.*"},
			}},
		},
		{
			args: []string{"foo.crt,foo.key", "bar.crt,bar.key"},
			expected: []NamedCertKey{{
				KeyFile:  "foo.key",
				CertFile: "foo.crt",
			}, {
				KeyFile:  "bar.key",
				CertFile: "bar.crt",
			}},
		},
	}
	for i, test := range tests {
		fs := pflag.NewFlagSet("testNamedCertKeyArray", pflag.ContinueOnError)
		var nkcs []NamedCertKey
		nkcs = append(nkcs, test.def...)

		fs.Var(NewNamedCertKeyArray(&nkcs), "tls-sni-cert-key", "usage")

		args := []string{}
		for _, a := range test.args {
			args = append(args, fmt.Sprintf("--tls-sni-cert-key=%s", a))
		}

		err := fs.Parse(args)
		if test.parseError != "" {
			if err == nil {
				t.Errorf("%d: expected error %q, got nil", i, test.parseError)
			} else if !strings.Contains(err.Error(), test.parseError) {
				t.Errorf("%d: expected error %q, got %q", i, test.parseError, err)
			}
		} else if err != nil {
			t.Errorf("%d: expected nil error, got %v", i, err)
		}
		if !reflect.DeepEqual(nkcs, test.expected) {
			t.Errorf("%d: expected %+v, got %+v", i, test.expected, nkcs)
		}
	}
}
