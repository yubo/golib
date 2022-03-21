/*
Copyright 2021 The Kubernetes Authors.

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

package register

import (
	"bytes"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/yubo/golib/api/resource"
	"github.com/yubo/golib/logs"
	"github.com/yubo/golib/logs/config"
	"github.com/yubo/golib/util/validation/field"
)

func TestJSONFlag(t *testing.T) {
	o := logs.NewOptions()
	fs := pflag.NewFlagSet("addflagstest", pflag.ContinueOnError)
	output := bytes.Buffer{}
	o.AddFlags(fs)
	fs.SetOutput(&output)
	fs.PrintDefaults()
	wantSubstring := `Permitted formats: "json", "text".`
	if !assert.Contains(t, output.String(), wantSubstring) {
		t.Errorf("JSON logging format flag is not available. expect to contain %q, got %q", wantSubstring, output.String())
	}
}

func TestJSONFormatRegister(t *testing.T) {
	defaultOptions := config.FormatOptions{
		JSON: config.JSONOptions{
			InfoBufferSize: resource.QuantityValue{
				Quantity: *resource.NewQuantity(0, resource.DecimalSI),
			},
		},
	}
	_ = defaultOptions.JSON.InfoBufferSize.String()
	testcases := []struct {
		name string
		args []string
		want *logs.Options
		errs field.ErrorList
	}{
		{
			name: "JSON log format",
			args: []string{"--logging-format=json"},
			want: &logs.Options{
				Config: config.LoggingConfiguration{
					Format:  logs.JSONLogFormat,
					Options: defaultOptions,
				},
			},
		},
		{
			name: "Unsupported log format",
			args: []string{"--logging-format=test"},
			want: &logs.Options{
				Config: config.LoggingConfiguration{
					Format:  "test",
					Options: defaultOptions,
				},
			},
			errs: field.ErrorList{&field.Error{
				Type:     "FieldValueInvalid",
				Field:    "format",
				BadValue: "test",
				Detail:   "Unsupported log format",
			}},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o := logs.NewOptions()
			fs := pflag.NewFlagSet("addflagstest", pflag.ContinueOnError)
			o.AddFlags(fs)
			fs.Parse(tc.args)
			if !assert.Equal(t, tc.want, o) {
				t.Errorf("Wrong Validate() result for %q. expect %v, got %v", tc.name, tc.want, o)
			}
			errs := o.Validate()
			if !assert.ElementsMatch(t, tc.errs, errs) {
				t.Errorf("Wrong Validate() result for %q.\n expect:\t%+v\n got:\t%+v", tc.name, tc.errs, errs)

			}
		})
	}
}
