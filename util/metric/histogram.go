// Copyright 2015 The Prometheus Authors
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

	//lint:ignore SA1019 Need to keep deprecated package for compatibility.

	"context"

	"github.com/uber-go/tally"
)

var (
	DefBuckets = tally.ValueBuckets{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
)

// LinearBuckets creates 'count' buckets, each 'width' wide, where the lowest
// bucket has an upper bound of 'start'. The final +Inf bucket is not counted
// and not included in the returned slice. The returned slice is meant to be
// used for the Buckets field of HistogramOpts.
//
// The function panics if 'count' is zero or negative.
func LinearBuckets(start, width float64, count int) tally.ValueBuckets {
	if count < 1 {
		panic("LinearBuckets needs a positive count")
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start += width
	}
	return tally.ValueBuckets(buckets)
}

// ExponentialBuckets creates 'count' buckets, where the lowest bucket has an
// upper bound of 'start' and each following bucket's upper bound is 'factor'
// times the previous bucket's upper bound. The final +Inf bucket is not counted
// and not included in the returned slice. The returned slice is meant to be
// used for the Buckets field of HistogramOpts.
//
// The function panics if 'count' is 0 or negative, if 'start' is 0 or negative,
// or if 'factor' is less than or equal 1.
func ExponentialBuckets(start, factor float64, count int) tally.ValueBuckets {
	if count < 1 {
		panic("ExponentialBuckets needs a positive count")
	}
	if start <= 0 {
		panic("ExponentialBuckets needs a positive start value")
	}
	if factor <= 1 {
		panic("ExponentialBuckets needs a factor greater than 1")
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start *= factor
	}
	return tally.ValueBuckets(buckets)
}

// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. HTTP request latencies, partitioned by status code and method). Create
// instances with NewHistogramVec.
type HistogramVec struct {
	*metricVec
}

// NewHistogramVec creates a new HistogramVec based on the provided HistogramOpts and
// partitioned by the given label names.
func NewHistogramVec(scope tally.Scope, name string, buckets tally.Buckets, labelNames []string) *HistogramVec {
	desc := NewDesc(scope, name, labelNames, nil)
	return &HistogramVec{
		metricVec: newMetricVec(desc, func(lvs ...string) interface{} {
			return scope.Tagged(MakeTags(desc, lvs)).Histogram(name, buckets)
		}),
	}
}

// GetMetricWithLabelValues returns the Histogram for the given slice of label
// values (same order as the VariableLabels in Desc). If that combination of
// label values is accessed for the first time, a new Histogram is created.
//
// It is possible to call this method without using the returned Histogram to only
// create the new Histogram but leave it at its starting value, a Histogram without
// any observations.
//
// Keeping the Histogram for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the Histogram from the HistogramVec. In that case, the
// Histogram will still exist, but it will not be exported anymore, even if a
// Histogram with the same label values is created later. See also the CounterVec
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
// See also the GaugeVec example.
func (v *HistogramVec) GetMetricWithLabelValues(lvs ...string) (tally.Histogram, error) {
	metric, err := v.getMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(tally.Histogram), err
	}
	return nil, err
}

// GetMetricWith returns the Histogram for the given Labels map (the label names
// must match those of the VariableLabels in Desc). If that label map is
// accessed for the first time, a new Histogram is created. Implications of
// creating a Histogram without using it and keeping the Histogram for later use
// are the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the VariableLabels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *HistogramVec) GetMetricWith(labels Labels) (tally.Histogram, error) {
	metric, err := v.metricVec.getMetricWith(labels)
	if metric != nil {
		return metric.(tally.Histogram), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. Not returning an
// error allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Observe(42.21)
func (v *HistogramVec) WithLabelValues(lvs ...string) tally.Histogram {
	h, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return h
}

// With works as GetMetricWith but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//     myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Observe(42.21)
func (v *HistogramVec) With(labels Labels) tally.Histogram {
	h, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return h
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the HistogramVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *HistogramVec) CurryWith(labels Labels) (*HistogramVec, error) {
	vec, err := v.curryWith(labels)
	if vec != nil {
		return &HistogramVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *HistogramVec) MustCurryWith(labels Labels) *HistogramVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}

// WithContext returns wrapped HistogramVec with context
func (v *HistogramVec) WithContext(ctx context.Context) *HistogramVecWithContext {
	return &HistogramVecWithContext{
		ctx:          ctx,
		HistogramVec: *v,
	}
}

// HistogramVecWithContext is the wrapper of HistogramVec with context.
type HistogramVecWithContext struct {
	HistogramVec
	ctx context.Context
}

// WithLabelValues is the wrapper of HistogramVec.WithLabelValues.
func (vc *HistogramVecWithContext) WithLabelValues(lvs ...string) tally.Histogram {
	return vc.HistogramVec.WithLabelValues(lvs...)
}

// With is the wrapper of HistogramVec.With.
func (vc *HistogramVecWithContext) With(labels map[string]string) tally.Histogram {
	return vc.HistogramVec.With(labels)
}
