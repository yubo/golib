package printer

type Metric struct {
	Name        string              `thrift:"name,1" json:"name"`
	MetricValue *MetricValue        `thrift:"metricValue,2" json:"metricValue"`
	Timestamp   *int64              `thrift:"timestamp,3" json:"timestamp"`
	Tags        map[*MetricTag]bool `thrift:"tags,4" json:"tags"`
}

type MetricValue struct {
	Count *CountValue `thrift:"count,1" json:"count"`
	Gauge *GaugeValue `thrift:"gauge,2" json:"gauge"`
	Timer *TimerValue `thrift:"timer,3" json:"timer"`
}

func (p *MetricValue) IsSetCount() bool {
	return p.Count != nil
}
func (p *MetricValue) IsSetGauge() bool {
	return p.Gauge != nil
}
func (p *MetricValue) IsSetTimer() bool {
	return p.Timer != nil
}

type MetricTag struct {
	TagName  string  `thrift:"tagName,1" json:"tagName"`
	TagValue *string `thrift:"tagValue,2" json:"tagValue"`
}

type CountValue struct {
	I64Value *int64 `thrift:"i64Value,1" json:"i64Value"`
}

type GaugeValue struct {
	I64Value *int64   `thrift:"i64Value,1" json:"i64Value"`
	DValue   *float64 `thrift:"dValue,2" json:"dValue"`
}

type TimerValue struct {
	I64Value *int64   `thrift:"i64Value,1" json:"i64Value"`
	DValue   *float64 `thrift:"dValue,2" json:"dValue"`
}

type MetricBatch struct {
	Metrics    []*Metric           `thrift:"metrics,1" json:"metrics"`
	CommonTags map[*MetricTag]bool `thrift:"commonTags,2" json:"commonTags"`
}

func NewMetricBatch() *MetricBatch {
	return &MetricBatch{}
}

func NewMetric() *Metric {
	return &Metric{}
}

func NewMetricTag() *MetricTag {
	return &MetricTag{}
}

func NewMetricValue() *MetricValue {
	return &MetricValue{}
}

func NewCountValue() *CountValue {
	return &CountValue{}
}

func NewGaugeValue() *GaugeValue {
	return &GaugeValue{}
}

func NewTimerValue() *TimerValue {
	return &TimerValue{}
}
