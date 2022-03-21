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

package logs

import (
	"fmt"

	cliflag "github.com/yubo/golib/cli/flag"
	"github.com/yubo/golib/logs/config"
	"github.com/yubo/golib/logs/registry"
	"github.com/yubo/golib/util/validation/field"
)

func ValidateLoggingConfiguration(c *config.LoggingConfiguration, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if c.Format != DefaultLogFormat {
		// WordSepNormalizeFunc is just a guess. Commands should use it,
		// but we cannot know for sure.
		allFlags := UnsupportedLoggingFlags(cliflag.WordSepNormalizeFunc)
		for _, f := range allFlags {
			if f.DefValue != f.Value.String() {
				errs = append(errs, field.Invalid(fldPath.Child("format"), c.Format, fmt.Sprintf("Non-default format doesn't honor flag: %s", f.Name)))
			}
		}
	}
	_, err := registry.LogRegistry.Get(c.Format)
	if err != nil {
		errs = append(errs, field.Invalid(fldPath.Child("format"), c.Format, "Unsupported log format"))
	}

	// Currently nothing to validate for c.Options.

	return errs
}
