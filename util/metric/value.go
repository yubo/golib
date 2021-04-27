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
	"fmt"
	"sort"
	"time"
	"unicode/utf8"

	//lint:ignore SA1019 Need to keep deprecated package for compatibility.
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	dto "github.com/prometheus/client_model/go"
)

// ValueType is an enumeration of metric types that represent a simple value.
type ValueType int

// Possible values for the ValueType enum. Use UntypedValue to mark a metric
// with an unknown type.
const (
	_ ValueType = iota
	CounterValue
	GaugeValue
	UntypedValue
)

// MakeLabelPairs is a helper function to create protobuf LabelPairs from the
// variable and constant labels in the provided Desc. The values for the
// variable labels are defined by the labelValues slice, which must be in the
// same order as the corresponding variable labels in the Desc.
//
// This function is only needed for custom Metric implementations. See MetricVec
// example.
func MakeLabelPairs(desc *Desc, labelValues []string) []*dto.LabelPair {
	totalLen := len(desc.variableLabels) + len(desc.constLabelPairs)
	if totalLen == 0 {
		// Super fast path.
		return nil
	}
	if len(desc.variableLabels) == 0 {
		// Moderately fast path.
		return desc.constLabelPairs
	}
	labelPairs := make([]*dto.LabelPair, 0, totalLen)
	for i, n := range desc.variableLabels {
		labelPairs = append(labelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(labelValues[i]),
		})
	}
	labelPairs = append(labelPairs, desc.constLabelPairs...)
	sort.Sort(labelPairSorter(labelPairs))
	return labelPairs
}

func MakeTags(desc *Desc, labelValues []string) map[string]string {
	totalLen := len(desc.variableLabels) + len(desc.constLabelPairs)
	if totalLen == 0 {
		// Super fast path.
		return nil
	}

	tags := make(map[string]string)

	for i, n := range desc.variableLabels {
		tags[n] = labelValues[i]
	}
	for _, v := range desc.constLabelPairs {
		tags[*v.Name] = *v.Value
	}

	return tags
}

// ExemplarMaxRunes is the max total number of runes allowed in exemplar labels.
const ExemplarMaxRunes = 64

// newExemplar creates a new dto.Exemplar from the provided values. An error is
// returned if any of the label names or values are invalid or if the total
// number of runes in the label names and values exceeds ExemplarMaxRunes.
func newExemplar(value float64, ts time.Time, l Labels) (*dto.Exemplar, error) {
	e := &dto.Exemplar{}
	e.Value = proto.Float64(value)
	tsProto, err := ptypes.TimestampProto(ts)
	if err != nil {
		return nil, err
	}
	e.Timestamp = tsProto
	labelPairs := make([]*dto.LabelPair, 0, len(l))
	var runes int
	for name, value := range l {
		if !checkLabelName(name) {
			return nil, fmt.Errorf("exemplar label name %q is invalid", name)
		}
		runes += utf8.RuneCountInString(name)
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("exemplar label value %q is not valid UTF-8", value)
		}
		runes += utf8.RuneCountInString(value)
		labelPairs = append(labelPairs, &dto.LabelPair{
			Name:  proto.String(name),
			Value: proto.String(value),
		})
	}
	if runes > ExemplarMaxRunes {
		return nil, fmt.Errorf("exemplar labels have %d runes, exceeding the limit of %d", runes, ExemplarMaxRunes)
	}
	e.Label = labelPairs
	return e, nil
}
