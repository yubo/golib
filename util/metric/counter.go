// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric

import (
	"context"

	"github.com/uber-go/tally"
)

// CounterVec is a Collector that bundles a set of Counters that all share the
// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. number of HTTP requests, partitioned by response code and
// method). Create instances with NewCounterVec.
type CounterVec struct {
	*metricVec
}

// NewCounterVec creates a new CounterVec based on the provided CounterOpts and
// partitioned by the given label names.
func NewCounterVec(scope tally.Scope, name string, labelNames []string) *CounterVec {
	desc := NewDesc(scope, name, labelNames, nil)
	return &CounterVec{
		metricVec: newMetricVec(desc, func(lvs ...string) interface{} {
			if len(lvs) != len(desc.variableLabels) {
				panic(makeInconsistentCardinalityError(desc.fqName, desc.variableLabels, lvs))
			}

			return scope.Tagged(MakeTags(desc, lvs)).Counter(name)
		}),
	}
}

// GetMetricWithLabelValues returns the Counter for the given slice of label
// values (same order as the variable labels in Desc). If that combination of
// label values is accessed for the first time, a new Counter is created.
//
// It is possible to call this method without using the returned Counter to only
// create the new Counter but leave it at its starting value 0. See also the
// SummaryVec example.
//
// Keeping the Counter for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the Counter from the CounterVec. In that case,
// the Counter will still exist, but it will not be exported anymore, even if a
// Counter with the same label values is created later.
//
// An error is returned if the number of label values is not the same as the
// number of variable labels in Desc (minus any curried labels).
//
// Note that for more than one label value, this method is prone to mistakes
// caused by an incorrect order of arguments. Consider GetMetricWith(Labels) as
// an alternative to avoid that type of mistake. For higher label numbers, the
// latter has a much more readable (albeit more verbose) syntax, but it comes
// with a performance overhead (for creating and processing the Labels map).
// See also the GaugeVec example.
func (v *CounterVec) GetMetricWithLabelValues(lvs ...string) (tally.Counter, error) {
	metric, err := v.getMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(tally.Counter), err
	}
	return nil, err
}

// GetMetricWith returns the Counter for the given Labels map (the label names
// must match those of the variable labels in Desc). If that label map is
// accessed for the first time, a new Counter is created. Implications of
// creating a Counter without using it and keeping the Counter for later use are
// the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the variable labels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *CounterVec) GetMetricWith(labels Labels) (tally.Counter, error) {
	metric, err := v.getMetricWith(labels)
	if metric != nil {
		return metric.(tally.Counter), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. Not returning an
// error allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Add(42)
func (v *CounterVec) WithLabelValues(lvs ...string) tally.Counter {
	c, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return c
}

// With works as GetMetricWith, but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//     myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Add(42)
func (v *CounterVec) With(labels Labels) tally.Counter {
	c, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return c
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the CounterVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *CounterVec) CurryWith(labels Labels) (*CounterVec, error) {
	vec, err := v.curryWith(labels)
	if vec != nil {
		return &CounterVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *CounterVec) MustCurryWith(labels Labels) *CounterVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}

// WithContext returns wrapped CounterVec with context
func (v *CounterVec) WithContext(ctx context.Context) *CounterVecWithContext {
	return &CounterVecWithContext{
		ctx:        ctx,
		CounterVec: *v,
	}
}

// CounterVecWithContext is the wrapper of CounterVec with context.
type CounterVecWithContext struct {
	CounterVec
	ctx context.Context
}

// WithLabelValues is the wrapper of CounterVec.WithLabelValues.
func (vc *CounterVecWithContext) WithLabelValues(lvs ...string) tally.Counter {
	return vc.CounterVec.WithLabelValues(lvs...)
}

// With is the wrapper of CounterVec.With.
func (vc *CounterVecWithContext) With(labels map[string]string) tally.Counter {
	return vc.CounterVec.With(labels)
}
