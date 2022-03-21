/*
Copyright 2020 The Kubernetes Authors.

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
	"github.com/spf13/pflag"
	"github.com/yubo/golib/api/resource"
	"github.com/yubo/golib/logs/sanitization"

	"k8s.io/klog/v2"

	"github.com/yubo/golib/logs/config"
	"github.com/yubo/golib/logs/registry"
)

// Options has klog format parameters
type Options struct {
	Config config.LoggingConfiguration
}

// RecommendedLoggingConfiguration defaults logging configuration.
// This will set the recommended default
// values, but they may be subject to change between API versions. This function
// is intentionally not registered in the scheme as a "normal" `SetDefaults_Foo`
// function to allow consumers of this type to set whatever defaults for their
// embedded configs. Forcing consumers to use these defaults would be problematic
// as defaulting in the scheme is done as part of the conversion, and there would
// be no easy way to opt-out. Instead, if you want to use this defaulting method
// run it in your wrapper struct of this type in its `SetDefaults_` method.
func RecommendedLoggingConfiguration(obj *config.LoggingConfiguration) {
	if obj.Format == "" {
		obj.Format = "text"
	}
	var empty resource.QuantityValue
	if obj.Options.JSON.InfoBufferSize == empty {
		obj.Options.JSON.InfoBufferSize = resource.QuantityValue{
			// This is similar, but not quite the same as a default
			// constructed instance.
			Quantity: *resource.NewQuantity(0, resource.DecimalSI),
		}
		// This sets the unexported Quantity.s which will be compared
		// by reflect.DeepEqual in some tests.
		_ = obj.Options.JSON.InfoBufferSize.String()
	}
}

// NewOptions return new klog options
func NewOptions() *Options {
	o := &Options{}
	RecommendedLoggingConfiguration(&o.Config)

	return o
}

// Validate verifies if any unsupported flag is set
// for non-default logging format
func (o *Options) Validate() []error {
	errs := ValidateLoggingConfiguration(&o.Config, nil)
	if len(errs) != 0 {
		return errs.ToAggregate().Errors()
	}
	return nil
}

// AddFlags add logging-format flag
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	BindLoggingFlags(&o.Config, fs)
}

// Apply set klog logger from LogFormat type
func (o *Options) Apply() {
	// if log format not exists, use nil loggr
	factory, _ := registry.LogRegistry.Get(o.Config.Format)
	if factory == nil {
		klog.ClearLogger()
	} else {
		log, flush := factory.Create(o.Config.Options)
		klog.SetLogger(log)
		logrFlush = flush
	}
	if o.Config.Sanitization {
		klog.SetLogFilter(&sanitization.SanitizingFilter{})
	}
}
