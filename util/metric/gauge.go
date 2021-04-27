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

// GaugeVec is a Collector that bundles a set of Gauges that all share the same
// Desc, but have different values for their variable labels. This is used if
// you want to count the same thing partitioned by various dimensions
// (e.g. number of operations queued, partitioned by user and operation
// type). Create instances with NewGaugeVec.
type GaugeVec struct {
	*metricVec
}

// NewGaugeVec creates a new GaugeVec based on the provided GaugeOpts and
// partitioned by the given label names.
func NewGaugeVec(scope tally.Scope, name string, labelNames []string) *GaugeVec {
	desc := NewDesc(scope, name, labelNames, nil)
	return &GaugeVec{
		metricVec: newMetricVec(desc, func(lvs ...string) interface{} {
			if len(lvs) != len(desc.variableLabels) {
				panic(makeInconsistentCardinalityError(desc.fqName, desc.variableLabels, lvs))
			}
			return scope.Tagged(MakeTags(desc, lvs)).Gauge(name)
		}),
	}
}

// GetMetricWithLabelValues returns the Gauge for the given slice of label
// values (same order as the VariableLabels in Desc). If that combination of
// label values is accessed for the first time, a new Gauge is created.
//
// It is possible to call this method without using the returned Gauge to only
// create the new Gauge but leave it at its starting value 0. See also the
// SummaryVec example.
//
// Keeping the Gauge for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the Gauge from the GaugeVec. In that case, the
// Gauge will still exist, but it will not be exported anymore, even if a
// Gauge with the same label values is created later. See also the CounterVec
// example.
//
// An error is returned if the number of label values is not the same as the
// number of VariableLabels in Desc (minus any curried labels).
//
// Note that for more than one label value, this method is prone to mistakes
// caused by an incorrect order of arguments. Consider GetMetricWith(Labels) as
// an alternative to avoid that type of mistake. For higher label numbers, the
// latter has a much more readable (albeit more verbose) syntax, but it comes
// with a performance overhead (for creating and processing the Labels map).
func (v *GaugeVec) GetMetricWithLabelValues(lvs ...string) (tally.Gauge, error) {
	metric, err := v.getMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(tally.Gauge), err
	}
	return nil, err
}

// GetMetricWith returns the Gauge for the given Labels map (the label names
// must match those of the VariableLabels in Desc). If that label map is
// accessed for the first time, a new Gauge is created. Implications of
// creating a Gauge without using it and keeping the Gauge for later use are
// the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the VariableLabels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *GaugeVec) GetMetricWith(labels Labels) (tally.Gauge, error) {
	metric, err := v.getMetricWith(labels)
	if metric != nil {
		return metric.(tally.Gauge), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. Not returning an
// error allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Add(42)
func (v *GaugeVec) WithLabelValues(lvs ...string) tally.Gauge {
	g, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return g
}

// With works as GetMetricWith, but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//     myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Add(42)
func (v *GaugeVec) With(labels Labels) tally.Gauge {
	g, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return g
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the GaugeVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *GaugeVec) CurryWith(labels Labels) (*GaugeVec, error) {
	vec, err := v.curryWith(labels)
	if vec != nil {
		return &GaugeVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *GaugeVec) MustCurryWith(labels Labels) *GaugeVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}

// WithContext returns wrapped GaugeVec with context
func (v *GaugeVec) WithContext(ctx context.Context) *GaugeVecWithContext {
	return &GaugeVecWithContext{
		ctx:      ctx,
		GaugeVec: *v,
	}
}

// GaugeVecWithContext is the wrapper of GaugeVec with context.
type GaugeVecWithContext struct {
	GaugeVec
	ctx context.Context
}

// WithLabelValues is the wrapper of GaugeVec.WithLabelValues.
func (vc *GaugeVecWithContext) WithLabelValues(lvs ...string) tally.Gauge {
	return vc.GaugeVec.WithLabelValues(lvs...)
}

// With is the wrapper of GaugeVec.With.
func (vc *GaugeVecWithContext) With(labels map[string]string) tally.Gauge {
	return vc.GaugeVec.With(labels)
}
